package obfuscator

import (
	"sync"
)

type GenerateReplacement func() string

type ReplacementReport struct {
	Replacements []Replacement
}

func (s ReplacementReport) AsMap() (m map[string]string) {
	m = map[string]string{}
	for _, r := range s.Replacements {
		for o := range r.Counter {
			m[o] = r.ReplacedWith
		}
	}
	return
}

type Replacement struct {
	Canonical    string
	ReplacedWith string
	Counter      map[string]uint
}

func (r *Replacement) Increment(original string, count uint) {
	r.Counter[original] += count
}

func NewReplacement(canonical string, original string, replacement string, count uint) *Replacement {
	return &Replacement{
		Canonical:    canonical,
		ReplacedWith: replacement,
		Counter: map[string]uint{
			original: count,
		},
	}
}

// ReplacementTracker is used to track and generate replacements used by obfuscators
type ReplacementTracker interface {
	// Initialize initializes the tracker with some existing replacements. It should be called only once and before
	// the first use of GetReplacement or AddReplacement
	Initialize(replacements map[string]string)

	// Report returns a mapping of strings which were replaced.
	Report() ReplacementReport

	// GenerateIfAbsent returns the previously used replacement if the entry is already present.
	// If the replacement is not present then it uses the GenerateReplacement function to generate a replacement.
	// Canonical is used as the key to replace, a replacement of original with respective count will be recorded for reporting reasons.
	GenerateIfAbsent(canonical string, original string, count uint, generator GenerateReplacement) string
}

type SimpleTracker struct {
	lock    sync.RWMutex
	mapping map[string]*Replacement
}

func (s *SimpleTracker) Report() ReplacementReport {
	s.lock.RLock()
	defer s.lock.RUnlock()

	var replacements []Replacement
	for _, v := range s.mapping {
		replacements = append(replacements, *v)
	}
	return ReplacementReport{Replacements: replacements}
}

func (s *SimpleTracker) GenerateIfAbsent(canonical string, original string, count uint, generator GenerateReplacement) string {
	s.lock.Lock()
	defer s.lock.Unlock()

	if r, ok := s.mapping[canonical]; ok {
		r.Increment(original, count)
		return r.ReplacedWith
	}

	g := generator()
	s.mapping[canonical] = NewReplacement(canonical, original, g, count)
	return g
}

func (s *SimpleTracker) Initialize(replacements map[string]string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	for k, v := range replacements {
		s.mapping[k] = NewReplacement(k, k, v, 1)
	}
}

func NewSimpleTracker() ReplacementTracker {
	return &SimpleTracker{mapping: map[string]*Replacement{}}
}
