package obfuscator

import (
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/openshift/must-gather-clean/pkg/schema"
	"k8s.io/klog/v2"
	"k8s.io/utils/set"
)

const (
	staticAzureSubscriptionReplacement    = "obfuscated-subscription"
	staticAzureResourceGroupReplacement   = "obfuscated-resourcegroup"
	staticAzureResourceNameReplacement    = "obfuscated-resource-name"
	staticAzureSubresourceNameReplacement = "obfuscated-subresource-name"
)

var (
	//     /subscriptions/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx/resourceGroups/myResourceGroup/providers/Microsoft.Network/virtualNetworks/myVNet/subnets/mySubnet
	// Azure resource path pattern
	azureSubscriptionPattern  = `(?i)/subscriptions/([^(/\s'")]+)`
	azureResourceGroupPattern = `(?i)/resource[Gg]roups/([^(/\s'")]+)`
	azureResourcePattern      = `(?i)/providers/([^/]+)/([^/]+)/([^(/\s'")]+)`
	azureSubresourcePattern   = `(?i)` + azureResourcePattern + `/([^/]+)/([^(/\s'")]+)`
	azureNodePoolPattern      = `(?i)Microsoft.RedHatOpenShift/hcpOpenShiftClusters/nodePools/([^(/\s'")]+)`
)

type partialRegexReplacer struct {
	pattern string
	regex   *regexp.Regexp
	repl    func(string) string

	lock                  sync.RWMutex
	generator             *petNameReplacementGenerator
	canonicalReplacements set.Set[string]
}

func newPartialRegexReplacer(pattern string, generator *petNameReplacementGenerator, replaceFn func(original string, matches []string, replacer *partialRegexReplacer) string) *partialRegexReplacer {
	currRegex := regexp.MustCompile(pattern)
	ret := &partialRegexReplacer{
		pattern: pattern,
		regex:   currRegex,

		generator:             generator,
		canonicalReplacements: set.Set[string]{},
	}
	ret.repl = func(s string) string {
		matches := currRegex.FindStringSubmatch(s)
		if matches == nil {
			return s
		}

		return replaceFn(s, matches, ret)
	}

	return ret
}

func (t *partialRegexReplacer) generateReplacement(canonical, original string, count uint, tracker ReplacementTracker) string {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.canonicalReplacements.Insert(canonical)
	return t.generator.generateReplacement(canonical, original, count, tracker)
}

type azureResourceObfuscator struct {
	ReplacementTracker

	// we always check all of them because more than one can match a line, but they are evaluated in order because some are more specific than others.
	orderedPartialRegexReplacers []*partialRegexReplacer
}

func (o *azureResourceObfuscator) Path(s string) string {
	return o.replace(s)
}

func (o *azureResourceObfuscator) Contents(s string) string {
	return o.replace(s)
}

func (o *azureResourceObfuscator) replace(s string) string {
	patternReplacedString := s

	for _, currPartialRegexReplacer := range o.orderedPartialRegexReplacers {
		if !currPartialRegexReplacer.regex.MatchString(s) {
			continue
		}

		patternReplacedString = currPartialRegexReplacer.regex.ReplaceAllStringFunc(patternReplacedString, currPartialRegexReplacer.repl)
	}

	// at this point we have found all new substitutions, but we must still replace all previously discovered substitutions in the remaining string
	// we do these in reverse order because it appears to substitute slightly better to replace subscriptions and resourcegroups before resource names.
	canonicalToReplacer := map[string]*partialRegexReplacer{}
	for _, currGenerator := range o.orderedPartialRegexReplacers {
		currGenerator.lock.RLock()
		canonicalReplacements := currGenerator.canonicalReplacements.UnsortedList()
		currGenerator.lock.RUnlock()

		for _, canonicalStringToReplace := range canonicalReplacements {
			if strings.Contains(patternReplacedString, canonicalStringToReplace) {
				if len(canonicalStringToReplace) < 5 {
					klog.Warningf("Azure resource obfuscator will skip '%s' because it's too short", canonicalStringToReplace)
					// we don't want to replace the canonical string if it's too short, because it's probably a trivial string like "0"
					continue
				}
				canonicalToReplacer[canonicalStringToReplace] = currGenerator
				continue
			}
		}
	}

	// now we have all strings.  order by longest so that we replace as few times as possible.
	// Sort by length (descending) and alphabetically
	canonicalStrings := set.KeySet(canonicalToReplacer)
	canonicalStringsList := canonicalStrings.UnsortedList()
	sort.Slice(canonicalStringsList, func(i, j int) bool {
		if len(canonicalStringsList[i]) != len(canonicalStringsList[j]) {
			return len(canonicalStringsList[i]) > len(canonicalStringsList[j])
		}
		return canonicalStringsList[i] < canonicalStringsList[j]
	})

	// now do the replace
	for _, canonicalStringToReplace := range canonicalStringsList {
		currGenerator := canonicalToReplacer[canonicalStringToReplace]
		replacementString := currGenerator.generator.generateReplacement(canonicalStringToReplace, canonicalStringToReplace, 1, o.ReplacementTracker)
		patternReplacedString = strings.ReplaceAll(patternReplacedString, canonicalStringToReplace, replacementString)
	}

	return patternReplacedString
}

func NewAzureResourceObfuscator(replacementType schema.ObfuscateReplacementType, tracker ReplacementTracker, desiredSeed *int) (ReportingObfuscator, error) {
	var randSource RandomSource
	randSource = cryptoRandSource{}
	if desiredSeed != nil {
		randSource = rand.New(rand.NewSource(int64(*desiredSeed)))
	}

	if replacementType != schema.ObfuscateReplacementTypeStatic && replacementType != schema.ObfuscateReplacementTypeConsistent {
		return nil, fmt.Errorf("unsupported replacement type: %s", replacementType)
	}

	// create a shared petname generator with a fixed seed for reproducibility
	petNameGen := NewPetNameGenerator("-", randSource)

	// shared by a couple regexes
	resourceNameGen := newPetNameReplacementGenerator("resource", staticAzureResourceNameReplacement, petNameGen, replacementType)

	orderedPartialRegexReplacers := []*partialRegexReplacer{
		newPartialRegexReplacer(
			azureSubresourcePattern,
			newPetNameReplacementGenerator("subresource", staticAzureSubresourceNameReplacement, petNameGen, replacementType),
			func(original string, matches []string, replacer *partialRegexReplacer) string {
				if len(matches) < 6 {
					return original
				}

				providerName := matches[1]
				resourceType := matches[2]
				resourceName := matches[3]
				subresourceType := matches[4]
				subresourceNameReplacement := replacer.generateReplacement(matches[5], matches[5], 1, tracker)
				return fmt.Sprintf("/providers/%s/%s/%s/%s/%s", providerName, resourceType, resourceName, subresourceType, subresourceNameReplacement)
			}),
		newPartialRegexReplacer(
			azureResourcePattern,
			resourceNameGen,
			func(original string, matches []string, replacer *partialRegexReplacer) string {
				if len(matches) < 4 {
					return original
				}

				providerName := matches[1]
				resourceType := matches[2]
				resourceNameReplacement := replacer.generateReplacement(matches[3], matches[3], 1, tracker)
				return fmt.Sprintf("/providers/%s/%s/%s", providerName, resourceType, resourceNameReplacement)
			}),
		newPartialRegexReplacer(
			azureResourceGroupPattern,
			newPetNameReplacementGenerator("resourcegroup", staticAzureResourceGroupReplacement, petNameGen, replacementType),
			func(original string, matches []string, replacer *partialRegexReplacer) string {
				if len(matches) < 2 {
					return original
				}

				resourceGroupNameReplacement := replacer.generateReplacement(matches[1], matches[1], 1, tracker)
				return fmt.Sprintf("/resourcegroups/%s", resourceGroupNameReplacement)
			}),
		newPartialRegexReplacer(
			azureSubscriptionPattern,
			newPetNameReplacementGenerator("subscription", staticAzureSubscriptionReplacement, petNameGen, replacementType),
			func(original string, matches []string, replacer *partialRegexReplacer) string {
				if len(matches) < 2 {
					return original
				}

				subscriptionReplacement := replacer.generateReplacement(matches[1], matches[1], 1, tracker)
				return fmt.Sprintf("/subscriptions/%s", subscriptionReplacement)
			}),
		newPartialRegexReplacer(
			azureNodePoolPattern,
			resourceNameGen,
			func(original string, matches []string, replacer *partialRegexReplacer) string {
				if len(matches) < 2 {
					return original
				}

				nodePoolReplacement := replacer.generateReplacement(matches[1], matches[1], 1, tracker)
				return fmt.Sprintf("Microsoft.RedHatOpenShift/hcpOpenShiftClusters/nodePools/%s", nodePoolReplacement)
			}),
	}

	return &azureResourceObfuscator{
		ReplacementTracker:           tracker,
		orderedPartialRegexReplacers: orderedPartialRegexReplacers,
	}, nil
}
