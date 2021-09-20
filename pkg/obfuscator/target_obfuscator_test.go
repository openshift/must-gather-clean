package obfuscator

import (
	"testing"

	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegexObfuscationTarget(t *testing.T) {
	for _, tc := range []struct {
		name              string
		target            schema.ObfuscateTarget
		fileNameObfuscate bool
		contentObfuscate  bool
	}{
		{
			name:              "filename only",
			target:            schema.ObfuscateTargetFilePath,
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
			obfuscator, err := NewRegexObfuscator("secret-word", map[string]string{})
			require.NoError(t, err)
			o := NewTargetObfuscator(tc.target, obfuscator)

			output := o.Path("secret-word")
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
