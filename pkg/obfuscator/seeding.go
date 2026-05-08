package obfuscator

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

// compoundPattern matches word-like tokens bounded by non-word/non-hyphen characters.
// Note: underscores are accepted internally (e.g. "consumer_name" matches as one token),
// but segmentPattern requires hyphen boundaries so underscore-joined tokens are rejected
// downstream and never seeded as compounds.
var compoundPattern = regexp.MustCompile(`[a-zA-Z0-9][a-zA-Z0-9_-]*[a-zA-Z0-9]|[a-zA-Z0-9]`)

// ExtractClusterToken finds the unique cluster identifier token from a list
// of must-gather file basenames. Returns "" if no mixed-alphanumeric cluster
// ID can be extracted (e.g., staging/prod where the ID is a region name).
func ExtractClusterToken(fileNames []string) string {
	_, token := ExtractClusterInfo(fileNames)
	return token
}

// ExtractClusterInfo returns both the common file name prefix (e.g.,
// "dev-wx5r9ktl-svc") and the unique cluster token (e.g., "wx5r9ktl").
// The prefix should be seeded as a canonical for path obfuscation; the
// token is used for compound discovery in file contents.
//
// Must-gather file names follow the pattern:
//
//	<env>-<clusterID>-<type>[-<stamp>]-<namespace>-<pod>.jsonl
//
// where <type> is "svc" or "mgmt" and <stamp> is an optional digit.
// The cluster ID is everything between the environment and the type.
func ExtractClusterInfo(fileNames []string) (prefix, token string) {
	if len(fileNames) == 0 {
		return "", ""
	}

	bases := make([]string, len(fileNames))
	for i, f := range fileNames {
		bases[i] = strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
	}

	prefix = longestCommonPrefix(bases)
	prefix = strings.TrimRight(prefix, "-")
	if prefix == "" {
		return "", ""
	}

	_, clusterID, _ := parseClusterPrefix(prefix)
	if clusterID == "" {
		return prefix, ""
	}

	if hasMixedAlphanumeric(clusterID) {
		return prefix, clusterID
	}
	return prefix, ""
}

// parseClusterPrefix splits a prefix like "dev-wx5r9ktl-svc" into its
// structural components: environment, cluster ID, and cluster type.
// Returns empty strings if the prefix doesn't match the expected structure.
func parseClusterPrefix(prefix string) (env, clusterID, clusterType string) {
	segments := strings.Split(prefix, "-")
	if len(segments) < 3 {
		return "", "", ""
	}

	env = segments[0]

	// Find "svc" or "mgmt" from the right — that's the cluster type boundary.
	// Skip trailing numeric stamps (e.g., the "1" in "tst-northeu-svc-1").
	typeIdx := -1
	for i := len(segments) - 1; i >= 1; i-- {
		if isNumeric(segments[i]) {
			continue
		}
		if segments[i] == "svc" || segments[i] == "mgmt" {
			typeIdx = i
			break
		}
		break
	}

	if typeIdx < 2 {
		return "", "", ""
	}

	clusterID = strings.Join(segments[1:typeIdx], "-")
	clusterType = segments[typeIdx]
	return env, clusterID, clusterType
}

func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs[1:] {
		for !strings.HasPrefix(s, prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}
	return prefix
}

func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// hasMixedAlphanumeric returns true if s contains both letters and digits.
// Pure-letter strings like "uksouth" (Azure region names) return false.
func hasMixedAlphanumeric(s string) bool {
	hasLetter := false
	hasDigit := false
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			hasLetter = true
		}
		if c >= '0' && c <= '9' {
			hasDigit = true
		}
	}
	return hasLetter && hasDigit
}

// DiscoverSensitiveCompounds finds all compound strings in text that contain
// the given token as a complete hyphen-delimited segment. A compound is a
// sequence of word characters and hyphens, bounded by non-word/non-hyphen
// characters (quotes, whitespace, dots, equals, brackets, etc.) or
// start/end of string.
func DiscoverSensitiveCompounds(text, token string, segmentPattern *regexp.Regexp) []string {
	if !strings.Contains(text, token) {
		return nil
	}

	decoded := text
	if strings.Contains(text, "%") {
		if d, err := url.QueryUnescape(text); err == nil {
			decoded = d
		}
	}

	seen := map[string]struct{}{}
	var results []string

	for _, match := range compoundPattern.FindAllString(decoded, -1) {
		if !segmentPattern.MatchString(match) {
			continue
		}
		if _, ok := seen[match]; ok {
			continue
		}
		seen[match] = struct{}{}
		results = append(results, match)
	}

	return results
}
