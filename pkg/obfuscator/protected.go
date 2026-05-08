package obfuscator

import (
	"regexp"
)

// semVerPattern matches semantic version strings like 4.19.18 or v1.2.3-rc1
// that should never be treated as Azure resource names.
var semVerPattern = regexp.MustCompile(`^v?\d+\.\d+\.\d+`)

// protectedPatterns match compound tokens (hyphenated like "kube-apiserver",
// dotted like "containerd.service") where a keyword match should be suppressed.
//
// A match is only protected when it is a STRICT SUBSET of the compound — i.e.,
// the compound is strictly larger than the match. This means:
//   - keyword "proxy" does NOT match inside "kube-proxy" (strict subset, protected)
//   - keyword "kube-proxy" DOES match "kube-proxy" exactly (not a strict subset)
//
// Without this, replacing "service" would corrupt "containerd.service" into
// "containerd.REDACTED", and replacing "proxy" would break "kube-proxy".
var protectedPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\w+(?:-\w+)+`),
	regexp.MustCompile(`\w+(?:\.\w+)+`),
}

// genericSkipWords are single, common English words that frequently appear as
// Azure resource names (e.g., a managed identity named "service") but also
// appear pervasively in logs and config files. They are excluded from BOTH
// free-text Azure replacement (via shouldSkipFreeTextReplacement) and keyword
// obfuscation (via isSkipListWord in NewKeywordsObfuscator).
//
// Add a word here when it is:
//  1. A single, unhyphenated, common English/infrastructure term, AND
//  2. Replacing it in free text would cause widespread false positives.
//
// Do NOT add compound names here — those belong in knownInfraComponents.
var genericSkipWords = map[string]struct{}{
	"service":    {},
	"cluster":    {},
	"network":    {},
	"default":    {},
	"proxy":      {},
	"agent":      {},
	"node":       {},
	"server":     {},
	"worker":     {},
	"master":     {},
	"system":     {},
	"controller": {},
	"monitor":    {},
	"gateway":    {},
	"storage":    {},
	"admin":      {},
	"ingress":    {},
}

// knownInfraComponents are hyphenated compound names used as both Azure managed
// identity resource names and standard OpenShift component names. They must
// not be replaced in free text.
var knownInfraComponents = map[string]struct{}{
	"cloud-controller-manager": {},
	"cloud-network-config":     {},
	"cluster-api-azure":        {},
	"control-plane":            {},
	"disk-csi-driver":          {},
	"file-csi-driver":          {},
	"image-registry":           {},
}

// isSkipListWord returns true if s exactly matches one of the hardcoded common words
// like "service", "proxy", "agent". Only exact matches — "my-service" is NOT in the
// list and would still be replaced.
//
// This is intentionally a different mechanism from isPlainWord: isSkipListWord catches
// known specific terms regardless of structure, while isPlainWord catches the broader
// class of letters-only single-case strings. The two guards are complementary, not redundant.
func isSkipListWord(s string) bool {
	_, ok := genericSkipWords[s]
	return ok
}

// isPlainWord returns true if s is a simple word with no hyphens, digits, or mixed case —
// e.g. "service", "GPU". These are too generic to safely replace in free text.
func isPlainWord(s string) bool {
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

// shouldSkipFreeTextReplacement returns true if s should NOT be replaced
// outside of structured Azure ARM paths. This prevents false positives
// when Azure resource names happen to collide with common English words,
// infrastructure component names, or version strings.
//
// foundViaPhase1 is true when s was discovered by the ARM-path regex (Phase 1)
// rather than only being seeded externally. Phase 1 discoveries bypass the plain-word
// guard so that pure-lowercase private names like "johndoe" are still replaced in
// free-text fields (e.g. "resource_name":"johndoe") after their ARM path is replaced.
// The genericSkipWords guard is never bypassed: words like "service" must never be
// replaced in free text regardless of how they were discovered.
//
// Guards (checked in order):
//  1. Too short (< 5 chars): Azure resource names under 5 characters (e.g., "vm",
//     "rg", "GPU", "0") are too ambiguous to safely replace in free text — they
//     appear as substrings in countless unrelated contexts.
//  2. Generic skip words: common English/infra terms in genericSkipWords (e.g.
//     "service", "proxy") are never replaced in free text, even after Phase 1 discovery.
//  3. Plain word (letters-only, single case): avoids replacing generic lowercase
//     or uppercase words that appear everywhere in logs. Skipped for Phase 1
//     discoveries so private names like "johndoe" are fully replaced.
//  4. Known infra components: hyphenated names like "cloud-controller-manager"
//     that are both Azure identity names and Kubernetes component names.
//  5. Semantic version: strings like "4.19.18" that look like resource names
//     but are version numbers.
func shouldSkipFreeTextReplacement(s string, foundViaPhase1 bool) bool {
	if len(s) < 5 {
		return true
	}
	if isSkipListWord(s) {
		return true
	}
	if !foundViaPhase1 && isPlainWord(s) {
		return true
	}
	if _, ok := knownInfraComponents[s]; ok {
		return true
	}
	if semVerPattern.MatchString(s) {
		return true
	}
	return false
}

type protectedRange struct {
	start, end int
}

// findProtectedRanges pre-computes all compound token locations in text.
func findProtectedRanges(text string) []protectedRange {
	var ranges []protectedRange
	for _, p := range protectedPatterns {
		for _, loc := range p.FindAllStringIndex(text, -1) {
			ranges = append(ranges, protectedRange{start: loc[0], end: loc[1]})
		}
	}
	return ranges
}

// isProtectedByRanges returns true if the match at [start, end) is a
// strict subset of any pre-computed protected range.
func isProtectedByRanges(ranges []protectedRange, start, end int) bool {
	for _, r := range ranges {
		if r.start <= start && end <= r.end && (r.start < start || end < r.end) {
			return true
		}
	}
	return false
}
