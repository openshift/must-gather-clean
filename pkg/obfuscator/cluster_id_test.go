package obfuscator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/must-gather-clean/pkg/schema"
)

func TestClusterIDObfuscatorContents(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  []string
		output []string
		report ReplacementReport
	}{
		{
			name: "basic cluster ID replacement",
			input: []string{
				"Cluster ID: 1234567890abcdefghijklmnopqrstuv",
				"Processing cluster 9876543210zyxwvutsrqponmlkjihgfe",
				"Processing cluster 1234567890abcdefghijklmnopqrstuv",
			},
			output: []string{
				"Cluster ID: x-obfuscated-clusterid-0000001-x",
				"Processing cluster 9876543210zyxwvutsrqponmlkjihgfe",
				"Processing cluster x-obfuscated-clusterid-0000001-x",
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "1234567890abcdefghijklmnopqrstuv", ReplacedWith: "x-obfuscated-clusterid-0000001-x", Counter: map[string]uint{
					"1234567890abcdefghijklmnopqrstuv": uint(2),
				}},
			}},
		},
		{
			name: "multiple occurrences of same cluster ID",
			input: []string{
				"Cluster: abcdef1234567890abcdef1234567890 - Status: Running",
				"The cluster abcdef1234567890abcdef1234567890 is healthy",
			},
			output: []string{
				"Cluster: x-obfuscated-clusterid-0000001-x - Status: Running",
				"The cluster x-obfuscated-clusterid-0000001-x is healthy",
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "abcdef1234567890abcdef1234567890", ReplacedWith: "x-obfuscated-clusterid-0000001-x", Counter: map[string]uint{
					"abcdef1234567890abcdef1234567890": uint(2),
				}},
			}},
		},
		{
			name: "cluster ID in JSON",
			input: []string{
				`{"clusterId": "0123456789abcdef0123456789abcdef", "status": "active"}`,
			},
			output: []string{
				`{"clusterId": "x-obfuscated-clusterid-0000001-x", "status": "active"}`,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "0123456789abcdef0123456789abcdef", ReplacedWith: "x-obfuscated-clusterid-0000001-x", Counter: map[string]uint{
					"0123456789abcdef0123456789abcdef": uint(1),
				}},
			}},
		},
		{
			name: "match cluster ID substring",
			input: []string{
				"This is just regular text",
				"No cluster IDs here: short-string",
				"Substring format: 1234567890abcdefghijklmnopqrstuv123", // too long
			},
			output: []string{
				"This is just regular text",
				"No cluster IDs here: short-string",
				"Substring format: x-obfuscated-clusterid-0000001-x123",
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "1234567890abcdefghijklmnopqrstuv", ReplacedWith: "x-obfuscated-clusterid-0000001-x", Counter: map[string]uint{
					"1234567890abcdefghijklmnopqrstuv": uint(1),
				}},
			}},
		},
		{
			name: "mixed valid and invalid formats",
			input: []string{
				"Valid: abcdef0123456789abcdef0123456789 Invalid: xyz",
				"Another valid: fedcba9876543210fedcba9876543210",
			},
			output: []string{
				"Valid: x-obfuscated-clusterid-0000001-x Invalid: xyz",
				"Another valid: x-obfuscated-clusterid-0000002-x",
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "abcdef0123456789abcdef0123456789", ReplacedWith: "x-obfuscated-clusterid-0000001-x", Counter: map[string]uint{
					"abcdef0123456789abcdef0123456789": uint(1),
				}},
				{Canonical: "fedcba9876543210fedcba9876543210", ReplacedWith: "x-obfuscated-clusterid-0000002-x", Counter: map[string]uint{
					"fedcba9876543210fedcba9876543210": uint(1),
				}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewClusterIDObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)
			for idx, i := range tc.input {
				output := o.Contents(i)
				assert.Equal(t, tc.output[idx], output)
			}
			replacementReportsMatch(t, tc.report, o.Report())
		})
	}
}

func TestClusterIDObfuscator_Path(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  string
		output string
		report ReplacementReport
	}{
		{
			name:   "cluster ID in file path",
			input:  "logs/cluster-abcdef0123456789abcdef0123456789/pods",
			output: "logs/cluster-x-obfuscated-clusterid-0000001-x/pods",
			report: ReplacementReport{[]Replacement{
				{Canonical: "abcdef0123456789abcdef0123456789", ReplacedWith: "x-obfuscated-clusterid-0000001-x", Counter: map[string]uint{
					"abcdef0123456789abcdef0123456789": uint(1),
				}},
			}},
		},
		{
			name:   "cluster ID in filename",
			input:  "cluster_fedcba9876543210fedcba9876543210.log",
			output: "cluster_x-obfuscated-clusterid-0000001-x.log",
			report: ReplacementReport{[]Replacement{
				{Canonical: "fedcba9876543210fedcba9876543210", ReplacedWith: "x-obfuscated-clusterid-0000001-x", Counter: map[string]uint{
					"fedcba9876543210fedcba9876543210": uint(1),
				}},
			}},
		},
		{
			name:   "no cluster ID in path",
			input:  "logs/regular-cluster-name/pods",
			output: "logs/regular-cluster-name/pods",
			report: ReplacementReport{[]Replacement{}},
		},
		{
			name:   "multiple cluster IDs in path",
			input:  "backup/1234567890abcdef1234567890abcdef/restore/fedcba0987654321fedcba0987654321/data",
			output: "backup/x-obfuscated-clusterid-0000001-x/restore/x-obfuscated-clusterid-0000002-x/data",
			report: ReplacementReport{[]Replacement{
				{Canonical: "1234567890abcdef1234567890abcdef", ReplacedWith: "x-obfuscated-clusterid-0000001-x", Counter: map[string]uint{
					"1234567890abcdef1234567890abcdef": uint(1),
				}},
				{Canonical: "fedcba0987654321fedcba0987654321", ReplacedWith: "x-obfuscated-clusterid-0000002-x", Counter: map[string]uint{
					"fedcba0987654321fedcba0987654321": uint(1),
				}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewClusterIDObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)
			output := o.Path(tc.input)
			assert.Equal(t, tc.output, output)
			replacementReportsMatch(t, tc.report, o.Report())
		})
	}
}

func TestClusterIDObfuscatorStatic(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  []string
		output []string
		report ReplacementReport
	}{
		{
			name: "static replacement for cluster IDs",
			input: []string{
				"Cluster: abcdef1234567890abcdef1234567890",
				"Another cluster: fedcba0987654321fedcba0987654321",
			},
			output: []string{
				"Cluster: " + staticClusterIDReplacement,
				"Another cluster: " + staticClusterIDReplacement,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "abcdef1234567890abcdef1234567890", ReplacedWith: staticClusterIDReplacement, Counter: map[string]uint{
					"abcdef1234567890abcdef1234567890": uint(1),
				}},
				{Canonical: "fedcba0987654321fedcba0987654321", ReplacedWith: staticClusterIDReplacement, Counter: map[string]uint{
					"fedcba0987654321fedcba0987654321": uint(1),
				}},
			}},
		},
		{
			name: "multiple occurrences with static replacement",
			input: []string{
				"Primary: 1234567890abcdef1234567890abcdef",
				"Backup of 1234567890abcdef1234567890abcdef completed",
			},
			output: []string{
				"Primary: " + staticClusterIDReplacement,
				"Backup of " + staticClusterIDReplacement + " completed",
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "1234567890abcdef1234567890abcdef", ReplacedWith: staticClusterIDReplacement, Counter: map[string]uint{
					"1234567890abcdef1234567890abcdef": uint(2),
				}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewClusterIDObfuscator(schema.ObfuscateReplacementTypeStatic, NewSimpleTracker())
			require.NoError(t, err)
			for idx, i := range tc.input {
				output := o.Contents(i)
				assert.Equal(t, tc.output[idx], output)
			}
			replacementReportsMatch(t, tc.report, o.Report())
		})
	}
}

func TestClusterIDObfuscatorEdgeCases(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "cluster ID at start of string",
			input:  "abcdef1234567890abcdef1234567890 is the cluster",
			output: "x-obfuscated-clusterid-0000001-x is the cluster",
		},
		{
			name:   "cluster ID at end of string",
			input:  "The cluster is abcdef1234567890abcdef1234567890",
			output: "The cluster is x-obfuscated-clusterid-0000001-x",
		},
		{
			name:   "cluster ID with special characters around",
			input:  "cluster_id=\"abcdef1234567890abcdef1234567890\"",
			output: "cluster_id=\"x-obfuscated-clusterid-0000001-x\"",
		},
		{
			name:   "cluster ID in URL",
			input:  "https://api.cluster.com/v1/clusters/abcdef1234567890abcdef1234567890/status",
			output: "https://api.cluster.com/v1/clusters/x-obfuscated-clusterid-0000001-x/status",
		},
		{
			name:   "invalid characters in cluster ID",
			input:  "wxyz567890abcdefghijklmnopqrstuv", // 'w', 'x', 'y', 'z' are not valid in OCM cluster ID pattern
			output: "wxyz567890abcdefghijklmnopqrstuv", // should not be replaced
		},
		{
			name:   "too short",
			input:  "abcdef1234567890abcdef123456789", // 31 characters instead of 32
			output: "abcdef1234567890abcdef123456789", // should not be replaced
		},
		{
			name:   "uppercase letters",
			input:  "ABCDEF1234567890abcdef1234567890", // has uppercase letters
			output: "ABCDEF1234567890abcdef1234567890", // should not be replaced (OCM pattern is lowercase only)
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewClusterIDObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)
			output := o.Contents(tc.input)
			assert.Equal(t, tc.output, output)
		})
	}
}

func TestClusterIDObfuscatorCreation(t *testing.T) {
	t.Run("successful creation with consistent type", func(t *testing.T) {
		o, err := NewClusterIDObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
		require.NoError(t, err)
		require.NotNil(t, o)
	})

	t.Run("successful creation with static type", func(t *testing.T) {
		o, err := NewClusterIDObfuscator(schema.ObfuscateReplacementTypeStatic, NewSimpleTracker())
		require.NoError(t, err)
		require.NotNil(t, o)
	})
}
