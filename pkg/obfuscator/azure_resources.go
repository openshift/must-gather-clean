package obfuscator

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/openshift/must-gather-clean/pkg/schema"
	"k8s.io/utils/set"
)

const (
	maximumSupportedObfuscationsAzure = 9999999999

	// Azure resource path templates
	azureSubscriptionTemplate    = "x-subscription-%010d-x"
	azureResourceGroupTemplate   = "x-resourcegroup-%010d-x"
	azureResourceNameTemplate    = "x-resource-%010d-x"
	azureSubresourceNameTemplate = "x-subresource-%010d-x"
	azureClusterIDTemplate       = "x-obfuscated-clusterid-%007d-x"

	staticAzureSubscriptionReplacement    = "obfuscated-subscription"
	staticAzureResourceGroupReplacement   = "obfuscated-resourcegroup"
	staticAzureResourceNameReplacement    = "obfuscated-resource-name"
	staticAzureSubresourceNameReplacement = "obfuscated-subresource-name"
	staticAzureClusterIDReplacement       = "x-obfuscated-clusterid-aaaaaaa-x"
)

var (
	//     /subscriptions/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx/resourceGroups/myResourceGroup/providers/Microsoft.Network/virtualNetworks/myVNet/subnets/mySubnet
	// Azure resource path pattern
	azureSubscriptionPattern  = `(?i)/subscriptions/([^(/\s')]+)`
	azureResourceGroupPattern = `(?i)/resource[Gg]roups/([^(/\s')]+)`
	azureResourcePattern      = `(?i)/providers/([^/]+)/([^/]+)/([^(/\s')]+)`
	azureSubresourcePattern   = `(?i)` + azureResourcePattern + `/([^/]+)/([^(/\s')]+)`
	azureNodePoolPattern      = `(?i)Microsoft.RedHatOpenShift/hcpOpenShiftClusters/nodePools/([^(/\s')]+)`
	azureClusterIdPattern     = "(?:^|[^0-9a-zA-Z])([0-9a-v]{32})(?:[^0-9a-zA-Z]|$)"
)

type partialRegexReplacer struct {
	pattern string
	regex   *regexp.Regexp
	repl    func(string) string

	lock                  sync.RWMutex
	generator             *generator
	canonicalReplacements set.Set[string]
}

func newPartialRegexReplacer(pattern string, generator *generator, replaceFn func(original string, matches []string, replacer *partialRegexReplacer) string) *partialRegexReplacer {
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

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func NewAzureResourceObfuscator(replacementType schema.ObfuscateReplacementType, tracker ReplacementTracker) (ReportingObfuscator, error) {
	// shared by a couple regexes
	resourceNameGen := must(newGenerator(azureResourceNameTemplate, staticAzureResourceNameReplacement, maximumSupportedObfuscationsAzure, replacementType))

	orderedPartialRegexReplacers := []*partialRegexReplacer{}
	orderedPartialRegexReplacers = append(orderedPartialRegexReplacers, newPartialRegexReplacer(
		azureSubresourcePattern,
		must(newGenerator(azureSubresourceNameTemplate, staticAzureSubresourceNameReplacement, maximumSupportedObfuscationsAzure, replacementType)),
		func(original string, matches []string, replacer *partialRegexReplacer) string {
			if len(matches) < 6 {
				return original
			}

			fullReplacementString := ""
			providerName := matches[1]
			resourceType := matches[2]
			resourceName := matches[3]
			subresourceType := matches[4]
			subresourceNameReplacement := replacer.generateReplacement(matches[5], matches[5], 1, tracker)
			fullReplacementString += fmt.Sprintf("/providers/%s/%s/%s/%s/%s", providerName, resourceType, resourceName, subresourceType, subresourceNameReplacement)

			return fullReplacementString
		}),
	)
	orderedPartialRegexReplacers = append(orderedPartialRegexReplacers, newPartialRegexReplacer(
		azureResourcePattern,
		resourceNameGen,
		func(original string, matches []string, replacer *partialRegexReplacer) string {
			if len(matches) < 4 {
				return original
			}

			fullReplacementString := ""
			providerName := matches[1]
			resourceType := matches[2]
			resourceNameReplacement := replacer.generateReplacement(matches[3], matches[3], 1, tracker)
			fullReplacementString += fmt.Sprintf("/providers/%s/%s/%s", providerName, resourceType, resourceNameReplacement)

			return fullReplacementString
		}),
	)
	orderedPartialRegexReplacers = append(orderedPartialRegexReplacers, newPartialRegexReplacer(
		azureResourceGroupPattern,
		must(newGenerator(azureResourceGroupTemplate, staticAzureResourceGroupReplacement, maximumSupportedObfuscationsAzure, replacementType)),
		func(original string, matches []string, replacer *partialRegexReplacer) string {
			if len(matches) < 2 {
				return original
			}

			fullReplacementString := ""
			resourceGroupNameReplacement := replacer.generateReplacement(matches[1], matches[1], 1, tracker)
			fullReplacementString += fmt.Sprintf("/resourcegroups/%s", resourceGroupNameReplacement)

			return fullReplacementString
		}),
	)
	orderedPartialRegexReplacers = append(orderedPartialRegexReplacers, newPartialRegexReplacer(
		azureSubscriptionPattern,
		must(newGenerator(azureSubscriptionTemplate, staticAzureSubscriptionReplacement, maximumSupportedObfuscationsAzure, replacementType)),
		func(original string, matches []string, replacer *partialRegexReplacer) string {
			if len(matches) < 2 {
				return original
			}

			fullReplacementString := ""
			resourceGroupNameReplacement := replacer.generateReplacement(matches[1], matches[1], 1, tracker)
			fullReplacementString += fmt.Sprintf("/subscriptions/%s", resourceGroupNameReplacement)

			return fullReplacementString
		}),
	)
	orderedPartialRegexReplacers = append(orderedPartialRegexReplacers, newPartialRegexReplacer(
		azureNodePoolPattern,
		resourceNameGen,
		func(original string, matches []string, replacer *partialRegexReplacer) string {
			if len(matches) < 2 {
				return original
			}

			fullReplacementString := ""
			resourceGroupNameReplacement := replacer.generateReplacement(matches[1], matches[1], 1, tracker)
			fullReplacementString += fmt.Sprintf("Microsoft.RedHatOpenShift/hcpOpenShiftClusters/nodePools/%s", resourceGroupNameReplacement)

			return fullReplacementString
		}),
	)
	orderedPartialRegexReplacers = append(orderedPartialRegexReplacers, newPartialRegexReplacer(
		azureClusterIdPattern,
		must(newGenerator(azureClusterIDTemplate, staticAzureClusterIDReplacement, maximumSupportedObfuscationsAzure, replacementType)),
		func(original string, matches []string, replacer *partialRegexReplacer) string {
			output := original
			if len(matches) >= 2 {
				clusterID := matches[1] // Extract the captured cluster ID from group 1
				fullMatch := matches[0] // The full match including boundary characters
				replacement := replacer.generateReplacement(clusterID, clusterID, 1, tracker)
				// Replace the full match with the boundary characters + replacement
				output = strings.ReplaceAll(output, fullMatch, strings.Replace(fullMatch, clusterID, replacement, 1))
			}
			return output
		}),
	)

	return &azureResourceObfuscator{
		ReplacementTracker:           tracker,
		orderedPartialRegexReplacers: orderedPartialRegexReplacers,
	}, nil
}
