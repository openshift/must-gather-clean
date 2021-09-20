package obfuscator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewKeywordsObfuscator(t *testing.T) {
	for _, tc := range []struct {
		name           string
		replacements   map[string]string
		input          string
		expectedOutput string
		expectLegend   ReplacementReport
	}{
		{
			name: "basic",
			replacements: map[string]string{
				"unique-word": "replacement",
			},
			input:          "input with unique-word",
			expectedOutput: "input with replacement",
			expectLegend: ReplacementReport{[]Replacement{
				{Original: "unique-word", Replaced: "replacement", Total: 1},
			}},
		},
		{
			name: "no replacement",
			replacements: map[string]string{
				"unique-word": "replacement",
			},
			input:          "input with common words",
			expectedOutput: "input with common words",
			expectLegend:   ReplacementReport{[]Replacement{}},
		},
		{
			name: "partial replacement",
			replacements: map[string]string{
				"first-unique":  "first-replacement",
				"second-unique": "second-replacement",
			},
			input:          "input with first-unique word",
			expectedOutput: "input with first-replacement word",
			expectLegend: ReplacementReport{[]Replacement{
				{Original: "first-unique", Replaced: "first-replacement", Total: 1},
			}},
		},
		{
			name: "partial replacement with repetition",
			replacements: map[string]string{
				"foo": "four",
				"bar": "zero",
			},
			input:          "input with foo foo foo times foo",
			expectedOutput: "input with four four four times four",
			expectLegend: ReplacementReport{[]Replacement{
				{Original: "foo", Replaced: "four", Total: 4},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := NewKeywordsObfuscator(tc.replacements)
			require.Equal(t, tc.expectedOutput, o.Contents(tc.input))
			require.Equal(t, tc.expectLegend, o.Report())
		})
	}
}
