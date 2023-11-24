package obfuscator

import (
	"fmt"
	"regexp"
	"strings"
)

type regexObfuscator struct {
	ReplacementTracker
	pattern *regexp.Regexp
}

func (r *regexObfuscator) Path(s string) string {
	return r.replace(s)
}

func (r *regexObfuscator) Contents(s string) string {
	return r.replace(s)
}

func (r *regexObfuscator) replace(input string) string {
	output := input
	matches := r.pattern.FindAllString(input, -1)
	for _, m := range matches {
		replacement := strings.Repeat("x", len(m))
		r.GenerateIfAbsent(m, m, 1, func() string {
			return replacement
		})
		output = strings.ReplaceAll(output, m, replacement)
	}
	return output
}

func NewRegexObfuscator(pattern string, tracker ReplacementTracker) (ReportingObfuscator, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("pattern %s is invalid: %w", pattern, err)
	}
	return &regexObfuscator{
		pattern:            regex,
		ReplacementTracker: tracker,
	}, nil
}
