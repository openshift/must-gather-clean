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
	// original is usually a substring of the (potentially multi-megabyte) line being obfuscated. Go overwrites the
	// stored key of a string-keyed map on every assignment, even if the key already exists, so we must always store
	// a copy. Otherwise every tracked value pins the most recent line it was seen in for the lifetime of the tracker.
	r.Counter[strings.Clone(original)] += count
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
	// Initialize imports existing replacements into the tracker. It may be
	// called before or after GenerateIfAbsent; an imported entry for a value
	// generated earlier must use the same replacement token. It panics if called
	// more than once, if the report assigns different tokens to the same
	// canonical value, or if reservation enabled trackers find a token already
	// assigned to another canonical value.
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

// ReplacementReservations coordinates replacement tokens across trackers for
// consistent IP, MAC, and Domain obfuscators. Other obfuscator types and static
// replacements do not participate.
//
// Imported reports are accepted even when independent tracker groups reused a
// token or assigned multiple tokens to one canonical value. Ambiguous imported
// associations are retained by their own trackers for exact replay, but their
// tokens are never reused by newly generated replacements. The first new token
// generated for an ambiguous canonical is shared by later trackers.
type ReplacementReservations struct {
	lock                     sync.Mutex
	owners                   map[string]string
	byCanonical              map[string]string
	reservedTokens           map[string]struct{}
	ambiguousTokens          map[string]struct{}
	ambiguousCanonicals      map[string]struct{}
	ambiguousGeneratedTokens map[string]string
}

func NewReplacementReservations() *ReplacementReservations {
	return &ReplacementReservations{
		owners:                   map[string]string{},
		byCanonical:              map[string]string{},
		reservedTokens:           map[string]struct{}{},
		ambiguousTokens:          map[string]struct{}{},
		ambiguousCanonicals:      map[string]struct{}{},
		ambiguousGeneratedTokens: map[string]string{},
	}
}

func (r *ReplacementReservations) ensureMapsLocked() {
	if r.owners == nil {
		r.owners = map[string]string{}
	}
	if r.byCanonical == nil {
		r.byCanonical = map[string]string{}
	}
	if r.reservedTokens == nil {
		r.reservedTokens = map[string]struct{}{}
	}
	if r.ambiguousTokens == nil {
		r.ambiguousTokens = map[string]struct{}{}
	}
	if r.ambiguousCanonicals == nil {
		r.ambiguousCanonicals = map[string]struct{}{}
	}
	if r.ambiguousGeneratedTokens == nil {
		r.ambiguousGeneratedTokens = map[string]string{}
	}
}

// reserve records one strict association.
func (r *ReplacementReservations) reserve(token, canonical string) bool {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.ensureMapsLocked()
	return r.reserveLocked(token, canonical)
}

func (r *ReplacementReservations) reserveLocked(token, canonical string) bool {
	if _, ambiguous := r.ambiguousTokens[token]; ambiguous {
		return false
	}
	if _, ambiguous := r.ambiguousCanonicals[canonical]; ambiguous {
		return false
	}
	if owner, exists := r.owners[token]; exists {
		return owner == canonical && r.byCanonical[canonical] == token
	}
	if existingToken, exists := r.byCanonical[canonical]; exists {
		return existingToken == token
	}
	r.owners[token] = canonical
	r.byCanonical[canonical] = token
	r.reservedTokens[token] = struct{}{}
	return true
}

// reserveStrictBatch validates a complete set of associations against the
// current reservation state and commits them only if every association can be
// accepted. It is used by Initialize, which must leave both the tracker and
// the shared reservations unchanged when an invalid report causes a panic.
func (r *ReplacementReservations) reserveStrictBatch(batch []replacementReservation) bool {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.ensureMapsLocked()

	candidate := &ReplacementReservations{
		owners:                   cloneStringMap(r.owners),
		byCanonical:              cloneStringMap(r.byCanonical),
		reservedTokens:           cloneSet(r.reservedTokens),
		ambiguousTokens:          cloneSet(r.ambiguousTokens),
		ambiguousCanonicals:      cloneSet(r.ambiguousCanonicals),
		ambiguousGeneratedTokens: cloneStringMap(r.ambiguousGeneratedTokens),
	}
	for _, reservation := range batch {
		if !candidate.reserveLocked(reservation.token, reservation.canonical) {
			return false
		}
	}

	r.owners = candidate.owners
	r.byCanonical = candidate.byCanonical
	r.reservedTokens = candidate.reservedTokens
	r.ambiguousTokens = candidate.ambiguousTokens
	r.ambiguousCanonicals = candidate.ambiguousCanonicals
	r.ambiguousGeneratedTokens = candidate.ambiguousGeneratedTokens
	return true
}

func cloneStringMap(source map[string]string) map[string]string {
	clone := make(map[string]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneSet(source map[string]struct{}) map[string]struct{} {
	clone := make(map[string]struct{}, len(source))
	for key := range source {
		clone[key] = struct{}{}
	}
	return clone
}

type replacementReservation struct {
	token     string
	canonical string
}

// reserveBatch imports a complete set of associations. Conflicts between
// distinct tracker groups are recorded as ambiguous rather than rejected. A
// conflicting association within the batch itself is rejected before any
// reservation is changed.
func (r *ReplacementReservations) reserveBatch(batch []replacementReservation) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	pendingByCanonical := make(map[string]string, len(batch))
	pendingByToken := make(map[string]string, len(batch))
	for _, reservation := range batch {
		if token, exists := pendingByCanonical[reservation.canonical]; exists && token != reservation.token {
			return fmt.Errorf(
				"conflicting existing replacement token %q for canonical value %q",
				reservation.token,
				reservation.canonical,
			)
		}
		pendingByCanonical[reservation.canonical] = reservation.token
		if canonical, exists := pendingByToken[reservation.token]; exists && canonical != reservation.canonical {
			return fmt.Errorf(
				"conflicting existing canonical value %q for replacement token %q",
				reservation.canonical,
				reservation.token,
			)
		}
		pendingByToken[reservation.token] = reservation.canonical
	}

	r.ensureMapsLocked()
	for _, reservation := range batch {
		r.importLocked(reservation)
	}
	return nil
}

func (r *ReplacementReservations) importLocked(reservation replacementReservation) {
	token := reservation.token
	canonical := reservation.canonical
	r.reservedTokens[token] = struct{}{}

	_, tokenIsAmbiguous := r.ambiguousTokens[token]
	_, canonicalIsAmbiguous := r.ambiguousCanonicals[canonical]
	if tokenIsAmbiguous || canonicalIsAmbiguous {
		r.markTokenAmbiguousLocked(token)
		r.markCanonicalAmbiguousLocked(canonical)
		return
	}

	if owner, exists := r.owners[token]; exists && owner != canonical {
		r.markTokenAmbiguousLocked(token)
		r.markCanonicalAmbiguousLocked(owner)
		r.markCanonicalAmbiguousLocked(canonical)
	} else if !exists {
		r.owners[token] = canonical
	}

	if _, ambiguous := r.ambiguousCanonicals[canonical]; !ambiguous {
		if existingToken, exists := r.byCanonical[canonical]; exists && existingToken != token {
			r.markCanonicalAmbiguousLocked(canonical)
		} else if !exists {
			r.byCanonical[canonical] = token
		}
	}
}

func (r *ReplacementReservations) markTokenAmbiguousLocked(token string) {
	r.ambiguousTokens[token] = struct{}{}
	if owner, exists := r.owners[token]; exists {
		delete(r.owners, token)
		r.markCanonicalAmbiguousLocked(owner)
	}
}

func (r *ReplacementReservations) markCanonicalAmbiguousLocked(canonical string) {
	r.ambiguousCanonicals[canonical] = struct{}{}
	delete(r.byCanonical, canonical)
}

const minimumReplacementReservationAttempts = 64

func replacementReservationAttemptLimit(reserved int) int {
	// Replaying a report can legitimately produce every previously reserved
	// token before reaching a free one. The minimum also tolerates occasional
	// repeats from generators that choose values randomly.
	if reserved < minimumReplacementReservationAttempts {
		return minimumReplacementReservationAttempts
	}
	return reserved + 1
}

// tokenForOrReserve returns the token already assigned to canonical, or
// generates and reserves the first unused token in one critical section. The
// generation and reservation must stay atomic: separate lookups let two
// trackers generate different tokens for the same canonical value.
func (r *ReplacementReservations) tokenForOrReserve(canonical string, generator GenerateReplacement) string {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.ensureMapsLocked()
	if token, exists := r.byCanonical[canonical]; exists {
		if _, tokenAmbiguous := r.ambiguousTokens[token]; !tokenAmbiguous {
			return token
		}
	}
	if token, exists := r.ambiguousGeneratedTokens[canonical]; exists {
		if _, tokenAmbiguous := r.ambiguousTokens[token]; !tokenAmbiguous {
			return token
		}
	}
	limit := replacementReservationAttemptLimit(len(r.reservedTokens))
	for attempt := 0; attempt < limit; attempt++ {
		token := generator()
		if _, exists := r.reservedTokens[token]; exists {
			continue
		}
		r.reservedTokens[token] = struct{}{}
		r.owners[token] = canonical
		if _, ambiguous := r.ambiguousCanonicals[canonical]; ambiguous {
			r.ambiguousGeneratedTokens[canonical] = token
		} else {
			r.byCanonical[canonical] = token
		}
		return token
	}
	panic(fmt.Sprintf(
		"obfuscator: replacement generator for canonical value %q did not produce an unused token in %d attempts",
		canonical,
		limit,
	))
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

	// canonical and original are frequently substrings of the (potentially multi-megabyte) line being obfuscated.
	// Retaining them as-is would pin the whole line in memory for the lifetime of the tracker, so we copy them.
	canonical = strings.Clone(canonical)
	var g string
	if s.allowDuplicateTokens {
		g = generator()
	} else {
		g = s.reservations.tokenForOrReserve(canonical, generator)
	}
	g = strings.Clone(g)
	s.mapping[canonical] = NewReplacement(canonical, strings.Clone(original), g, count)
	return g
}

func (s *SimpleTracker) Initialize(report ReplacementReport) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.initialized {
		panic("obfuscator: replacement tracker initialized more than once")
	}
	s.ensureMaps()

	nextMapping := cloneReplacementMap(s.mapping)
	batch := make([]replacementReservation, 0, len(report.Replacements))
	for _, r := range report.Replacements {
		if existing, ok := nextMapping[r.Canonical]; ok {
			if existing.ReplacedWith != r.ReplacedWith {
				panic(fmt.Sprintf("obfuscator: canonical value %q is assigned conflicting replacement tokens", r.Canonical))
			}
			for original, count := range r.Counter {
				existing.Increment(original, count)
			}
			continue
		}
		c := make(map[string]uint)
		for keyCopy, valueCopy := range r.Counter {
			c[keyCopy] = valueCopy
		}
		nextMapping[r.Canonical] = &Replacement{
			Canonical:    r.Canonical,
			ReplacedWith: r.ReplacedWith,
			Counter:      c,
		}
		if !s.allowDuplicateTokens && r.ReplacedWith != "" {
			batch = append(batch, replacementReservation{token: r.ReplacedWith, canonical: r.Canonical})
		}
	}
	if !s.allowDuplicateTokens && !s.reservations.reserveStrictBatch(batch) {
		panic("obfuscator: replacement token is already assigned to another canonical value")
	}
	s.mapping = nextMapping
	s.initialized = true
}

func cloneReplacementMap(source map[string]*Replacement) map[string]*Replacement {
	clone := make(map[string]*Replacement, len(source))
	for canonical, replacement := range source {
		counter := make(map[string]uint, len(replacement.Counter))
		for original, count := range replacement.Counter {
			counter[original] = count
		}
		clone[canonical] = &Replacement{
			Canonical:    replacement.Canonical,
			ReplacedWith: replacement.ReplacedWith,
			Counter:      counter,
		}
	}
	return clone
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
	if obfuscationType == schema.ObfuscateTypeDomain {
		if err := validateDomainNames(domainNames); err != nil {
			return nil, err
		}
	}
	domainIndex := newDomainCanonicalIndex(domainNames)

	if reservations == nil {
		reservations = NewReplacementReservations()
	}
	allowDuplicateTokens := replacementType != schema.ObfuscateReplacementTypeConsistent ||
		(obfuscationType != schema.ObfuscateTypeIP &&
			obfuscationType != schema.ObfuscateTypeMAC &&
			obfuscationType != schema.ObfuscateTypeDomain)

	mapping := make(map[string]*Replacement, len(existingReplacements))
	batch := make([]replacementReservation, 0, len(existingReplacements))
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
		if existing, ok := mapping[canonical]; ok {
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
		if !allowDuplicateTokens && replacement != "" {
			batch = append(batch, replacementReservation{token: replacement, canonical: canonical})
		}
		mapping[canonical] = NewReplacement(canonical, original, replacement, 0)
	}
	if err := reservations.reserveBatch(batch); err != nil {
		return nil, err
	}
	return &SimpleTracker{
		mapping:              mapping,
		reservations:         reservations,
		allowDuplicateTokens: allowDuplicateTokens,
	}, nil
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
	switch obfuscationType {
	case schema.ObfuscateTypeIP:
		return canonicalIP(decodePercentEncoded(original))
	case schema.ObfuscateTypeMAC:
		return canonicalMAC(original)
	case schema.ObfuscateTypeDomain:
		if domain := domainIndex.endingAt(original); domain != "" {
			return domain
		}
		return lowerASCIIString(decodePercentEncoded(original))
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
