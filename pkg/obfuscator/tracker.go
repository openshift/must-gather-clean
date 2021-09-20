package obfuscator

import (
	"sync"

	"k8s.io/klog/v2"
)

type GenerateReplacement func() string

// ReplacementTracker is used to track and generate replacements used by obfuscators
type ReplacementTracker interface {
	// Initialize initializes the tracker with some existing replacements. It should be called only once and before
	// the first use of GetReplacement or AddReplacement
	Initialize(replacements map[string]string)

	// Report returns a mapping of strings which were replaced.
	Report() ReplacementReport

	// AddReplacement will add a replacement along with its original string to the report.
	// If there is an existing value that does not match the given replacement, it will exit with a non-zero status.
	AddReplacement(canonical, original string, replacement string)

	// AddReplacementCount will add a replacement along with its original string to the report.
	// Allows to set how many times 'original' occurs in source.
	// If there is an existing value that does not match the given replacement, it will exit with a non-zero status.
	AddReplacementCount(canonical, original string, replacement string, count uint)

	// GenerateIfAbsent returns the previously used replacement if the entry is already present.
	// If the replacement is not present then it uses the GenerateReplacement function to generate a replacement.
	// The "key" parameter must be used for lookup and the "generator" parameter to generate the replacement.
	GenerateIfAbsent(canonical string, generator GenerateReplacement) string
}

type SimpleTracker struct {
	lock    sync.RWMutex
	mapping map[string]Replacement
}

var _ ReplacementTracker = (*SimpleTracker)(nil)

func (s *SimpleTracker) Report() ReplacementReport {
	s.lock.RLock()
	defer s.lock.RUnlock()
	replacements := make([]Replacement, 0, len(s.mapping))
	for _, v := range s.mapping {
		replacements = append(replacements, v)
	}
	return ReplacementReport{Replacements: replacements}
}

func (s *SimpleTracker) AddReplacement(canonical, original string, replacement string) {
	s.AddReplacementCount(canonical, original, replacement, 1)
}

func (s *SimpleTracker) AddReplacementCount(canonical, original string, replacement string, count uint) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if r, ok := s.mapping[canonical]; ok {
		s.mapping[canonical] = r.Increment(original, count)
		return
	}
	new := Replacement{Canonical: canonical, ReplacedWith: replacement}
	new.Increment(original, count)
	s.mapping[canonical] = new
}

func (s *SimpleTracker) GenerateIfAbsent(canonical string, generator GenerateReplacement) string {
	s.lock.Lock()
	defer s.lock.Unlock()
	if r, ok := s.mapping[canonical]; ok {
		return r.ReplacedWith
	}
	if generator == nil {
		return ""
	}
	return generator()
}

func (s *SimpleTracker) Initialize(replacements map[string]string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if len(s.mapping) > 0 {
		klog.Exitf("tracker was initialized more than once or after some replacements were already added.")
	}
	for k, v := range replacements {
		s.mapping[k] = Replacement{Canonical: k, ReplacedWith: v, Occurrences: []Occurrence{{Original: k, Count: 1}}}
	}
}

func (s ReplacementReport) AsMap() (m map[string]string) {
	m = map[string]string{}
	for _, r := range s.Replacements {
		for _, o := range r.Occurrences {
			m[o.Original] = r.ReplacedWith
		}
	}
	return
}

func NewSimpleTracker() ReplacementTracker {
	return &SimpleTracker{mapping: map[string]Replacement{}}
}
