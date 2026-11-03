package obfuscator

import (
	"fmt"
	"sort"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHappyPathTracking(t *testing.T) {
	st := NewSimpleTracker()
	r := st.GenerateIfAbsent("a-canonical", "a-original", 1, func() string {
		return "a-replaced"
	})

	assert.Equal(t, "a-replaced", r)
	replacementReportsMatch(t, ReplacementReport{Replacements: []Replacement{
		{
			Canonical:    "a-canonical",
			ReplacedWith: "a-replaced",
			Counter:      map[string]uint{"a-original": 1},
		},
	}}, st.Report())
}

func TestMultiGeneratedReplacementIncrements(t *testing.T) {
	st := NewSimpleTracker()
	r := st.GenerateIfAbsent("a-canonical", "a-original", 1, func() string {
		return "a-replaced"
	})

	assert.Equal(t, "a-replaced", r)
	r = st.GenerateIfAbsent("a-canonical", "a-original", 1, func() string {
		return "a-replaced"
	})
	assert.Equal(t, "a-replaced", r)
	replacementReportsMatch(t, ReplacementReport{Replacements: []Replacement{
		{
			Canonical:    "a-canonical",
			ReplacedWith: "a-replaced",
			Counter:      map[string]uint{"a-original": 2},
		},
	}}, st.Report())
}

func TestMultiGeneratedReplacementDifferentOriginals(t *testing.T) {
	st := NewSimpleTracker()
	r := st.GenerateIfAbsent("a-canonical", "a-original", 1, func() string {
		return "a-replaced"
	})

	assert.Equal(t, "a-replaced", r)
	r = st.GenerateIfAbsent("a-canonical", "a-original-2", 1, func() string {
		return "a-replaced"
	})
	assert.Equal(t, "a-replaced", r)
	replacementReportsMatch(t, ReplacementReport{Replacements: []Replacement{
		{
			Canonical:    "a-canonical",
			ReplacedWith: "a-replaced",
			Counter:      map[string]uint{"a-original": 1, "a-original-2": 1},
		},
	}}, st.Report())
}

func TestHappyPathInit(t *testing.T) {
	st := NewSimpleTracker()
	r := ReplacementReport{
		[]Replacement{
			{
				Canonical:    "a",
				ReplacedWith: "b",
				Counter:      map[string]uint{"a": 1},
			},
		},
	}
	st.Initialize(r)
	replacementReportsMatch(t, ReplacementReport{Replacements: []Replacement{
		{
			Canonical:    "a",
			ReplacedWith: "b",
			Counter:      map[string]uint{"a": 1},
		},
	}}, st.Report())
}

func replacementReportsMatch(t *testing.T, want, got ReplacementReport) {
	assert.Equal(t, len(want.Replacements), len(got.Replacements))
	if len(want.Replacements) != len(got.Replacements) {
		return
	}

	sort.Slice(want.Replacements, func(i, j int) bool {
		return want.Replacements[i].Canonical > want.Replacements[j].Canonical
	})
	sort.Slice(got.Replacements, func(i, j int) bool {
		return got.Replacements[i].Canonical > got.Replacements[j].Canonical
	})

	for i := range got.Replacements {
		w := want.Replacements[i]
		g := got.Replacements[i]
		assert.Equal(t, w.Canonical, g.Canonical)
		assert.Equal(t, w.ReplacedWith, g.ReplacedWith)
		assert.Equal(t, w.Counter, g.Counter)
	}
}

// aliases reports whether s shares its backing memory with line.
func aliases(s, line string) bool {
	if len(s) == 0 || len(line) == 0 {
		return false
	}
	p := uintptr(unsafe.Pointer(unsafe.StringData(s)))
	start := uintptr(unsafe.Pointer(unsafe.StringData(line)))
	return p >= start && p < start+uintptr(len(line))
}

// The tracker lives for the whole run, so anything it stores must not alias the (potentially huge) input lines.
// Go overwrites the stored key of a string-keyed map on every assignment, so this also has to hold for repeat hits.
func TestTrackerDoesNotRetainInputLines(t *testing.T) {
	tracker := NewSimpleTracker()
	first := "prefix 10.0.0.1 suffix"
	second := "other 10.0.0.1 line"
	tracker.GenerateIfAbsent(first[7:15], first[7:15], 1, func() string { return "x-ipv4-0000000001-x" })
	tracker.GenerateIfAbsent(second[6:14], second[6:14], 1, func() string { return "unused" })

	for _, r := range tracker.Report().Replacements {
		for _, line := range []string{first, second} {
			assert.False(t, aliases(r.Canonical, line), "canonical aliases input line")
			for original := range r.Counter {
				assert.False(t, aliases(original, line), "counter key aliases input line")
			}
		}
	}
}

func TestSimpleTrackerReportIsSorted(t *testing.T) {
	tracker := NewSimpleTracker()
	tracker.GenerateIfAbsent("z-canonical", "z-original", 1, func() string { return "z-replaced" })
	tracker.GenerateIfAbsent("a-canonical", "a-original", 1, func() string { return "a-replaced" })
	tracker.GenerateIfAbsent("m-canonical", "m-original", 1, func() string { return "m-replaced" })

	assert.Equal(t, []Replacement{
		{Canonical: "a-canonical", ReplacedWith: "a-replaced", Counter: map[string]uint{"a-original": 1}},
		{Canonical: "m-canonical", ReplacedWith: "m-replaced", Counter: map[string]uint{"m-original": 1}},
		{Canonical: "z-canonical", ReplacedWith: "z-replaced", Counter: map[string]uint{"z-original": 1}},
	}, tracker.Report().Replacements)
}

func TestSimpleTrackerReportReturnsSnapshot(t *testing.T) {
	tracker := NewSimpleTracker()
	tracker.GenerateIfAbsent("canonical", "original", 1, func() string { return "replacement" })

	report := tracker.Report()
	tracker.GenerateIfAbsent("canonical", "original", 1, func() string { return "replacement" })

	assert.Equal(t, map[string]uint{"original": 1}, report.Replacements[0].Counter)
	assert.Equal(t, map[string]uint{"original": 2}, tracker.Report().Replacements[0].Counter)
}

func TestSimpleTrackerInitializeReservesExistingTokens(t *testing.T) {
	tracker, err := NewSimpleTrackerMapForObfuscation(
		nil,
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		nil,
	)
	require.NoError(t, err)
	tracker.Initialize(ReplacementReport{Replacements: []Replacement{{
		Canonical:    "192.0.2.1",
		ReplacedWith: "x-ipv4-0000000001-x",
		Counter:      map[string]uint{"192.0.2.1": 1},
	}}})

	nextToken := 0
	generator := func() string {
		nextToken++
		return "x-ipv4-000000000" + string(rune('0'+nextToken)) + "-x"
	}
	assert.Equal(t, "x-ipv4-0000000002-x", tracker.GenerateIfAbsent("198.51.100.7", "198.51.100.7", 1, generator))
	assert.Equal(t, 2, nextToken)
}

func TestSimpleTrackerCanBeInitializedOncePerInstance(t *testing.T) {
	report := ReplacementReport{Replacements: []Replacement{{
		Canonical:    "original",
		ReplacedWith: "token",
		Counter:      map[string]uint{"original": 1},
	}}}
	first := NewSimpleTracker()
	second := NewSimpleTracker()

	require.NotPanics(t, func() { first.Initialize(report) })
	require.NotPanics(t, func() { second.Initialize(report) })
	require.Panics(t, func() { first.Initialize(report) })
}

func TestSimpleTrackerRejectsTokenSharedByDifferentCanonicalValues(t *testing.T) {
	_, err := NewSimpleTrackerMapForObfuscation(
		map[string]string{
			"192.0.2.1":    "x-ipv4-0000000001-x",
			"198.51.100.7": "x-ipv4-0000000001-x",
		},
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "x-ipv4-0000000001-x")
}

func TestSimpleTrackerAcceptsTokenSharedByIndependentReplacementMaps(t *testing.T) {
	reservations := NewReplacementReservations()
	first, err := NewSimpleTrackerMapForObfuscation(
		map[string]string{"192.0.2.1": "x-ipv4-0000000001-x"},
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		reservations,
	)
	require.NoError(t, err)
	second, err := NewSimpleTrackerMapForObfuscation(
		map[string]string{"198.51.100.7": "x-ipv4-0000000001-x"},
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		reservations,
	)
	require.NoError(t, err)

	assert.Equal(t, "x-ipv4-0000000001-x", first.GenerateIfAbsent(
		"192.0.2.1", "192.0.2.1", 1, func() string { return "must-not-generate" },
	))
	assert.Equal(t, "x-ipv4-0000000001-x", second.GenerateIfAbsent(
		"198.51.100.7", "198.51.100.7", 1, func() string { return "must-not-generate" },
	))

	generated := 0
	assert.Equal(t, "x-ipv4-0000000002-x", first.GenerateIfAbsent(
		"203.0.113.9", "203.0.113.9", 1, func() string {
			generated++
			if generated == 1 {
				return "x-ipv4-0000000001-x"
			}
			return "x-ipv4-0000000002-x"
		},
	))
	assert.Equal(t, 2, generated)
}

func TestSimpleTrackerAcceptsTokenSharedByCanonicalAliases(t *testing.T) {
	tracker, err := NewSimpleTrackerMapForObfuscation(
		map[string]string{
			"192.0.2.1":                   "x-ipv4-0000000001-x",
			"192_0_2_1":                   "x-ipv4-0000000001-x",
			"%31%39%32%2E%30%2E%32%2E%31": "x-ipv4-0000000001-x",
		},
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		nil,
	)
	require.NoError(t, err)

	report := tracker.Report().Replacements
	require.Len(t, report, 1)
	assert.Equal(t, "192.0.2.1", report[0].Canonical)
	assert.Len(t, report[0].Counter, 3)
}

func TestReplacementReservationsPreventCrossTrackerCollisions(t *testing.T) {
	reservations := NewReplacementReservations()
	first, err := NewSimpleTrackerMapForObfuscation(
		nil,
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		reservations,
	)
	require.NoError(t, err)
	second, err := NewSimpleTrackerMapForObfuscation(
		nil,
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		reservations,
	)
	require.NoError(t, err)

	firstToken := first.GenerateIfAbsent("192.0.2.1", "192.0.2.1", 1, func() string {
		return "x-ipv4-0000000001-x"
	})
	generated := 0
	secondToken := second.GenerateIfAbsent("198.51.100.7", "198.51.100.7", 1, func() string {
		generated++
		if generated == 1 {
			return "x-ipv4-0000000001-x"
		}
		return "x-ipv4-0000000002-x"
	})

	assert.Equal(t, "x-ipv4-0000000001-x", firstToken)
	assert.Equal(t, "x-ipv4-0000000002-x", secondToken)
	assert.Equal(t, 2, generated)
}

func TestReplacementReservationsRejectSameCanonicalWithDifferentToken(t *testing.T) {
	reservations := NewReplacementReservations()
	require.True(t, reservations.reserve("same", "same"))
	require.False(t, reservations.reserve("different", "same"))
	require.False(t, reservations.reserve("same", "different"))
	require.True(t, reservations.reserve("same", "same"))
}

func TestReplacementReservationsReuseTokenAcrossTrackers(t *testing.T) {
	reservations := NewReplacementReservations()
	first, err := NewSimpleTrackerMapForObfuscation(
		nil,
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		reservations,
	)
	require.NoError(t, err)
	second, err := NewSimpleTrackerMapForObfuscation(
		nil,
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		reservations,
	)
	require.NoError(t, err)

	firstToken := first.GenerateIfAbsent("192.0.2.1", "192.0.2.1", 1, func() string {
		return "x-ipv4-0000000001-x"
	})
	generated := 0
	secondToken := second.GenerateIfAbsent("192.0.2.1", "192%2E0%2E2%2E1", 1, func() string {
		generated++
		return "x-ipv4-0000000002-x"
	})

	assert.Equal(t, firstToken, secondToken)
	assert.Zero(t, generated)
	report := second.Report().Replacements
	require.Len(t, report, 1)
	assert.Equal(t, map[string]uint{"192%2E0%2E2%2E1": 1}, report[0].Counter)
}

func TestReplacementReservationsGenerateOnlyOnceForCanonical(t *testing.T) {
	reservations := NewReplacementReservations()
	reservations.reserve("shared-token", "canonical")

	reservations.tokenForOrReserve("canonical", func() string {
		t.Fatal("generator called for an already reserved canonical value")
		return ""
	})
}

func TestReplacementReservationsReservedGeneratorStops(t *testing.T) {
	reservations := NewReplacementReservations()
	require.True(t, reservations.reserve("taken", "existing"))
	limit := replacementReservationAttemptLimit(len(reservations.owners))
	calls := 0

	require.Panics(t, func() {
		reservations.tokenForOrReserve("new", func() string {
			calls++
			if calls > limit {
				return "terminate-regressed-loop"
			}
			return "taken"
		})
	})
	assert.Equal(t, limit, calls)
}

func TestReplacementReservationsAttemptBudgetScalesWithRestoredTokens(t *testing.T) {
	reservations := NewReplacementReservations()
	restored := make([]string, 0, minimumReplacementReservationAttempts+961)
	for index := 0; index < cap(restored); index++ {
		token := fmt.Sprintf("restored-%04d", index)
		require.True(t, reservations.reserve(token, token))
		restored = append(restored, token)
	}
	calls := 0

	token := reservations.tokenForOrReserve("new", func() string {
		defer func() { calls++ }()
		if calls < len(restored) {
			return restored[calls]
		}
		return "fresh-token"
	})

	assert.Equal(t, "fresh-token", token)
	assert.Equal(t, len(restored)+1, calls)
}

func TestReplacementReservationsConcurrentSameCanonical(t *testing.T) {
	reservations := NewReplacementReservations()
	first, err := NewSimpleTrackerMapForObfuscation(
		nil,
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		reservations,
	)
	require.NoError(t, err)
	second, err := NewSimpleTrackerMapForObfuscation(
		nil,
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		reservations,
	)
	require.NoError(t, err)

	start := make(chan struct{})
	var calls atomic.Int32
	results := make(chan string, 2)
	for _, tracker := range []ReplacementTracker{first, second} {
		go func(tracker ReplacementTracker) {
			<-start
			results <- tracker.GenerateIfAbsent("192.0.2.1", "192.0.2.1", 1, func() string {
				calls.Add(1)
				return "x-ipv4-0000000001-x"
			})
		}(tracker)
	}
	close(start)

	for range 2 {
		select {
		case token := <-results:
			assert.Equal(t, "x-ipv4-0000000001-x", token)
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent same-canonical generation did not finish")
		}
	}
	assert.Equal(t, int32(1), calls.Load())
}

func TestReplacementReservationsConcurrentDifferentCanonicals(t *testing.T) {
	const workers = 64
	reservations := NewReplacementReservations()
	start := make(chan struct{})
	results := make(chan string, workers)
	for worker := 0; worker < workers; worker++ {
		go func(worker int) {
			<-start
			calls := 0
			results <- reservations.tokenForOrReserve(
				fmt.Sprintf("canonical-%d", worker),
				func() string {
					calls++
					if calls == 1 {
						return "shared-token"
					}
					return fmt.Sprintf("token-%d", worker)
				},
			)
		}(worker)
	}
	close(start)

	seen := make(map[string]struct{}, workers)
	for range workers {
		select {
		case token := <-results:
			_, duplicate := seen[token]
			assert.False(t, duplicate, "token %q was assigned more than once", token)
			seen[token] = struct{}{}
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent different-canonical generation did not finish")
		}
	}
	assert.Len(t, seen, workers)
}

func TestSimpleTrackerRejectsInvalidIPAndMACReplacementKeys(t *testing.T) {
	for _, tc := range []struct {
		name            string
		obfuscationType schema.ObfuscateType
		replacementKey  string
	}{
		{name: "IP", obfuscationType: schema.ObfuscateTypeIP, replacementKey: "not-an-ip"},
		{name: "MAC", obfuscationType: schema.ObfuscateTypeMAC, replacementKey: "not-a-mac"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewSimpleTrackerMapForObfuscation(
				map[string]string{tc.replacementKey: "token"},
				tc.obfuscationType,
				nil,
				schema.ObfuscateReplacementTypeConsistent,
				nil,
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.replacementKey)
		})
	}
}

func TestSimpleTrackerAcceptsSinglyPercentEncodedMACReplacementKey(t *testing.T) {
	const encodedMAC = "%41%41%3A%41%41%3A%41%41%3A%41%41%3A%41%41%3A%41%41"
	tracker, err := NewSimpleTrackerMapForObfuscation(
		map[string]string{encodedMAC: "mac-token"},
		schema.ObfuscateTypeMAC,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		nil,
	)
	require.NoError(t, err)

	report := tracker.Report().Replacements
	require.Len(t, report, 1)
	assert.Equal(t, "AA:AA:AA:AA:AA:AA", report[0].Canonical)
	assert.Equal(t, map[string]uint{encodedMAC: 0}, report[0].Counter)
}

func TestSimpleTrackerRejectsDoublePercentEncodedMACReplacementKey(t *testing.T) {
	const encodedMAC = "%2541%2541%253A%2541%2541%253A%2541%2541%253A%2541%2541%253A%2541%2541%253A%2541%2541"
	_, err := NewSimpleTrackerMapForObfuscation(
		map[string]string{encodedMAC: "mac-token"},
		schema.ObfuscateTypeMAC,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), encodedMAC)
}

func TestNewSimpleTrackerMapForObfuscationDoesNotReserveFailedBatch(t *testing.T) {
	reservations := NewReplacementReservations()
	_, err := NewSimpleTrackerMapForObfuscation(
		map[string]string{
			"192.0.2.1": "shared-token",
			"not-an-ip": "other-token",
		},
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		reservations,
	)
	require.Error(t, err)

	tracker, err := NewSimpleTrackerMapForObfuscation(
		map[string]string{"198.51.100.7": "shared-token"},
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		reservations,
	)
	require.NoError(t, err)
	report := tracker.Report().Replacements
	require.Len(t, report, 1)
	assert.Equal(t, "198.51.100.7", report[0].Canonical)
	assert.Equal(t, "shared-token", report[0].ReplacedWith)
}

func TestReplacementReservationsImportCrossGroupTokenConflict(t *testing.T) {
	reservations := NewReplacementReservations()
	require.NoError(t, reservations.reserveBatch([]replacementReservation{{
		token: "shared-token", canonical: "192.0.2.1",
	}}))
	require.NoError(t, reservations.reserveBatch([]replacementReservation{{
		token: "shared-token", canonical: "198.51.100.7",
	}}))

	calls := 0
	token := reservations.tokenForOrReserve("192.0.2.1", func() string {
		calls++
		if calls == 1 {
			return "shared-token"
		}
		return "fresh-token"
	})
	assert.Equal(t, "fresh-token", token)
	assert.Equal(t, 2, calls)
}

func TestReplacementReservationsImportCrossGroupCanonicalConflict(t *testing.T) {
	reservations := NewReplacementReservations()
	require.NoError(t, reservations.reserveBatch([]replacementReservation{{
		token: "first-token", canonical: "192.0.2.1",
	}}))
	require.NoError(t, reservations.reserveBatch([]replacementReservation{{
		token: "second-token", canonical: "192.0.2.1",
	}}))

	token := reservations.tokenForOrReserve("192.0.2.1", func() string {
		return "fresh-token"
	})
	assert.Equal(t, "fresh-token", token)
}

func TestReplacementReservationsShareGeneratedTokenForAmbiguousCanonical(t *testing.T) {
	const canonical = "192.0.2.1"
	newAmbiguousReservations := func(t *testing.T) *ReplacementReservations {
		t.Helper()
		reservations := NewReplacementReservations()
		require.NoError(t, reservations.reserveBatch([]replacementReservation{{
			token: "first-token", canonical: canonical,
		}}))
		require.NoError(t, reservations.reserveBatch([]replacementReservation{{
			token: "second-token", canonical: canonical,
		}}))
		require.Contains(t, reservations.ambiguousCanonicals, canonical)
		return reservations
	}

	t.Run("subsequent tracker", func(t *testing.T) {
		reservations := newAmbiguousReservations(t)
		generated := 0
		token := reservations.tokenForOrReserve(canonical, func() string {
			generated++
			switch generated {
			case 1:
				return "first-token"
			case 2:
				return "second-token"
			default:
				return "generated-token"
			}
		})

		require.Equal(t, "generated-token", token)
		require.Equal(t, 3, generated)
		require.Contains(t, reservations.ambiguousCanonicals, canonical)
		require.Equal(t, token, reservations.tokenForOrReserve(canonical, func() string {
			t.Fatal("generator called for a canonical with a generated token")
			return ""
		}))
	})

	t.Run("concurrent trackers", func(t *testing.T) {
		const workers = 32
		reservations := newAmbiguousReservations(t)
		start := make(chan struct{})
		results := make(chan string, workers)
		var generated atomic.Int32
		for range workers {
			go func() {
				<-start
				results <- reservations.tokenForOrReserve(canonical, func() string {
					return fmt.Sprintf("generated-token-%d", generated.Add(1))
				})
			}()
		}
		close(start)

		tokens := make(map[string]struct{}, workers)
		for range workers {
			select {
			case token := <-results:
				tokens[token] = struct{}{}
			case <-time.After(5 * time.Second):
				t.Fatal("concurrent ambiguous-canonical generation did not finish")
			}
		}
		assert.Len(t, tokens, 1)
		assert.Equal(t, int32(1), generated.Load())
		require.Contains(t, reservations.ambiguousCanonicals, canonical)
	})
}

func TestReplacementReservationsShareGeneratedTokenAcrossTrackers(t *testing.T) {
	const canonical = "192.0.2.1"
	reservations := NewReplacementReservations()
	_, err := NewSimpleTrackerMapForObfuscation(
		map[string]string{canonical: "first-token"},
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		reservations,
	)
	require.NoError(t, err)
	_, err = NewSimpleTrackerMapForObfuscation(
		map[string]string{canonical: "second-token"},
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		reservations,
	)
	require.NoError(t, err)
	first, err := NewSimpleTrackerMapForObfuscation(
		nil,
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		reservations,
	)
	require.NoError(t, err)
	second, err := NewSimpleTrackerMapForObfuscation(
		nil,
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		reservations,
	)
	require.NoError(t, err)

	generated := 0
	firstToken := first.GenerateIfAbsent(canonical, canonical, 1, func() string {
		generated++
		return "generated-token"
	})
	secondToken := second.GenerateIfAbsent(canonical, canonical, 1, func() string {
		generated++
		return "generated-token"
	})

	assert.Equal(t, "generated-token", firstToken)
	assert.Equal(t, firstToken, secondToken)
	assert.Equal(t, 1, generated)
}

func TestReplacementReservationsBatchIsAtomic(t *testing.T) {
	for _, tc := range []struct {
		name  string
		batch []replacementReservation
	}{
		{
			name: "same canonical with different tokens",
			batch: []replacementReservation{
				{token: "fresh-token", canonical: "192.0.2.1"},
				{token: "other-token", canonical: "192.0.2.1"},
			},
		},
		{
			name: "same token for different canonicals",
			batch: []replacementReservation{
				{token: "fresh-token", canonical: "192.0.2.1"},
				{token: "fresh-token", canonical: "198.51.100.7"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reservations := NewReplacementReservations()
			err := reservations.reserveBatch(tc.batch)
			require.Error(t, err)

			_, err = NewSimpleTrackerMapForObfuscation(
				map[string]string{"203.0.113.9": "fresh-token"},
				schema.ObfuscateTypeIP,
				nil,
				schema.ObfuscateReplacementTypeConsistent,
				reservations,
			)
			require.NoError(t, err)
		})
	}
}

func TestNewSimpleTrackerMapForObfuscationAllowsEmptyReplacementOutputs(t *testing.T) {
	for _, tc := range []struct {
		name              string
		obfuscationType   schema.ObfuscateType
		replacements      map[string]string
		expectedCanonical []string
	}{
		{
			name:            "IP",
			obfuscationType: schema.ObfuscateTypeIP,
			replacements: map[string]string{
				"192.0.2.1":    "",
				"198.51.100.7": "",
			},
			expectedCanonical: []string{"192.0.2.1", "198.51.100.7"},
		},
		{
			name:            "MAC",
			obfuscationType: schema.ObfuscateTypeMAC,
			replacements: map[string]string{
				"AA:AA:AA:AA:AA:AA": "",
				"BB:BB:BB:BB:BB:BB": "",
			},
			expectedCanonical: []string{"AA:AA:AA:AA:AA:AA", "BB:BB:BB:BB:BB:BB"},
		},
		{
			name:            "Domain",
			obfuscationType: schema.ObfuscateTypeDomain,
			replacements: map[string]string{
				"alpha.example.com": "",
				"beta.example.com":  "",
			},
			expectedCanonical: []string{"alpha.example.com", "beta.example.com"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tracker, err := NewSimpleTrackerMapForObfuscation(
				tc.replacements,
				tc.obfuscationType,
				nil,
				schema.ObfuscateReplacementTypeConsistent,
				nil,
			)
			require.NoError(t, err)

			report := tracker.Report().Replacements
			require.Len(t, report, len(tc.expectedCanonical))
			for i, canonical := range tc.expectedCanonical {
				assert.Equal(t, canonical, report[i].Canonical)
				assert.Empty(t, report[i].ReplacedWith)
				assert.Empty(t, tracker.GenerateIfAbsent(canonical, canonical, 1, func() string {
					t.Fatal("empty configured output must not generate a replacement token")
					return ""
				}))
			}
		})
	}
}

func TestSimpleTrackerInitializeRejectsConflictingCanonicalValues(t *testing.T) {
	tracker := NewSimpleTracker()
	require.Panics(t, func() {
		tracker.Initialize(ReplacementReport{Replacements: []Replacement{
			{Canonical: "192.0.2.1", ReplacedWith: "first", Counter: map[string]uint{"192.0.2.1": 1}},
			{Canonical: "192.0.2.1", ReplacedWith: "second", Counter: map[string]uint{"192%2E0%2E2%2E1": 1}},
		}})
	})
}

func TestSimpleTrackerInitializePanicLeavesStateUnchanged(t *testing.T) {
	reservations := NewReplacementReservations()
	tracker, err := NewSimpleTrackerMapForObfuscation(
		map[string]string{"192.0.2.1": "first-token"},
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
		reservations,
	)
	require.NoError(t, err)
	before := tracker.Report()

	require.Panics(t, func() {
		tracker.Initialize(ReplacementReport{Replacements: []Replacement{
			{Canonical: "198.51.100.7", ReplacedWith: "second-token", Counter: map[string]uint{"198.51.100.7": 1}},
			{Canonical: "203.0.113.9", ReplacedWith: "first-token", Counter: map[string]uint{"203.0.113.9": 1}},
		}})
	})

	assert.Equal(t, before, tracker.Report())
	reservations.lock.Lock()
	_, leakedToken := reservations.reservedTokens["second-token"]
	_, leakedOwner := reservations.owners["second-token"]
	_, leakedCanonical := reservations.byCanonical["198.51.100.7"]
	reservations.lock.Unlock()
	assert.False(t, leakedToken)
	assert.False(t, leakedOwner)
	assert.False(t, leakedCanonical)

	require.NotPanics(t, func() {
		tracker.Initialize(ReplacementReport{})
	})
}

func TestSimpleTrackerZeroValueInitializesMaps(t *testing.T) {
	var tracker SimpleTracker
	assert.Equal(t, "replacement", tracker.GenerateIfAbsent("canonical", "original", 1, func() string {
		return "replacement"
	}))

	var initializedTracker SimpleTracker
	require.NotPanics(t, func() {
		initializedTracker.Initialize(ReplacementReport{Replacements: []Replacement{{
			Canonical:    "canonical",
			ReplacedWith: "replacement",
			Counter:      map[string]uint{"original": 1},
		}}})
	})
}
