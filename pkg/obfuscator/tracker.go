package obfuscator

import (
	"sync"
)

var onlyOneInit = make(chan struct{})

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
	// Initialize initializes the tracker with some existing replacements. It should be called before
	// the first use of GetReplacement or AddReplacement. Panics if called more than once.
	Initialize(report ReplacementReport)

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

func (s *SimpleTracker) Initialize(report ReplacementReport) {
	close(onlyOneInit) // panics when called twice

	s.lock.Lock()
	defer s.lock.Unlock()

	for _, r := range report.Replacements {
		c := make(map[string]uint)
		for keyCopy, valueCopy := range r.Counter {
			c[keyCopy] = valueCopy
		}
		s.mapping[r.Canonical] = &Replacement{
			Canonical:    r.Canonical,
			ReplacedWith: r.ReplacedWith,
			Counter:      c,
		}
	}
}

func NewSimpleTracker() ReplacementTracker {
	return &SimpleTracker{mapping: map[string]*Replacement{}}
}

// NewSimpleTrackerMap takes the existing map of replacements as an argument and builds, returns the required ReplacementTracker
func NewSimpleTrackerMap(existingReplacements map[string]string) ReplacementTracker {

	var m = map[string]*Replacement{}
	// injecting the already-existing replacement report
	for key, value := range existingReplacements {
		m[key] = NewReplacement(key, key, value, 0)
	}
	return &SimpleTracker{mapping: m}
}
