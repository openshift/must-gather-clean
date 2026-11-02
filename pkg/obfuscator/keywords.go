package obfuscator

import (
	"sort"
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
	keywords := make([]string, 0, len(replacements))
	for keyword := range replacements {
		keywords = append(keywords, keyword)
	}
	sort.Slice(keywords, func(first, second int) bool {
		if len(keywords[first]) != len(keywords[second]) {
			return len(keywords[first]) > len(keywords[second])
		}
		return keywords[first] < keywords[second]
	})

	counts := make(map[string]uint, len(keywords))
	var output strings.Builder
	output.Grow(len(name))
	for index := 0; index < len(name); {
		matched := ""
		for _, keyword := range keywords {
			if keyword != "" && strings.HasPrefix(name[index:], keyword) {
				matched = keyword
				break
			}
		}
		if matched == "" {
			output.WriteByte(name[index])
			index++
			continue
		}
		output.WriteString(replacements[matched])
		counts[matched]++
		index += len(matched)
	}

	// GenerateIfAbsent populates the report for values that do not occur as
	// well as those that do. Call it in the same deterministic order used by
	// matching, after scanning the original input exactly once.
	for _, keyword := range keywords {
		replacement := replacements[keyword]
		_ = reporter.GenerateIfAbsent(keyword, keyword, counts[keyword], func() string {
			return replacement
		})
	}
	return output.String()
}

// NewKeywordsObfuscator returns an Obfuscator which replace all occurrences of keys in the map
// passed to it with the value of the key.
func NewKeywordsObfuscator(replacements map[string]string) ReportingObfuscator {
	tracker := NewSimpleTrackerMap(replacements)
	return NewKeywordsObfuscatorWithTracker(replacements, tracker)
}

// NewKeywordsObfuscatorWithTracker returns a keyword obfuscator using an
// already validated tracker. The tracker supplies configuration canonicalization
// and report metadata while the keyword map drives the literal replacements.
func NewKeywordsObfuscatorWithTracker(replacements map[string]string, tracker ReplacementTracker) ReportingObfuscator {
	return &keywordsObfuscator{
		ReplacementTracker: tracker,
		replacements:       replacements,
	}
}
