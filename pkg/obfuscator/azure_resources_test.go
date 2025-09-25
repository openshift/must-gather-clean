package obfuscator

import (
	"testing"

	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAzureResourcesObfuscatorContents(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  []string
		output []string
		report ReplacementReport
	}{
		{
			name: "basic",
			input: []string{
				"https://management.azure.com/subscriptions/64f0619f-ebc2-4156-9d91-c4c781de7e54/resourcegroups/gpu-nodepools-NC4asT4v3-r79j5l/providers/Microsoft.Resources/deployments/aro-hcp-gpu-nodepool-NC4asT4v3/operationStatuses/08584458931762048867",
				"The Resource 'Microsoft.RedHatOpenShift/hcpOpenShiftClusters/nodePools/np-gpu-NC4asT4v3' under resource group 'gpu-nodepools-NC4asT4v3-r79j5l' does not conform to the naming restriction",
			},
			output: []string{
				"https://management.azure.com/subscriptions/x-subscription-0000000001-x/resourcegroups/x-resourcegroup-0000000001-x/providers/Microsoft.Resources/deployments/x-resource-0000000001-x/operationStatuses/x-subresource-0000000001-x",
				// notice here that we sub the nodePool *and* we sub using the xisting known resource group name
				"The Resource 'Microsoft.RedHatOpenShift/hcpOpenShiftClusters/nodePools/x-resource-0000000002-x' under resource group 'x-resourcegroup-0000000001-x' does not conform to the naming restriction",
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "64f0619f-ebc2-4156-9d91-c4c781de7e54", ReplacedWith: "x-subscription-0000000001-x", Counter: map[string]uint{
					"64f0619f-ebc2-4156-9d91-c4c781de7e54": uint(1),
				}},
				{Canonical: "gpu-nodepools-NC4asT4v3-r79j5l", ReplacedWith: "x-resourcegroup-0000000001-x", Counter: map[string]uint{
					"gpu-nodepools-NC4asT4v3-r79j5l": uint(2),
				}},
				{Canonical: "aro-hcp-gpu-nodepool-NC4asT4v3", ReplacedWith: "x-resource-0000000001-x", Counter: map[string]uint{
					"aro-hcp-gpu-nodepool-NC4asT4v3": uint(1),
				}},
				{Canonical: "08584458931762048867", ReplacedWith: "x-subresource-0000000001-x", Counter: map[string]uint{
					"08584458931762048867": uint(1),
				}},
				{Canonical: "np-gpu-NC4asT4v3", ReplacedWith: "x-resource-0000000002-x", Counter: map[string]uint{
					"np-gpu-NC4asT4v3": uint(1),
				}},
			}},
		},
		{
			name: "prove case insensitivity",
			input: []string{
				"https://management.azure.com/subscriptions/64f0619f-ebc2-4156-9d91-c4c781de7e54/resourcegroups/gpu-nodepools-NC4asT4v3-r79j5l/providers/Microsoft.Resources/deployments/aro-hcp-gpu-nodepool-NC4asT4v3/operationStatuses/08584458931762048867",
				"https://management.azure.com/SUbScRiPtIoNs/64f0619f-ebc2-4156-9d91-c4c781de7e54/rEsOuRcEgRoUpS/gpu-nodepools-NC4asT4v3-r79j5l/PrOvIdErS/MiCrOsOfT.ReSeArChEs/dEpLoYmEnTs/aro-hcp-gpu-nodepool-NC4asT4v3/oPeRaTiOnStAtUsEs/08584458931762048867",
				"The Resource 'Microsoft.RedHatOpenShift/hcpOpenShiftClusters/nodePools/np-gpu-NC4asT4v3' under resource group 'gpu-nodepools-NC4asT4v3-r79j5l' does not conform to the naming restriction",
			},
			output: []string{
				"https://management.azure.com/subscriptions/x-subscription-0000000001-x/resourcegroups/x-resourcegroup-0000000001-x/providers/Microsoft.Resources/deployments/x-resource-0000000001-x/operationStatuses/x-subresource-0000000001-x",
				"https://management.azure.com/subscriptions/x-subscription-0000000001-x/resourcegroups/x-resourcegroup-0000000001-x/providers/MiCrOsOfT.ReSeArChEs/dEpLoYmEnTs/x-resource-0000000001-x/oPeRaTiOnStAtUsEs/x-subresource-0000000001-x",
				// notice here that we sub the nodePool *and* we sub using the xisting known resource group name
				"The Resource 'Microsoft.RedHatOpenShift/hcpOpenShiftClusters/nodePools/x-resource-0000000002-x' under resource group 'x-resourcegroup-0000000001-x' does not conform to the naming restriction",
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "64f0619f-ebc2-4156-9d91-c4c781de7e54", ReplacedWith: "x-subscription-0000000001-x", Counter: map[string]uint{
					"64f0619f-ebc2-4156-9d91-c4c781de7e54": uint(2),
				}},
				{Canonical: "gpu-nodepools-NC4asT4v3-r79j5l", ReplacedWith: "x-resourcegroup-0000000001-x", Counter: map[string]uint{
					"gpu-nodepools-NC4asT4v3-r79j5l": uint(3),
				}},
				{Canonical: "aro-hcp-gpu-nodepool-NC4asT4v3", ReplacedWith: "x-resource-0000000001-x", Counter: map[string]uint{
					"aro-hcp-gpu-nodepool-NC4asT4v3": uint(2),
				}},
				{Canonical: "08584458931762048867", ReplacedWith: "x-subresource-0000000001-x", Counter: map[string]uint{
					"08584458931762048867": uint(2),
				}},
				{Canonical: "np-gpu-NC4asT4v3", ReplacedWith: "x-resource-0000000002-x", Counter: map[string]uint{
					"np-gpu-NC4asT4v3": uint(1),
				}},
			}},
		},
		{
			name: "managed identities bug",
			input: []string{
				"- id: /subscriptions/64f0619f-ebc2-4156-9d91-c4c781de7e54/resourceGroups/basic-cluster-k4tbpz/providers/Microsoft.Resources/deployments/managed-identities/operations/ED24FB60AE05A5A5",
			},
			output: []string{
				"- id: /subscriptions/x-subscription-0000000001-x/resourcegroups/x-resourcegroup-0000000001-x/providers/Microsoft.Resources/deployments/x-resource-0000000001-x/operations/x-subresource-0000000001-x",
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "64f0619f-ebc2-4156-9d91-c4c781de7e54", ReplacedWith: "x-subscription-0000000001-x", Counter: map[string]uint{
					"64f0619f-ebc2-4156-9d91-c4c781de7e54": uint(1),
				}},
				{Canonical: "basic-cluster-k4tbpz", ReplacedWith: "x-resourcegroup-0000000001-x", Counter: map[string]uint{
					"basic-cluster-k4tbpz": uint(1),
				}},
				{Canonical: "managed-identities", ReplacedWith: "x-resource-0000000001-x", Counter: map[string]uint{
					"managed-identities": uint(1),
				}},
				{Canonical: "ED24FB60AE05A5A5", ReplacedWith: "x-subresource-0000000001-x", Counter: map[string]uint{
					"ED24FB60AE05A5A5": uint(1),
				}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)
			for idx, i := range tc.input {
				output := o.Contents(i)
				assert.Equal(t, tc.output[idx], output)
			}
			replacementReportsMatch(t, tc.report, o.Report())
		})
	}
}

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
				"Substring format: 1234567890abcdefghijklmnopqrstuv-extra", // cluster ID followed by non-hex
			},
			output: []string{
				"This is just regular text",
				"No cluster IDs here: short-string",
				"Substring format: x-obfuscated-clusterid-0000001-x-extra",
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
		{
			name: "sha256 value",
			input: []string{
				"53b7ae0a5dac8c542073e69d1acc1b30cf21d367ab7718d9aca1494feed5fdbd",
			},
			output: []string{
				"53b7ae0a5dac8c542073e69d1acc1b30cf21d367ab7718d9aca1494feed5fdbd",
			},
			report: ReplacementReport{[]Replacement{}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
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
			o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
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
				"Cluster: " + staticAzureClusterIDReplacement,
				"Another cluster: " + staticAzureClusterIDReplacement,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "abcdef1234567890abcdef1234567890", ReplacedWith: staticAzureClusterIDReplacement, Counter: map[string]uint{
					"abcdef1234567890abcdef1234567890": uint(1),
				}},
				{Canonical: "fedcba0987654321fedcba0987654321", ReplacedWith: staticAzureClusterIDReplacement, Counter: map[string]uint{
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
				"Primary: " + staticAzureClusterIDReplacement,
				"Backup of " + staticAzureClusterIDReplacement + " completed",
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "1234567890abcdef1234567890abcdef", ReplacedWith: staticAzureClusterIDReplacement, Counter: map[string]uint{
					"1234567890abcdef1234567890abcdef": uint(2),
				}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeStatic, NewSimpleTracker())
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
			o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)
			output := o.Contents(tc.input)
			assert.Equal(t, tc.output, output)
		})
	}
}
