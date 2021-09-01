package obfuscator

import (
	"sync"

	"k8s.io/klog/v2"
)

type GenerateReplacement func(string) string

// ReplacementTracker is used to track and generate Replacements used by obfuscators
type ReplacementTracker interface {
	// Initialize initializes the tracker with some existing Replacements. It should be called only once and before
	// the first use of GetReplacement or AddReplacement
	Initialize(replacements map[string]string)

	// Report returns a mapping of strings which were replaced.
	Report() map[string]string

	// AddReplacement will add a replacement along with its original string to the report.
	// If there is an existing value that does not match the given replacement, it will exit with a non-zero status.
	AddReplacement(original string, replacement string)

	// GenerateIfAbsent returns the previously used replacement if already set. If the replacement is not present then it
	// uses the GenerateReplacement function to generate a replacement. Generator should not be empty. The original
	// parameter must be used for lookup and the key parameter to generate the replacement.
	GenerateIfAbsent(original string, key string, generator GenerateReplacement) string
}

type SimpleTracker struct {
	lock    sync.RWMutex
	mapping map[string]string
}

func (s *SimpleTracker) Report() map[string]string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	defensiveCopy := make(map[string]string)
	for k, v := range s.mapping {
		defensiveCopy[k] = v
	}
	return defensiveCopy
}

func (s *SimpleTracker) AddReplacement(original string, replacement string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if val, ok := s.mapping[original]; ok {
		if replacement != val {
			klog.Exitf("'%s' already has a value reported as '%s', tried to report '%s'", original, val, replacement)
		}
		return
	}
	s.mapping[original] = replacement
}

func (s *SimpleTracker) GenerateIfAbsent(original string, key string, generator GenerateReplacement) string {
	s.lock.Lock()
	defer s.lock.Unlock()
	if val, ok := s.mapping[original]; ok {
		return val
	}
	if generator == nil {
		return ""
	}
	r := generator(key)
	s.mapping[original] = r
	return r
}

func (s *SimpleTracker) Initialize(replacements map[string]string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if len(s.mapping) > 0 {
		klog.Exitf("tracker was initialized more than once or after some Replacements were already added.")
	}
	for k, v := range replacements {
		s.mapping[k] = v
	}
}

func NewSimpleTracker() ReplacementTracker {
	return &SimpleTracker{mapping: map[string]string{}}
}
