package obfuscator

import (
	"sync"

	"k8s.io/klog/v2"
)

type ReplacementReport struct {
	Replacements []Replacement
}

type Replacement struct {
	Original string `yaml:"original,omitempty"`
	Replaced string `yaml:"replaced,omitempty"`
	Total    uint   `yaml:"total,omitempty"`
}

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
	AddReplacement(original string, replacement string)

	// AddBulkReplacement will add a replacement along with its original string to the report.
	// Allows to set how many times 'original' occurs in source.
	// If there is an existing value that does not match the given replacement, it will exit with a non-zero status.
	AddBulkReplacement(original string, replacement string, occurrences uint)

	// GenerateIfAbsent returns the previously used replacement if the entry is already present.
	// If the replacement is not present then it uses the GenerateReplacement function to generate a replacement.
	// The "key" parameter must be used for lookup and the "generator" parameter to generate the replacement.
	GenerateIfAbsent(key string, generator GenerateReplacement) string
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

func (s *SimpleTracker) AddReplacement(original string, replacement string) {
	s.AddBulkReplacement(original, replacement, 1)
}

func (s *SimpleTracker) AddBulkReplacement(original string, replacement string, occurrences uint) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if val, ok := s.mapping[original]; ok {
		if replacement != val.Replaced {
			klog.Exitf("'%s' already has a value reported as '%s', tried to report '%s'", original, val.Replaced, replacement)
		}
		val.Total += occurrences
		s.mapping[original] = val
		return
	}
	s.mapping[original] = Replacement{Original: original, Replaced: replacement, Total: 1}
}

func (s *SimpleTracker) GenerateIfAbsent(key string, generator GenerateReplacement) string {
	s.lock.Lock()
	defer s.lock.Unlock()
	if val, ok := s.mapping[key]; ok {
		val.Total++
		s.mapping[key] = val
		return val.Replaced
	}
	if generator == nil {
		return ""
	}
	r := generator()
	s.mapping[key] = Replacement{Original: key, Replaced: r, Total: 1}
	return r
}

func (s *SimpleTracker) Initialize(replacements map[string]string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if len(s.mapping) > 0 {
		klog.Exitf("tracker was initialized more than once or after some replacements were already added.")
	}
	for k, v := range replacements {
		s.mapping[k] = Replacement{Original: k, Replaced: v, Total: 1}
	}
}

func (s ReplacementReport) AsMap() (m map[string]string) {
	m = map[string]string{}
	for _, v := range s.Replacements {
		m[v.Original] = v.Replaced
	}
	return
}

func NewSimpleTracker() ReplacementTracker {
	return &SimpleTracker{mapping: map[string]Replacement{}}
}
