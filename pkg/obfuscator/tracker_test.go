package obfuscator

import (
	"sort"
	"testing"

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
