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
		expectLegend   map[string]string
	}{
		{
			name: "basic",
			replacements: map[string]string{
				"unique-word": "replacement",
			},
			input:          "input with unique-word",
			expectedOutput: "input with replacement",
			expectLegend:   map[string]string{"unique-word": "replacement"},
		},
		{
			name: "no replacement",
			replacements: map[string]string{
				"unique-word": "replacement",
			},
			input:          "input with common words",
			expectedOutput: "input with common words",
			expectLegend:   map[string]string{},
		},
		{
			name: "partial replacement",
			replacements: map[string]string{
				"first-unique":  "first-replacement",
				"second-unique": "second-replacement",
			},
			input:          "input with first-unique word",
			expectedOutput: "input with first-replacement word",
			expectLegend:   map[string]string{"first-unique": "first-replacement"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := NewKeywordsObfuscator(tc.replacements)
			require.Equal(t, tc.expectedOutput, o.Contents(tc.input))
			require.Equal(t, tc.expectLegend, o.Report())
		})
	}
}
