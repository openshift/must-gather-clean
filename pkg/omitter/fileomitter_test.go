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
		{
			name:     "real world sdn glob",
			pattern:  "*/namespaces/openshift-sdn/pods/*/*/*/logs/*.log",
			input:    "quay-io-openshift-release-dev-ocp-v4-0-art-dev-sha256-47c2f751ab0d5ee88e2826749f1372e6a24db3d0c0c942136ae84db17cb7f086/namespaces/openshift-sdn/pods/ovn-2vqtd/openvswitch/openvswitch/logs/current.log",
			expected: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			omitter, err := NewFilenamePatternOmitter(tc.pattern)
			require.NoError(t, err)
			parts := strings.Split(tc.input, "/")
			omit, err := omitter.Omit(parts[len(parts)-1], tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expected, omit)
		})
	}
}

func TestEmptyPattern(t *testing.T) {
	_, err := NewFilenamePatternOmitter("")
	require.Error(t, err)
}
