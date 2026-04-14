package obfuscator

import (
	"regexp"
	"sort"
)

type keywordsObfuscator struct {
	ReplacementTracker
	replacements map[string]string
	patterns     map[string]*regexp.Regexp
	orderedKeys  []string
}

func (o *keywordsObfuscator) Path(name string) string {
	return o.replace(name)
}

func (o *keywordsObfuscator) Contents(contents string) string {
	return o.replace(contents)
}

func (o *keywordsObfuscator) replace(name string) string {
	for _, keyword := range o.orderedKeys {
		replacement := o.replacements[keyword]
		pattern := o.patterns[keyword]
		matches := pattern.FindAllString(name, -1)
		if len(matches) > 0 {
			cnt := uint(len(matches))
			// Track the replacement; return value is unused because keywords use a fixed mapping.
			_ = o.GenerateIfAbsent(keyword, keyword, cnt, func() string {
				return replacement
			})
			name = pattern.ReplaceAllString(name, replacement)
		}
	}
	return name
}

// NewKeywordsObfuscator returns an Obfuscator that replaces word-boundary (\b) matches
// of each key with its corresponding value. Go's \b treats underscores and digits as
// word characters, so "server" won't match inside "grpc_server" or "server01", but
// dots and hyphens are boundaries, so it will match inside "containerd.service".
func NewKeywordsObfuscator(replacements map[string]string) ReportingObfuscator {
	tracker := NewSimpleTrackerMap(replacements)
	patterns := make(map[string]*regexp.Regexp, len(replacements))
	// Longest-first so longer matches win over overlapping shorter ones.
	orderedKeys := make([]string, 0, len(replacements))
	for keyword := range replacements {
		patterns[keyword] = regexp.MustCompile(`\b` + regexp.QuoteMeta(keyword) + `\b`)
		orderedKeys = append(orderedKeys, keyword)
	}
	sort.Slice(orderedKeys, func(i, j int) bool {
		if len(orderedKeys[i]) != len(orderedKeys[j]) {
			return len(orderedKeys[i]) > len(orderedKeys[j])
		}
		return orderedKeys[i] < orderedKeys[j]
	})
	return &keywordsObfuscator{
		ReplacementTracker: tracker,
		replacements:       replacements,
		patterns:           patterns,
		orderedKeys:        orderedKeys,
	}
}
