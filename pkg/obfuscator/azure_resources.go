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
	staticAzureIdentityIDReplacement      = "obfuscated-identity-id"
)

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// replaceStandalone replaces old with repl only when old stands alone — i.e. the characters
// immediately before and after it are not letters. Hyphens, dots, slashes, quotes, spaces,
// and start/end of string are all valid boundaries.
// Example: replacing "Service" in `"name":"my-Service"` works (boundaries are `"` and `-`),
// but replacing "Service" in `myServiceHandler` is skipped (boundaries are letters `y` and `H`).
//
// skip, if non-nil, is an additional per-match guard: when skip(matchStart, matchEnd) returns
// true the match is left unchanged. Use this to suppress replacements inside protected ranges
// (e.g. hyphenated or dotted compound tokens).
func replaceStandalone(s, old, repl string, skip func(start, end int) bool) (uint, string) {
	if old == "" {
		return 0, s
	}
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

		if leftIsLetter || rightIsLetter || (skip != nil && skip(absIdx, endIdx)) {
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

// countStandalone counts standalone occurrences of old in s without allocating a result string.
// An occurrence is standalone when neither the byte immediately before nor after it is a letter
// and skip (if non-nil) returns false for that position.
func countStandalone(s, old string, skip func(start, end int) bool) uint {
	if old == "" || !strings.Contains(s, old) {
		return 0
	}
	var count uint
	i := 0
	for {
		idx := strings.Index(s[i:], old)
		if idx == -1 {
			break
		}
		absIdx := i + idx
		endIdx := absIdx + len(old)
		leftIsLetter := absIdx > 0 && isLetter(s[absIdx-1])
		rightIsLetter := endIdx < len(s) && isLetter(s[endIdx])
		if !leftIsLetter && !rightIsLetter && (skip == nil || !skip(absIdx, endIdx)) {
			count++
		}
		i = endIdx
	}
	return count
}

// azureNameChar matches a single character that can appear in an Azure resource name.
// Excluded: ( ) / whitespace ' " ? ] [ : \
const azureNameChar = `[^(/\s'"?\]\[:\\)]`

// uuidPattern matches a lowercase UUID (8-4-4-4-12 hex groups).
const uuidHex = `[0-9a-f]`
const uuidPattern = uuidHex + `{8}-` + uuidHex + `{4}-` + uuidHex + `{4}-` + uuidHex + `{4}-` + uuidHex + `{12}`

// azureIdentityFieldPattern matches JSON fields whose values are Azure identity
// UUIDs that should never appear in shared must-gather bundles.
// Case-insensitivity is scoped to the field name only so that the UUID capture
// group stays case-sensitive (matching lowercase hex as expected by Azure APIs).
// Captured groups: (1) field name, (2) UUID value.
const azureIdentityFieldPattern = `"(?i:(clientId|principalId|tenantId|objectId|appId))"\s*:\s*"(` + uuidPattern + `)"`

// azureK8sLabelPattern matches Kubernetes node labels set by the Azure cloud
// provider that contain sensitive UUIDs (subscription IDs, VNET GUIDs, etc.).
// Format: kubernetes.azure.com/LABEL=UUID
// Captured groups: (1) label name, (2) UUID value.
const azureK8sLabelPattern = `kubernetes\.azure\.com/([\w-]+)=(` + uuidPattern + `)`

var (
	//     /subscriptions/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx/resourceGroups/myResourceGroup/providers/Microsoft.Network/virtualNetworks/myVNet/subnets/mySubnet
	// Azure resource path pattern
	azureSubscriptionPattern  = `(?i)/subscriptions/(` + azureNameChar + `+)`
	azureResourceGroupPattern = `(?i)/resource[Gg]roups/(` + azureNameChar + `+)`
	azureResourcePattern      = `(?i)/providers/(` + azureNameChar + `+)/(` + azureNameChar + `+)/(` + azureNameChar + `+)`
	azureSubresourcePattern   = `(?i)` + azureResourcePattern + `/(` + azureNameChar + `+)/(` + azureNameChar + `+)`
	azureNodePoolPattern      = `(?i)Microsoft.RedHatOpenShift/hcpOpenShiftClusters/nodePools/(` + azureNameChar + `+)`
)

type partialRegexReplacer struct {
	pattern string
	regex   *regexp.Regexp
	repl    func(string) string

	lock                  sync.RWMutex
	generator             *petNameReplacementGenerator
	canonicalReplacements set.Set[string]
	// phase1Canonicals tracks canonicals discovered via ARM-path regex (Phase 1).
	// Unlike canonicalReplacements (which also includes seeded canonicals),
	// this set is only populated by generateReplacement, never by SeedCanonical.
	// Used to bypass the isPlainWord guard for private names like "johndoe".
	phase1Canonicals set.Set[string]
}

func newPartialRegexReplacer(pattern string, generator *petNameReplacementGenerator, replaceFn func(original string, matches []string, replacer *partialRegexReplacer) string) *partialRegexReplacer {
	currRegex := regexp.MustCompile(pattern)
	ret := &partialRegexReplacer{
		pattern: pattern,
		regex:   currRegex,

		generator:             generator,
		canonicalReplacements: set.Set[string]{},
		phase1Canonicals:      set.Set[string]{},
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
	t.phase1Canonicals.Insert(canonical)
	return t.generator.generateReplacement(canonical, original, count, tracker)
}

type azureResourceObfuscator struct {
	ReplacementTracker

	// we always check all of them because more than one can match a line, but they are evaluated in order because some are more specific than others.
	orderedPartialRegexReplacers []*partialRegexReplacer
	resourceReplacer             *partialRegexReplacer

	// clusterMu protects clusterToken and clusterSegmentPattern, which are
	// written once by SetClusterToken (before workers start) and read
	// concurrently by Contents.
	clusterMu             sync.RWMutex
	clusterToken          string
	clusterSegmentPattern *regexp.Regexp

	discoveredCompounds sync.Map
}

func (o *azureResourceObfuscator) Path(s string) string {
	return o.replace(s)
}

func (o *azureResourceObfuscator) Contents(s string) string {
	o.clusterMu.RLock()
	token := o.clusterToken
	pattern := o.clusterSegmentPattern
	o.clusterMu.RUnlock()

	if token != "" {
		for _, compound := range DiscoverSensitiveCompounds(s, token, pattern) {
			if _, loaded := o.discoveredCompounds.LoadOrStore(compound, struct{}{}); !loaded {
				o.SeedCanonical(compound)
			}
		}
	}
	return o.replace(s)
}

func (o *azureResourceObfuscator) SeedCanonical(canonical string) {
	replacer := o.resourceReplacer
	replacer.lock.Lock()
	replacer.canonicalReplacements.Insert(canonical)
	replacer.lock.Unlock()
}

func (o *azureResourceObfuscator) SetClusterToken(token string) {
	o.clusterMu.Lock()
	defer o.clusterMu.Unlock()
	o.clusterToken = token
	o.clusterSegmentPattern = regexp.MustCompile(`(?:^|-)` + regexp.QuoteMeta(token) + `(?:-|$)`)
}

// replace obfuscates Azure resource names in s using a two-phase approach:
//
//	Phase 1: Match structured ARM paths (/subscriptions/.../providers/...) via
//	         regex and replace resource names with generated pet names.
//	Phase 2: Replace any previously-discovered canonical resource names that
//	         also appear in free text (outside ARM paths), subject to the
//	         false-positive guards in shouldSkipFreeTextReplacement.
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
		phase1Snapshot := currGenerator.phase1Canonicals.UnsortedList()
		currGenerator.lock.RUnlock()

		phase1Set := make(map[string]struct{}, len(phase1Snapshot))
		for _, c := range phase1Snapshot {
			phase1Set[c] = struct{}{}
		}

		for _, canonicalStringToReplace := range canonicalReplacements {
			if strings.Contains(patternReplacedString, canonicalStringToReplace) {
				_, foundViaPhase1 := phase1Set[canonicalStringToReplace]
				// Gate prevents generic words like "service" from entering free-text
				// replacement. Phase 1 discoveries bypass the plain-word guard so
				// private lowercase names (e.g. "johndoe") are fully replaced.
				if shouldSkipFreeTextReplacement(canonicalStringToReplace, foundViaPhase1) {
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

	// Replace canonicals in free text, skipping matches embedded inside larger words
	// or inside compound tokens.
	//
	// For non-hyphenated canonicals (e.g. "myDisk123"), we also guard against
	// replacement when the canonical is a strict subset of a hyphenated or dotted
	// compound token (e.g. "myDisk123-handler"). Hyphenated canonicals (e.g.
	// "dev-wx5r9ktl-svc") are themselves compounds and need no additional guard —
	// applying compound protection to them would incorrectly block replacement when
	// they appear as a prefix of a longer hyphenated path like
	// "dev-wx5r9ktl-svc-kube-system-coredns.jsonl".
	for _, canonicalStringToReplace := range canonicalStringsList {
		var skipFn func(start, end int) bool
		if !strings.Contains(canonicalStringToReplace, "-") && !strings.Contains(canonicalStringToReplace, ".") {
			ranges := findProtectedRanges(patternReplacedString)
			skipFn = func(start, end int) bool {
				return isProtectedByRanges(ranges, start, end)
			}
		}
		count := countStandalone(patternReplacedString, canonicalStringToReplace, skipFn)
		if count == 0 {
			continue
		}
		currGenerator := canonicalToReplacer[canonicalStringToReplace]
		replacementString := currGenerator.generator.generateReplacement(canonicalStringToReplace, canonicalStringToReplace, count, o.ReplacementTracker)
		_, patternReplacedString = replaceStandalone(patternReplacedString, canonicalStringToReplace, replacementString, skipFn)
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

	// generatedReplacements tracks already-generated replacement strings so that
	// a replacement pet name is not itself re-obfuscated by a later regex pass.
	var generatedReplacements sync.Map

	// Named before adding to the slice so resourceReplacer can reference it directly
	// without relying on a fragile index into orderedPartialRegexReplacers.
	azureResourceReplacer := newPartialRegexReplacer(
		azureResourcePattern,
		resourceNameGen,
		func(original string, matches []string, replacer *partialRegexReplacer) string {
			if len(matches) < 4 {
				return original
			}

			providerName := matches[1]
			resourceType := matches[2]
			if strings.EqualFold(resourceType, "locations") {
				return original
			}
			if _, ok := generatedReplacements.Load(matches[3]); ok {
				return original
			}
			if semVerPattern.MatchString(matches[3]) {
				return original
			}
			resourceNameReplacement := replacer.generateReplacement(matches[3], matches[3], 1, tracker)
			generatedReplacements.Store(resourceNameReplacement, struct{}{})
			return fmt.Sprintf("/providers/%s/%s/%s", providerName, resourceType, resourceNameReplacement)
		})

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
				subresourceName := matches[5]
				if _, ok := generatedReplacements.Load(resourceName); ok {
					return original
				}
				if _, ok := generatedReplacements.Load(subresourceName); ok {
					return original
				}
				if semVerPattern.MatchString(subresourceName) {
					return original
				}
				subresourceNameReplacement := replacer.generateReplacement(subresourceName, subresourceName, 1, tracker)
				generatedReplacements.Store(subresourceNameReplacement, struct{}{})
				return fmt.Sprintf("/providers/%s/%s/%s/%s/%s", providerName, resourceType, resourceName, subresourceType, subresourceNameReplacement)
			}),
		azureResourceReplacer,
		newPartialRegexReplacer(
			azureResourceGroupPattern,
			newPetNameReplacementGenerator("resourcegroup", staticAzureResourceGroupReplacement, petNameGen, replacementType),
			func(original string, matches []string, replacer *partialRegexReplacer) string {
				if len(matches) < 2 {
					return original
				}
				if _, ok := generatedReplacements.Load(matches[1]); ok {
					return original
				}

				resourceGroupNameReplacement := replacer.generateReplacement(matches[1], matches[1], 1, tracker)
				generatedReplacements.Store(resourceGroupNameReplacement, struct{}{})
				return fmt.Sprintf("/resourcegroups/%s", resourceGroupNameReplacement)
			}),
		newPartialRegexReplacer(
			azureSubscriptionPattern,
			newPetNameReplacementGenerator("subscription", staticAzureSubscriptionReplacement, petNameGen, replacementType),
			func(original string, matches []string, replacer *partialRegexReplacer) string {
				if len(matches) < 2 {
					return original
				}
				if _, ok := generatedReplacements.Load(matches[1]); ok {
					return original
				}

				subscriptionReplacement := replacer.generateReplacement(matches[1], matches[1], 1, tracker)
				generatedReplacements.Store(subscriptionReplacement, struct{}{})
				return fmt.Sprintf("/subscriptions/%s", subscriptionReplacement)
			}),
		newPartialRegexReplacer(
			azureNodePoolPattern,
			resourceNameGen,
			func(original string, matches []string, replacer *partialRegexReplacer) string {
				if len(matches) < 2 {
					return original
				}
				if _, ok := generatedReplacements.Load(matches[1]); ok {
					return original
				}

				nodePoolReplacement := replacer.generateReplacement(matches[1], matches[1], 1, tracker)
				generatedReplacements.Store(nodePoolReplacement, struct{}{})
				return fmt.Sprintf("Microsoft.RedHatOpenShift/hcpOpenShiftClusters/nodePools/%s", nodePoolReplacement)
			}),
		newPartialRegexReplacer(
			azureIdentityFieldPattern,
			newPetNameReplacementGenerator("identity", staticAzureIdentityIDReplacement, petNameGen, replacementType),
			func(original string, matches []string, replacer *partialRegexReplacer) string {
				if len(matches) < 3 {
					return original
				}
				fieldName := matches[1]
				uuidValue := matches[2]
				if _, ok := generatedReplacements.Load(uuidValue); ok {
					return original
				}
				replacement := replacer.generateReplacement(uuidValue, uuidValue, 1, tracker)
				generatedReplacements.Store(replacement, struct{}{})
				return fmt.Sprintf(`"%s": "%s"`, fieldName, replacement)
			}),
		newPartialRegexReplacer(
			azureK8sLabelPattern,
			newPetNameReplacementGenerator("identity", staticAzureIdentityIDReplacement, petNameGen, replacementType),
			func(original string, matches []string, replacer *partialRegexReplacer) string {
				if len(matches) < 3 {
					return original
				}
				labelName := matches[1]
				uuidValue := matches[2]
				if _, ok := generatedReplacements.Load(uuidValue); ok {
					return original
				}
				replacement := replacer.generateReplacement(uuidValue, uuidValue, 1, tracker)
				generatedReplacements.Store(replacement, struct{}{})
				return fmt.Sprintf("kubernetes.azure.com/%s=%s", labelName, replacement)
			}),
	}

	return &azureResourceObfuscator{
		ReplacementTracker:           tracker,
		orderedPartialRegexReplacers: orderedPartialRegexReplacers,
		resourceReplacer:             azureResourceReplacer,
	}, nil
}
