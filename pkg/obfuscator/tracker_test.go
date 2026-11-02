package obfuscator

import (
	"sort"
	"sync"
	"sync/atomic"
	"testing"

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
	var waitGroup sync.WaitGroup
	for _, tracker := range []ReplacementTracker{first, second} {
		waitGroup.Add(1)
		go func(tracker ReplacementTracker) {
			defer waitGroup.Done()
			<-start
			results <- tracker.GenerateIfAbsent("192.0.2.1", "192.0.2.1", 1, func() string {
				calls.Add(1)
				return "x-ipv4-0000000001-x"
			})
		}(tracker)
	}
	close(start)
	waitGroup.Wait()
	close(results)

	for token := range results {
		assert.Equal(t, "x-ipv4-0000000001-x", token)
	}
	assert.Equal(t, int32(1), calls.Load())
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

func TestSimpleTrackerInitializeRejectsConflictingCanonicalValues(t *testing.T) {
	tracker := NewSimpleTracker()
	require.Panics(t, func() {
		tracker.Initialize(ReplacementReport{Replacements: []Replacement{
			{Canonical: "192.0.2.1", ReplacedWith: "first", Counter: map[string]uint{"192.0.2.1": 1}},
			{Canonical: "192.0.2.1", ReplacedWith: "second", Counter: map[string]uint{"192%2E0%2E2%2E1": 1}},
		}})
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
