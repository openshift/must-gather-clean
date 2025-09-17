package obfuscator

import (
	"testing"

	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoNotReplaceShortStrings(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedOutput string
	}{
		{
			name: "short_strings_not_replaced",
			input: `
providerID: azure:///subscriptions/x-subscription-0000000001-x/resourcegroups/0
foo: 0
bar: 1
`,
			expectedOutput: `
providerID: azure:///subscriptions/x-subscription-0000000001-x/resourcegroups/x-resourcegroup-0000000001-x
foo: 0
bar: 1
`,
		},
		{
			name: "azure_subscription_pattern",
			input: `
short_sub: /subscriptions/0
0
`,
			expectedOutput: `
short_sub: /subscriptions/x-subscription-0000000001-x
0
`,
		},
		{
			name: "azure_resource_group_pattern",
			input: `
short_rg: /resourceGroups/0
0
`,
			expectedOutput: `
short_rg: /resourcegroups/x-resourcegroup-0000000001-x
0
`,
		},
		{
			name: "azure_subresource_pattern",
			input: `
short_sr: /providers/Microsoft.Compute/virtualMachineScaleSets/0
0
`,
			expectedOutput: `
short_sr: /providers/Microsoft.Compute/virtualMachineScaleSets/x-resource-0000000001-x
0
`,
		},
		{
			name: "azure_node_pool_pattern",
			input: `
short_np: Microsoft.RedHatOpenShift/hcpOpenShiftClusters/nodePools/0
0
`,
			expectedOutput: `
short_np: Microsoft.RedHatOpenShift/hcpOpenShiftClusters/nodePools/x-resource-0000000001-x
0
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			actualOutput := o.Contents(tt.input)
			assert.Equal(t, tt.expectedOutput, actualOutput)
		})
	}
}

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
