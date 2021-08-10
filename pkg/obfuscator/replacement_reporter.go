package obfuscator

import "fmt"

// this struct mainly exists in case we later want to make it thread-safe, so we don't have to individually go through
// dozens of obfuscators.

type ReplacementReporter interface {
	// ReportingResult returns a mapping of strings which were replaced.
	ReportingResult() map[string]string

	// ReportReplacement will add a replacement along with its original string to the report.
	// If there is an existing value that does not match the given replacement, it will panic as this very likely denotes a bug.
	ReportReplacement(original string, replacement string)
}

type SimpleReporter struct {
	mapping map[string]string
}

func (s *SimpleReporter) ReportingResult() map[string]string {
	return s.mapping
}

func (s *SimpleReporter) ReportReplacement(original string, replacement string) {
	if val, ok := s.mapping[original]; ok {
		if replacement != val {
			panic(fmt.Sprintf("'%s' already has a value reported as '%s', tried to report '%s'", original, val, replacement))
		}
	}

	s.mapping[original] = replacement
}

func NewSimpleReporter() ReplacementReporter {
	return &SimpleReporter{map[string]string{}}
}
