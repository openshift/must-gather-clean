package obfuscator

import (
	"testing"

	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
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
providerID: azure:///subscriptions/some-subscription-id/resourcegroups/0
foo: 0
bar: 1
`,
			expectedOutput: `
providerID: azure:///subscriptions/subscription-generous-ostrich/resourcegroups/resourcegroup-touched-monkey
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
short_sub: /subscriptions/subscription-touched-monkey
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
short_rg: /resourcegroups/resourcegroup-touched-monkey
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
short_sr: /providers/Microsoft.Compute/virtualMachineScaleSets/resource-touched-monkey
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
short_np: Microsoft.RedHatOpenShift/hcpOpenShiftClusters/nodePools/resource-touched-monkey
0
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
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
				"https://management.azure.com/subscriptions/subscription-precise-parakeet/resourcegroups/resourcegroup-feasible-magpie/providers/Microsoft.Resources/deployments/resource-generous-ostrich/operationStatuses/subresource-touched-monkey",
				// notice here that we sub the nodePool *and* we sub using the xisting known resource group name
				"The Resource 'Microsoft.RedHatOpenShift/hcpOpenShiftClusters/nodePools/resource-deciding-hyena' under resource group 'resourcegroup-feasible-magpie' does not conform to the naming restriction",
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "64f0619f-ebc2-4156-9d91-c4c781de7e54", ReplacedWith: "subscription-precise-parakeet", Counter: map[string]uint{
					"64f0619f-ebc2-4156-9d91-c4c781de7e54": uint(1),
				}},
				{Canonical: "gpu-nodepools-NC4asT4v3-r79j5l", ReplacedWith: "resourcegroup-feasible-magpie", Counter: map[string]uint{
					"gpu-nodepools-NC4asT4v3-r79j5l": uint(2),
				}},
				{Canonical: "aro-hcp-gpu-nodepool-NC4asT4v3", ReplacedWith: "resource-generous-ostrich", Counter: map[string]uint{
					"aro-hcp-gpu-nodepool-NC4asT4v3": uint(1),
				}},
				{Canonical: "08584458931762048867", ReplacedWith: "subresource-touched-monkey", Counter: map[string]uint{
					"08584458931762048867": uint(1),
				}},
				{Canonical: "np-gpu-NC4asT4v3", ReplacedWith: "resource-deciding-hyena", Counter: map[string]uint{
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
				"https://management.azure.com/subscriptions/subscription-precise-parakeet/resourcegroups/resourcegroup-feasible-magpie/providers/Microsoft.Resources/deployments/resource-generous-ostrich/operationStatuses/subresource-touched-monkey",
				"https://management.azure.com/subscriptions/subscription-precise-parakeet/resourcegroups/resourcegroup-feasible-magpie/providers/MiCrOsOfT.ReSeArChEs/dEpLoYmEnTs/resource-generous-ostrich/oPeRaTiOnStAtUsEs/subresource-touched-monkey",
				// notice here that we sub the nodePool *and* we sub using the xisting known resource group name
				"The Resource 'Microsoft.RedHatOpenShift/hcpOpenShiftClusters/nodePools/resource-deciding-hyena' under resource group 'resourcegroup-feasible-magpie' does not conform to the naming restriction",
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "64f0619f-ebc2-4156-9d91-c4c781de7e54", ReplacedWith: "subscription-precise-parakeet", Counter: map[string]uint{
					"64f0619f-ebc2-4156-9d91-c4c781de7e54": uint(2),
				}},
				{Canonical: "gpu-nodepools-NC4asT4v3-r79j5l", ReplacedWith: "resourcegroup-feasible-magpie", Counter: map[string]uint{
					"gpu-nodepools-NC4asT4v3-r79j5l": uint(3),
				}},
				{Canonical: "aro-hcp-gpu-nodepool-NC4asT4v3", ReplacedWith: "resource-generous-ostrich", Counter: map[string]uint{
					"aro-hcp-gpu-nodepool-NC4asT4v3": uint(2),
				}},
				{Canonical: "08584458931762048867", ReplacedWith: "subresource-touched-monkey", Counter: map[string]uint{
					"08584458931762048867": uint(2),
				}},
				{Canonical: "np-gpu-NC4asT4v3", ReplacedWith: "resource-deciding-hyena", Counter: map[string]uint{
					"np-gpu-NC4asT4v3": uint(1),
				}},
			}},
		},
		{
			name: "double quote terminated resource paths",
			input: []string{
				`"resourceId": "/subscriptions/64f0619f-ebc2-4156-9d91-c4c781de7e54/resourceGroups/gpu-nodepools-NC4asT4v3-r79j5l/providers/Microsoft.Resources/deployments/aro-hcp-gpu-nodepool-NC4asT4v3"`,
				`The Resource "Microsoft.RedHatOpenShift/hcpOpenShiftClusters/nodePools/np-gpu-NC4asT4v3" was not found`,
			},
			output: []string{
				`"resourceId": "/subscriptions/subscription-feasible-magpie/resourcegroups/resourcegroup-generous-ostrich/providers/Microsoft.Resources/deployments/resource-touched-monkey"`,
				`The Resource "Microsoft.RedHatOpenShift/hcpOpenShiftClusters/nodePools/resource-precise-parakeet" was not found`,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "64f0619f-ebc2-4156-9d91-c4c781de7e54", ReplacedWith: "subscription-feasible-magpie", Counter: map[string]uint{
					"64f0619f-ebc2-4156-9d91-c4c781de7e54": uint(1),
				}},
				{Canonical: "gpu-nodepools-NC4asT4v3-r79j5l", ReplacedWith: "resourcegroup-generous-ostrich", Counter: map[string]uint{
					"gpu-nodepools-NC4asT4v3-r79j5l": uint(1),
				}},
				{Canonical: "aro-hcp-gpu-nodepool-NC4asT4v3", ReplacedWith: "resource-touched-monkey", Counter: map[string]uint{
					"aro-hcp-gpu-nodepool-NC4asT4v3": uint(1),
				}},
				{Canonical: "np-gpu-NC4asT4v3", ReplacedWith: "resource-precise-parakeet", Counter: map[string]uint{
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
				"- id: /subscriptions/subscription-precise-parakeet/resourcegroups/resourcegroup-feasible-magpie/providers/Microsoft.Resources/deployments/resource-generous-ostrich/operations/subresource-touched-monkey",
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "64f0619f-ebc2-4156-9d91-c4c781de7e54", ReplacedWith: "subscription-precise-parakeet", Counter: map[string]uint{
					"64f0619f-ebc2-4156-9d91-c4c781de7e54": uint(1),
				}},
				{Canonical: "basic-cluster-k4tbpz", ReplacedWith: "resourcegroup-feasible-magpie", Counter: map[string]uint{
					"basic-cluster-k4tbpz": uint(1),
				}},
				{Canonical: "managed-identities", ReplacedWith: "resource-generous-ostrich", Counter: map[string]uint{
					"managed-identities": uint(1),
				}},
				{Canonical: "ED24FB60AE05A5A5", ReplacedWith: "subresource-touched-monkey", Counter: map[string]uint{
					"ED24FB60AE05A5A5": uint(1),
				}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
			require.NoError(t, err)
			for idx, i := range tc.input {
				output := o.Contents(i)
				assert.Equal(t, tc.output[idx], output)
			}
			replacementReportsMatch(t, tc.report, o.Report())
		})
	}
}
