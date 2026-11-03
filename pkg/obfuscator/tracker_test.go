package obfuscator

import (
	"sort"
	"testing"
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

func TestSimpleTrackerRejectsTokenSharedByDifferentCanonicalValues(t *testing.T) {
	_, err := NewSimpleTrackerMapForObfuscation(
		map[string]string{
			"192.0.2.1":    "x-ipv4-0000000001-x",
			"198.51.100.7": "x-ipv4-0000000001-x",
		},
		schema.ObfuscateTypeIP,
		nil,
		schema.ObfuscateReplacementTypeConsistent,
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
	)
	require.NoError(t, err)

	report := tracker.Report().Replacements
	require.Len(t, report, 1)
	assert.Equal(t, "192.0.2.1", report[0].Canonical)
	assert.Len(t, report[0].Counter, 3)
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
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), encodedMAC)
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
