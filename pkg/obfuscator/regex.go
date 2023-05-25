package obfuscator

import (
	"fmt"
	"strings"

	"github.com/gijsbers/go-pcre"
)

type regexObfuscator struct {
	ReplacementTracker
	pattern    pcre.Regexp
	patternStr string
}

func (r *regexObfuscator) Path(s string) string {
	return r.replace(s)
}

func (r *regexObfuscator) Contents(s string) string {
	return r.replace(s)
}

func (r *regexObfuscator) replace(input string) string {
	output := input
	matcher := r.pattern.MatcherString(input, 0)
	if r.patternStr == ".*" {
		m := matcher.GroupString(0)
		replacement := strings.Repeat("x", len(m))
		r.GenerateIfAbsent(m, m, 1, func() string {
			return replacement
		})
		output = strings.ReplaceAll(output, m, replacement)
	} else {
		for matcher.Matches() {
			m := matcher.GroupString(0)
			replacement := strings.Repeat("x", len(m))
			r.GenerateIfAbsent(m, m, 1, func() string {
				return replacement
			})
			output = strings.ReplaceAll(output, m, replacement)
			matcher = r.pattern.MatcherString(output, 0)
		}
	}
	return output
}

func NewRegexObfuscator(pattern string, tracker ReplacementTracker) (ReportingObfuscator, error) {
	regex, err := pcre.Compile(pattern, 0)
	if err != nil {
		return nil, fmt.Errorf("pattern %s is invalid: %w", pattern, err)
	}
	return &regexObfuscator{
		pattern:            regex,
		patternStr:         pattern,
		ReplacementTracker: tracker,
	}, nil
}
