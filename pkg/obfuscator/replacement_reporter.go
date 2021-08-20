package obfuscator

import (
	"sync"

	"k8s.io/klog/v2"
)

// ReplacementReporter contains all the replacements which are performed by an Obfuscator
type ReplacementReporter interface {
	// ReportingResult returns a mapping of strings which were replaced.
	ReportingResult() map[string]string

	// ReportReplacement will add a replacement along with its original string to the report.
	// If there is an existing value that does not match the given replacement, it will panic as this very likely denotes a bug.
	ReportReplacement(original string, replacement string)

	// GetReplacement returns the previously used replacement if already set, otherwise returns an empty string
	GetReplacement(original string) string
}

type SimpleReporter struct {
	lock    sync.RWMutex
	mapping map[string]string
}

func (s *SimpleReporter) ReportingResult() map[string]string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	defensiveCopy := make(map[string]string)
	for k, v := range s.mapping {
		defensiveCopy[k] = v
	}
	return defensiveCopy
}

func (s *SimpleReporter) ReportReplacement(original string, replacement string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if val, ok := s.mapping[original]; ok {
		if replacement != val {
			klog.Exitf("'%s' already has a value reported as '%s', tried to report '%s'", original, val, replacement)
		}
	}

	s.mapping[original] = replacement
}

func (s *SimpleReporter) GetReplacement(original string) string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.mapping[original]
}

func NewSimpleReporter() ReplacementReporter {
	return &SimpleReporter{mapping: map[string]string{}}
}
