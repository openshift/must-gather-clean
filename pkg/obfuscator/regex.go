package obfuscator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/openshift/must-gather-clean/pkg/schema"
)

type regexObfuscator struct {
	ReplacementTracker
	pattern  *regexp.Regexp
	location schema.ObfuscateTarget
}

func (r *regexObfuscator) FileName(s string) string {
	if r.location == schema.ObfuscateTargetAll || r.location == schema.ObfuscateTargetFileName {
		return r.replace(s)
	}
	return s
}

func (r *regexObfuscator) Contents(s string) string {
	if r.location == schema.ObfuscateTargetAll || r.location == schema.ObfuscateTargetFileContents {
		return r.replace(s)
	}
	return s
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

func NewRegexObfuscator(pattern string, replacementLocation schema.ObfuscateTarget) (Obfuscator, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("pattern %s is invalid: %w", pattern, err)
	}
	return &regexObfuscator{
		pattern:            regex,
		location:           replacementLocation,
		ReplacementTracker: NewSimpleTracker(),
	}, nil
}
