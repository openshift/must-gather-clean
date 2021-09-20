package obfuscator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/openshift/must-gather-clean/pkg/schema"
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

func (r *regexObfuscator) Type() string {
	return string(schema.ObfuscateTypeRegex)
}

func (r *regexObfuscator) replace(input string) string {
	output := input
	matches := r.pattern.FindAllString(input, -1)
	for _, m := range matches {
		replacement := strings.Repeat("x", len(m))
		output = strings.ReplaceAll(output, m, replacement)
		r.AddReplacement(m, replacement)
	}
	return output
}

func NewRegexObfuscator(pattern string) (ReportingObfuscator, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("pattern %s is invalid: %w", pattern, err)
	}
	return &regexObfuscator{
		pattern:            regex,
		ReplacementTracker: NewSimpleTracker(),
	}, nil
}
