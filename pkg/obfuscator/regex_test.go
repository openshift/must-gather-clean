package obfuscator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/must-gather-clean/pkg/schema"
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
			o, err := NewRegexObfuscator(tc.pattern, schema.ObfuscateTargetAll)
			require.NoError(t, err)
			output := o.Contents(tc.input)
			assert.Equal(t, tc.output, output)
			assert.Equal(t, tc.report, o.Report())
		})
	}
}

func TestRegexObfuscationTarget(t *testing.T) {
	for _, tc := range []struct {
		name              string
		target            schema.ObfuscateTarget
		fileNameObfuscate bool
		contentObfuscate  bool
	}{
		{
			name:              "filename only",
			target:            schema.ObfuscateTargetFileName,
			fileNameObfuscate: true,
		},
		{
			name:             "content only",
			target:           schema.ObfuscateTargetFileContents,
			contentObfuscate: true,
		},
		{
			name:              "both file and content",
			target:            schema.ObfuscateTargetAll,
			fileNameObfuscate: true,
			contentObfuscate:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewRegexObfuscator("secret-word", tc.target)
			require.NoError(t, err)
			output := o.FileName("secret-word")
			if tc.fileNameObfuscate {
				assert.NotEqual(t, "secret-word", output)
			} else {
				assert.Equal(t, "secret-word", output)
			}
			output = o.Contents("secret-word")
			if tc.contentObfuscate {
				assert.NotEqual(t, "secret-word", output)
			} else {
				assert.Equal(t, "secret-word", output)
			}
		})
	}
}
