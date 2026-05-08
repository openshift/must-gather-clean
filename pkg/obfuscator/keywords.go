package obfuscator

import (
	"regexp"
	"sort"

	"k8s.io/klog/v2"
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
		allMatches := pattern.FindAllStringIndex(name, -1)
		if len(allMatches) == 0 {
			continue
		}

		ranges := findProtectedRanges(name)
		var unprotectedCount uint
		// Replace right-to-left so indices stay valid.
		for i := len(allMatches) - 1; i >= 0; i-- {
			loc := allMatches[i]
			if isProtectedByRanges(ranges, loc[0], loc[1]) {
				continue
			}
			unprotectedCount++
			name = name[:loc[0]] + replacement + name[loc[1]:]
		}

		if unprotectedCount > 0 {
			_ = o.GenerateIfAbsent(keyword, keyword, unprotectedCount, func() string {
				return replacement
			})
		}
	}
	return name
}

// NewKeywordsObfuscator returns an Obfuscator that replaces word-boundary (\b) matches
// of each key with its corresponding value. Keywords in the generic skip list are excluded
// at construction time. Matches inside protected compounds (dotted like containerd.service,
// hyphenated like kube-apiserver) are also skipped at replacement time via findProtectedRanges.
func NewKeywordsObfuscator(replacements map[string]string) ReportingObfuscator {
	tracker := NewSimpleTrackerMap(replacements)
	patterns := make(map[string]*regexp.Regexp, len(replacements))
	orderedKeys := make([]string, 0, len(replacements))
	for keyword := range replacements {
		if isSkipListWord(keyword) {
			klog.Warningf("keyword %q is too generic and will be skipped to avoid false positives", keyword)
			continue
		}
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
