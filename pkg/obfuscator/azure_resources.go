package obfuscator

import (
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/openshift/must-gather-clean/pkg/schema"
	"k8s.io/utils/set"
)

const (
	staticAzureSubscriptionReplacement    = "obfuscated-subscription"
	staticAzureResourceGroupReplacement   = "obfuscated-resourcegroup"
	staticAzureResourceNameReplacement    = "obfuscated-resource-name"
	staticAzureSubresourceNameReplacement = "obfuscated-subresource-name"
)

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// replaceNotInsideWord is like strings.ReplaceAll but skips matches embedded
// inside a larger alphabetic token (e.g. "Proxy1" inside "MyProxy1Handler").
func replaceNotInsideWord(s, old, repl string) (uint, string) {
	if !strings.Contains(s, old) {
		return 0, s
	}
	var count uint
	var result strings.Builder
	result.Grow(len(s))
	i := 0
	for {
		idx := strings.Index(s[i:], old)
		if idx == -1 {
			result.WriteString(s[i:])
			break
		}
		absIdx := i + idx
		endIdx := absIdx + len(old)

		leftIsLetter := absIdx > 0 && isLetter(s[absIdx-1])
		rightIsLetter := endIdx < len(s) && isLetter(s[endIdx])

		if leftIsLetter || rightIsLetter {
			result.WriteString(s[i:endIdx])
			i = endIdx
		} else {
			result.WriteString(s[i:absIdx])
			result.WriteString(repl)
			count++
			i = endIdx
		}
	}
	return count, result.String()
}

// isGenericWord returns true for single-case alphabetic strings like "service" or "GPU"
// that are too common to safely replace in free text.
func isGenericWord(s string) bool {
	if len(s) == 0 {
		return false
	}
	hasUpper := false
	hasLower := false
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		default:
			return false
		}
	}
	return !(hasUpper && hasLower)
}

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
				// Skip strings shorter than 5 characters to avoid false-positive replacements
				// on trivial values like "0", "vm", or "rg" that appear frequently in
				// unrelated contexts.
				if len(canonicalStringToReplace) < 5 {
					continue
				}
				if isGenericWord(canonicalStringToReplace) {
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

	// Replace canonicals in free text, skipping matches embedded inside larger words.
	for _, canonicalStringToReplace := range canonicalStringsList {
		// Count matches first (replace-with-self is a counting no-op)
		count, _ := replaceNotInsideWord(patternReplacedString, canonicalStringToReplace, canonicalStringToReplace)
		if count == 0 {
			continue
		}
		currGenerator := canonicalToReplacer[canonicalStringToReplace]
		replacementString := currGenerator.generator.generateReplacement(canonicalStringToReplace, canonicalStringToReplace, count, o.ReplacementTracker)
		_, patternReplacedString = replaceNotInsideWord(patternReplacedString, canonicalStringToReplace, replacementString)
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
