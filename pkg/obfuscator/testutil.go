package obfuscator

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
		assert.ElementsMatch(t, w.Occurrences, g.Occurrences)
	}
}
