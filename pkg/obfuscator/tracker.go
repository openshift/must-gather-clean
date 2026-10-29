package obfuscator

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/openshift/must-gather-clean/pkg/schema"
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
	lock                 sync.RWMutex
	mapping              map[string]*Replacement
	reservations         *ReplacementReservations
	allowDuplicateTokens bool
	initialized          bool
}

// ReplacementReservations coordinates replacement tokens across trackers. A
// token may identify only one canonical value, even when separate obfuscator
// entries produce it. This keeps report-driven deobfuscation unambiguous.
type ReplacementReservations struct {
	lock        sync.Mutex
	owners      map[string]string
	byCanonical map[string]string
}

func NewReplacementReservations() *ReplacementReservations {
	return &ReplacementReservations{
		owners:      map[string]string{},
		byCanonical: map[string]string{},
	}
}

func (r *ReplacementReservations) reserve(token, canonical string) bool {
	r.lock.Lock()
	defer r.lock.Unlock()
	if r.owners == nil {
		r.owners = map[string]string{}
	}
	if r.byCanonical == nil {
		r.byCanonical = map[string]string{}
	}
	if owner, exists := r.owners[token]; exists {
		return owner == canonical && r.byCanonical[canonical] == token
	}
	if existingToken, exists := r.byCanonical[canonical]; exists {
		return existingToken == token
	}
	r.owners[token] = canonical
	r.byCanonical[canonical] = token
	return true
}

// tokenForOrReserve returns the token already assigned to canonical, or
// generates and reserves the first unused token in one critical section. The
// generation and reservation must stay atomic: separate lookups let two
// trackers generate different tokens for the same canonical value.
func (r *ReplacementReservations) tokenForOrReserve(canonical string, generator GenerateReplacement) string {
	r.lock.Lock()
	defer r.lock.Unlock()
	if r.owners == nil {
		r.owners = map[string]string{}
	}
	if r.byCanonical == nil {
		r.byCanonical = map[string]string{}
	}
	if token, exists := r.byCanonical[canonical]; exists {
		return token
	}
	for {
		token := generator()
		if _, exists := r.owners[token]; exists {
			continue
		}
		r.owners[token] = canonical
		r.byCanonical[canonical] = token
		return token
	}
}

func (s *SimpleTracker) Report() ReplacementReport {
	s.lock.RLock()
	defer s.lock.RUnlock()

	var replacements []Replacement
	for _, v := range s.mapping {
		counter := make(map[string]uint, len(v.Counter))
		for original, count := range v.Counter {
			counter[original] = count
		}
		replacements = append(replacements, Replacement{
			Canonical:    v.Canonical,
			ReplacedWith: v.ReplacedWith,
			Counter:      counter,
		})
	}
	sort.SliceStable(replacements, func(first, second int) bool {
		if replacements[first].Canonical != replacements[second].Canonical {
			return replacements[first].Canonical < replacements[second].Canonical
		}
		return replacements[first].ReplacedWith < replacements[second].ReplacedWith
	})
	return ReplacementReport{Replacements: replacements}
}

func (s *SimpleTracker) GenerateIfAbsent(canonical string, original string, count uint, generator GenerateReplacement) string {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.ensureMaps()

	if r, ok := s.mapping[canonical]; ok {
		r.Increment(original, count)
		return r.ReplacedWith
	}

	var g string
	if s.allowDuplicateTokens {
		g = generator()
	} else {
		g = s.reservations.tokenForOrReserve(canonical, generator)
	}
	s.mapping[canonical] = NewReplacement(canonical, original, g, count)
	return g
}

func (s *SimpleTracker) Initialize(report ReplacementReport) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.initialized {
		panic("obfuscator: replacement tracker initialized more than once")
	}
	s.initialized = true
	s.ensureMaps()

	for _, r := range report.Replacements {
		if existing, ok := s.mapping[r.Canonical]; ok {
			if existing.ReplacedWith != r.ReplacedWith {
				panic(fmt.Sprintf("obfuscator: canonical value %q is assigned conflicting replacement tokens", r.Canonical))
			}
			for original, count := range r.Counter {
				existing.Increment(original, count)
			}
			continue
		}
		if !s.allowDuplicateTokens && r.ReplacedWith != "" &&
			!s.reservations.reserve(r.ReplacedWith, r.Canonical) {
			panic(fmt.Sprintf("obfuscator: replacement token %q is already assigned to another canonical value", r.ReplacedWith))
		}
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

func (s *SimpleTracker) ensureMaps() {
	if s.mapping == nil {
		s.mapping = map[string]*Replacement{}
	}
	if s.reservations == nil {
		s.reservations = NewReplacementReservations()
	}
}

func NewSimpleTracker() ReplacementTracker {
	return &SimpleTracker{
		mapping:              map[string]*Replacement{},
		reservations:         NewReplacementReservations(),
		allowDuplicateTokens: true,
	}
}

// NewSimpleTrackerMap takes the existing map of replacements as an argument and builds, returns the required ReplacementTracker
func NewSimpleTrackerMap(existingReplacements map[string]string) ReplacementTracker {
	tracker, err := NewSimpleTrackerMapForObfuscation(
		existingReplacements,
		schema.ObfuscateTypeKeywords,
		nil,
		schema.ObfuscateReplacementTypeStatic,
		nil,
	)
	if err != nil {
		// The legacy constructor has no error return. Any map this call could
		// reject is also rejected by the corresponding obfuscator constructor.
		panic(err)
	}
	return tracker
}

// NewSimpleTrackerMapForObfuscation rebuilds a tracker from a config or report
// replacement map. The type-aware canonicalization is essential when a report
// contains percent-encoded or alternate MAC/domain spellings: the obfuscators
// look those values up using their canonical form.
func NewSimpleTrackerMapForObfuscation(
	existingReplacements map[string]string,
	obfuscationType schema.ObfuscateType,
	domainNames []string,
	replacementType schema.ObfuscateReplacementType,
	reservations *ReplacementReservations,
) (ReplacementTracker, error) {
	domainIndex := newDomainCanonicalIndex(domainNames)

	if reservations == nil {
		reservations = NewReplacementReservations()
	}
	tracker := &SimpleTracker{
		mapping:      map[string]*Replacement{},
		reservations: reservations,
		allowDuplicateTokens: replacementType != schema.ObfuscateReplacementTypeConsistent ||
			(obfuscationType != schema.ObfuscateTypeIP &&
				obfuscationType != schema.ObfuscateTypeMAC &&
				obfuscationType != schema.ObfuscateTypeDomain),
	}

	keys := make([]string, 0, len(existingReplacements))
	for original := range existingReplacements {
		keys = append(keys, original)
	}
	sort.Strings(keys)
	for _, original := range keys {
		replacement := existingReplacements[original]
		if err := validateReplacementKey(original, obfuscationType); err != nil {
			return nil, err
		}
		canonical := canonicalReplacementKey(original, obfuscationType, domainIndex)
		if existing, ok := tracker.mapping[canonical]; ok {
			if existing.ReplacedWith != replacement {
				return nil, fmt.Errorf(
					"conflicting existing replacements for canonical value %q: %q and %q",
					canonical,
					existing.ReplacedWith,
					replacement,
				)
			}
			existing.Counter[original] = 0
			continue
		}
		if !tracker.allowDuplicateTokens && !tracker.reservations.reserve(replacement, canonical) {
			return nil, fmt.Errorf("conflicting existing replacement token %q for canonical value %q", replacement, canonical)
		}
		tracker.mapping[canonical] = NewReplacement(canonical, original, replacement, 0)
	}
	return tracker, nil
}

type domainCanonicalIndex map[string]string

func newDomainCanonicalIndex(domains []string) domainCanonicalIndex {
	index := make(domainCanonicalIndex, len(domains))
	for _, domain := range domains {
		normalized := lowerASCIIString(domain)
		if existing, exists := index[normalized]; !exists ||
			domainPatternLength(normalized) > domainPatternLength(existing) ||
			(domainPatternLength(normalized) == domainPatternLength(existing) && normalized < existing) {
			index[normalized] = normalized
		}
	}
	return index
}

// endingAt returns the most specific configured domain at the end of a
// report value. Both the literal spelling and one decoded layer are checked:
// matching accepts raw literal percent signs as well as their %25 encoding.
// Map lookups keep this independent of the number of configured domains.
func (i domainCanonicalIndex) endingAt(original string) string {
	best := ""
	for start := 0; start < len(original); start++ {
		if insidePercentEncodingAt(original, start) {
			continue
		}
		if domain := i.match(lowerASCIIString(original[start:])); domain != "" {
			best = moreSpecificDomain(best, domain)
		}
		if strings.Contains(original[start:], "%") {
			if domain := i.match(lowerASCIIString(decodePercentEncoded(original[start:]))); domain != "" {
				best = moreSpecificDomain(best, domain)
			}
		}
	}
	return best
}

func (i domainCanonicalIndex) match(value string) string {
	return i[value]
}

func moreSpecificDomain(current, candidate string) string {
	if candidate == "" {
		return current
	}
	if current == "" || domainPatternLength(candidate) > domainPatternLength(current) ||
		(domainPatternLength(candidate) == domainPatternLength(current) && candidate < current) {
		return candidate
	}
	return current
}

func canonicalReplacementKey(original string, obfuscationType schema.ObfuscateType, domainIndex domainCanonicalIndex) string {
	decoded := decodePercentEncoded(original)
	switch obfuscationType {
	case schema.ObfuscateTypeIP:
		return canonicalIP(decoded)
	case schema.ObfuscateTypeMAC:
		return canonicalMAC(decoded)
	case schema.ObfuscateTypeDomain:
		if domain := domainIndex.endingAt(original); domain != "" {
			return domain
		}
		return lowerASCIIString(decoded)
	default:
		return original
	}
}

func lowerASCIIString(value string) string {
	var lowered strings.Builder
	lowered.Grow(len(value))
	for index := 0; index < len(value); index++ {
		lowered.WriteByte(lowerASCIIByte(value[index]))
	}
	return lowered.String()
}
