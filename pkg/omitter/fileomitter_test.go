package omitter

import (
	"strings"
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
			name:     "log files with path",
			pattern:  "*.log",
			input:    "ingress/pods/application.log",
			expected: true,
		},
		{
			name:     "text file with log pattern",
			pattern:  "*.log",
			input:    "application.log.txt",
			expected: false,
		},
		{
			name:     "path glob",
			pattern:  "release-4.10/ingress_controllers/*/haproxy.*",
			input:    "release-4.10/ingress_controllers/pod1/haproxy.conf",
			expected: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			omitter := NewFilenamePatternOmitter(tc.pattern)
			parts := strings.Split(tc.input, "/")
			omit, err := omitter.File(parts[len(parts)-1], tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expected, omit)
		})
	}
}
