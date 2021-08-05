package omitter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileOmitter(t *testing.T) {
	for _, tc := range []struct {
		name     string
		pattern  string
		input    string
		expected bool
	}{
		{
			name:     "log files",
			pattern:  "*.log",
			input:    "application.log",
			expected: true,
		},
		{
			name:     "text file with log pattern",
			pattern:  "*.log",
			input:    "application.log.txt",
			expected: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			omitter := NewFilenamePatternOmitter(tc.pattern)
			omit, err := omitter.File(tc.input, "")
			require.NoError(t, err)
			require.Equal(t, tc.expected, omit)
		})
	}
}
