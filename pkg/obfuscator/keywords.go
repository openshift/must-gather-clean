package obfuscator

import (
	"strings"
)

type keywordsObfuscator struct {
	ReplacementTracker
	replacements map[string]string
}

func (o *keywordsObfuscator) Path(name string) string {
	return replace(name, o.replacements, o.ReplacementTracker)
}

func (o *keywordsObfuscator) Contents(contents string) string {
	return replace(contents, o.replacements, o.ReplacementTracker)
}

func replace(name string, replacements map[string]string, reporter ReplacementTracker) string {
	for keyword, replacement := range replacements {
		if strings.Contains(name, keyword) {
			cnt := uint(strings.Count(name, keyword))
			_ = reporter.GenerateIfAbsent(keyword, keyword, cnt, func() string {
				return replacement
			})
			name = strings.Replace(name, keyword, replacement, -1)
		}
	}
	return name
}

// NewKeywordsObfuscator returns an Obfuscator which replace all occurrences of keys in the map
// passed to it with the value of the key.
func NewKeywordsObfuscator(replacements map[string]string) ReportingObfuscator {
	tracker := NewSimpleTrackerMap(replacements)
	return &keywordsObfuscator{
		ReplacementTracker: tracker,
		replacements:       replacements,
	}
}
