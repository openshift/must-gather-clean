package obfuscator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegexObfuscator(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pattern string
		input   string
		output  string
		report  map[string]string
	}{
		{
			name:    "match-basic",
			pattern: `(super\-)+(secret)`,
			input:   "line with a super-super-secret, super-secret and a non-secret",
			output:  "line with a xxxxxxxxxxxxxxxxxx, xxxxxxxxxxxx and a non-secret",
			report: map[string]string{
				"super-secret": "xxxxxxxxxxxx", "super-super-secret": "xxxxxxxxxxxxxxxxxx",
			},
		},
		{
			name:    "match-everything",
			pattern: `.*`,
			input:   "line with a super-super-secret, super-secret and a non-secret",
			output:  "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			report: map[string]string{
				"line with a super-super-secret, super-secret and a non-secret": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			},
		},
		{
			name:    "match-nothing",
			pattern: `(super\-)+(secret)`,
			input:   "no secrets here",
			output:  "no secrets here",
			report:  map[string]string{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewRegexObfuscator(tc.pattern)
			require.NoError(t, err)
			output := o.Contents(tc.input)
			assert.Equal(t, tc.output, output)
			assert.Equal(t, tc.report, o.Report())
		})
	}
}
