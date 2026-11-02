package obfuscator

import (
	"sort"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
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

func TestSimpleTrackerReportReturnsSnapshot(t *testing.T) {
	tracker := NewSimpleTracker()
	tracker.GenerateIfAbsent("canonical", "original", 1, func() string { return "replacement" })

	report := tracker.Report()
	tracker.GenerateIfAbsent("canonical", "original", 1, func() string { return "replacement" })
	report.Replacements[0].Counter["caller-change"] = 1

	assert.Equal(t, map[string]uint{"original": 2}, tracker.Report().Replacements[0].Counter)
	assert.Equal(t, map[string]uint{"original": 1, "caller-change": 1}, report.Replacements[0].Counter)
}
