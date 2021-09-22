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
				{Canonical: "unique-word", ReplacedWith: "replacement",
					Counter: map[string]uint{
						"unique-word": 1,
					}},
			}},
		},
		{
			name: "no replacement",
			replacements: map[string]string{
				"unique-word": "replacement",
			},
			input:          "input with common words",
			expectedOutput: "input with common words",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "unique-word", ReplacedWith: "replacement",
					Counter: map[string]uint{
						"unique-word": 0,
					}},
			}},
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
				{Canonical: "first-unique", ReplacedWith: "first-replacement",
					Counter: map[string]uint{
						"first-unique": 1,
					}},
				{Canonical: "second-unique", ReplacedWith: "second-replacement",
					Counter: map[string]uint{
						"second-unique": 0,
					}},
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
				{Canonical: "foo", ReplacedWith: "four",
					Counter: map[string]uint{
						"foo": 4,
					}},
				{Canonical: "bar", ReplacedWith: "zero",
					Counter: map[string]uint{
						"bar": 0,
					}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := NewKeywordsObfuscator(tc.replacements)
			require.Equal(t, tc.expectedOutput, o.Contents(tc.input))
			replacementReportsMatch(t, tc.expectLegend, o.Report())
		})
	}
}
