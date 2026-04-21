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
		{
			name: "common word resource names should not corrupt component names",
			input: []string{
				// First line discovers "service" as a resource name via Azure path
				`/subscriptions/64f0619f-ebc2-4156-9d91-c4c781de7e54/resourceGroups/my-rg-123/providers/Microsoft.ManagedIdentity/userAssignedIdentities/service`,
				// Second line must NOT have "service" replaced in "containerd.service"
				`"systemd_unit":"containerd.service"`,
			},
			output: []string{
				`/subscriptions/subscription-feasible-magpie/resourcegroups/resourcegroup-generous-ostrich/providers/Microsoft.ManagedIdentity/userAssignedIdentities/resource-touched-monkey`,
				// This is the key assertion: "containerd.service" must be preserved
				`"systemd_unit":"containerd.service"`,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "64f0619f-ebc2-4156-9d91-c4c781de7e54", ReplacedWith: "subscription-feasible-magpie", Counter: map[string]uint{
					"64f0619f-ebc2-4156-9d91-c4c781de7e54": uint(1),
				}},
				{Canonical: "my-rg-123", ReplacedWith: "resourcegroup-generous-ostrich", Counter: map[string]uint{
					"my-rg-123": uint(1),
				}},
				{Canonical: "service", ReplacedWith: "resource-touched-monkey", Counter: map[string]uint{
					"service": uint(1),
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

func TestIsGenericWord(t *testing.T) {
	for _, tc := range []struct {
		input    string
		expected bool
	}{
		{"service", true},     // pure lowercase → generic word
		{"proxy", true},       // pure lowercase → generic word
		{"network", true},     // pure lowercase → generic word
		{"GPU", true},         // pure uppercase → generic word
		{"DNS", true},         // pure uppercase → generic word
		{"API", true},         // pure uppercase → generic word
		{"Service", false},    // mixed case → identifier
		{"MyResource", false}, // mixed case → identifier
		{"my-service", false}, // has hyphen → identifier
		{"proxy1", false},     // has digit → identifier
		{"a-b", false},        // has hyphen → identifier
		{"", false},           // empty
		{"node_pool", false},  // has underscore → identifier
	} {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, isGenericWord(tc.input))
		})
	}
}

func TestAzureResourceEdgeCases(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  []string
		output []string
		report ReplacementReport
	}{
		{
			name: "capitalized resource name bypasses isGenericWord and gets replaced globally",
			input: []string{
				`/subscriptions/aaaa-bbbb/resourceGroups/my-rg-123/providers/Microsoft.Compute/disks/Service`,
				`component: Service is running`,
			},
			output: []string{
				`/subscriptions/subscription-feasible-magpie/resourcegroups/resourcegroup-generous-ostrich/providers/Microsoft.Compute/disks/resource-touched-monkey`,
				`component: resource-touched-monkey is running`,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "aaaa-bbbb", ReplacedWith: "subscription-feasible-magpie", Counter: map[string]uint{
					"aaaa-bbbb": 1,
				}},
				{Canonical: "my-rg-123", ReplacedWith: "resourcegroup-generous-ostrich", Counter: map[string]uint{
					"my-rg-123": 1,
				}},
				// count=2: once in the ARM path, once in free-text (mixed case bypasses isGenericWord)
				{Canonical: "Service", ReplacedWith: "resource-touched-monkey", Counter: map[string]uint{
					"Service": 2,
				}},
			}},
		},
		{
			name: "resource name with digit is not a common word and gets replaced",
			input: []string{
				`/subscriptions/aaaa-bbbb/resourceGroups/my-rg-123/providers/Microsoft.Compute/disks/proxy1`,
				`disk proxy1 is attached`,
			},
			output: []string{
				`/subscriptions/subscription-feasible-magpie/resourcegroups/resourcegroup-generous-ostrich/providers/Microsoft.Compute/disks/resource-touched-monkey`,
				`disk resource-touched-monkey is attached`,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "aaaa-bbbb", ReplacedWith: "subscription-feasible-magpie", Counter: map[string]uint{
					"aaaa-bbbb": 1,
				}},
				{Canonical: "my-rg-123", ReplacedWith: "resourcegroup-generous-ostrich", Counter: map[string]uint{
					"my-rg-123": 1,
				}},
				// count=2: once in ARM path, once in free-text (digit makes it non-generic)
				{Canonical: "proxy1", ReplacedWith: "resource-touched-monkey", Counter: map[string]uint{
					"proxy1": 2,
				}},
			}},
		},
		{
			name: "hyphenated resource name is not a common word and gets replaced",
			input: []string{
				`/subscriptions/aaaa-bbbb/resourceGroups/my-rg-123/providers/Microsoft.Compute/disks/my-service`,
				`disk my-service is attached`,
			},
			output: []string{
				`/subscriptions/subscription-feasible-magpie/resourcegroups/resourcegroup-generous-ostrich/providers/Microsoft.Compute/disks/resource-touched-monkey`,
				`disk resource-touched-monkey is attached`,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "aaaa-bbbb", ReplacedWith: "subscription-feasible-magpie", Counter: map[string]uint{
					"aaaa-bbbb": 1,
				}},
				{Canonical: "my-rg-123", ReplacedWith: "resourcegroup-generous-ostrich", Counter: map[string]uint{
					"my-rg-123": 1,
				}},
				// count=2: once in ARM path, once in free-text (hyphen makes it non-generic)
				{Canonical: "my-service", ReplacedWith: "resource-touched-monkey", Counter: map[string]uint{
					"my-service": 2,
				}},
			}},
		},
		{
			name: "lowercase common word resource name is skipped in free-text but replaced in ARM path",
			input: []string{
				`/subscriptions/aaaa-bbbb/resourceGroups/my-rg-123/providers/Microsoft.Network/virtualNetworks/network`,
				`checking network connectivity`,
			},
			output: []string{
				`/subscriptions/subscription-feasible-magpie/resourcegroups/resourcegroup-generous-ostrich/providers/Microsoft.Network/virtualNetworks/resource-touched-monkey`,
				`checking network connectivity`,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "aaaa-bbbb", ReplacedWith: "subscription-feasible-magpie", Counter: map[string]uint{
					"aaaa-bbbb": 1,
				}},
				{Canonical: "my-rg-123", ReplacedWith: "resourcegroup-generous-ostrich", Counter: map[string]uint{
					"my-rg-123": 1,
				}},
				// count=1: ARM path only — "network" is a generic word (pure lowercase), skipped in free-text
				{Canonical: "network", ReplacedWith: "resource-touched-monkey", Counter: map[string]uint{
					"network": 1,
				}},
			}},
		},
		{
			name: "5-char lowercase word (proxy) is common and skipped in free-text",
			input: []string{
				`/subscriptions/aaaa-bbbb/resourceGroups/my-rg-123/providers/Microsoft.Network/applicationGateways/proxy`,
				`kube-proxy is healthy`,
			},
			output: []string{
				`/subscriptions/subscription-feasible-magpie/resourcegroups/resourcegroup-generous-ostrich/providers/Microsoft.Network/applicationGateways/resource-touched-monkey`,
				`kube-proxy is healthy`,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "aaaa-bbbb", ReplacedWith: "subscription-feasible-magpie", Counter: map[string]uint{
					"aaaa-bbbb": 1,
				}},
				{Canonical: "my-rg-123", ReplacedWith: "resourcegroup-generous-ostrich", Counter: map[string]uint{
					"my-rg-123": 1,
				}},
				// count=1: ARM path only — "proxy" is a generic word (pure lowercase), skipped in free-text
				{Canonical: "proxy", ReplacedWith: "resource-touched-monkey", Counter: map[string]uint{
					"proxy": 1,
				}},
			}},
		},
		{
			name: "mixed-case resource name should not replace inside longer tokens",
			input: []string{
				`/subscriptions/aaaa-bbbb/resourceGroups/my-rg-123/providers/Microsoft.Compute/disks/Proxy1`,
				`MyProxy1Handler started, Proxy1 is ready`,
			},
			output: []string{
				`/subscriptions/subscription-feasible-magpie/resourcegroups/resourcegroup-generous-ostrich/providers/Microsoft.Compute/disks/resource-touched-monkey`,
				// "Proxy1" as a standalone word is replaced, but NOT inside "MyProxy1Handler"
				`MyProxy1Handler started, resource-touched-monkey is ready`,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "aaaa-bbbb", ReplacedWith: "subscription-feasible-magpie", Counter: map[string]uint{
					"aaaa-bbbb": 1,
				}},
				{Canonical: "my-rg-123", ReplacedWith: "resourcegroup-generous-ostrich", Counter: map[string]uint{
					"my-rg-123": 1,
				}},
				// count=2: once in ARM path, once as standalone word in free-text (NOT inside "MyProxy1Handler")
				{Canonical: "Proxy1", ReplacedWith: "resource-touched-monkey", Counter: map[string]uint{
					"Proxy1": 2,
				}},
			}},
		},
		{
			name: "all-uppercase resource name (GPU) is generic and skipped in free-text",
			input: []string{
				`/subscriptions/aaaa-bbbb/resourceGroups/my-rg-123/providers/Microsoft.Compute/virtualMachineScaleSets/GPU`,
				`GPU utilization is 80%`,
			},
			output: []string{
				`/subscriptions/subscription-feasible-magpie/resourcegroups/resourcegroup-generous-ostrich/providers/Microsoft.Compute/virtualMachineScaleSets/resource-touched-monkey`,
				`GPU utilization is 80%`,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "aaaa-bbbb", ReplacedWith: "subscription-feasible-magpie", Counter: map[string]uint{
					"aaaa-bbbb": 1,
				}},
				{Canonical: "my-rg-123", ReplacedWith: "resourcegroup-generous-ostrich", Counter: map[string]uint{
					"my-rg-123": 1,
				}},
				// count=1: ARM path only — "GPU" is skipped in free-text (len < 5)
				{Canonical: "GPU", ReplacedWith: "resource-touched-monkey", Counter: map[string]uint{
					"GPU": 1,
				}},
			}},
		},
		{
			name: "multiple free-text occurrences are counted accurately",
			input: []string{
				`/subscriptions/aaaa-bbbb/resourceGroups/my-rg-123/providers/Microsoft.Compute/disks/MyDisk`,
				`MyDisk failed, retrying MyDisk, still failing on MyDisk`,
			},
			output: []string{
				`/subscriptions/subscription-feasible-magpie/resourcegroups/resourcegroup-generous-ostrich/providers/Microsoft.Compute/disks/resource-touched-monkey`,
				`resource-touched-monkey failed, retrying resource-touched-monkey, still failing on resource-touched-monkey`,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "aaaa-bbbb", ReplacedWith: "subscription-feasible-magpie", Counter: map[string]uint{
					"aaaa-bbbb": 1,
				}},
				{Canonical: "my-rg-123", ReplacedWith: "resourcegroup-generous-ostrich", Counter: map[string]uint{
					"my-rg-123": 1,
				}},
				// count=4: once in ARM path + three standalone occurrences in free-text
				{Canonical: "MyDisk", ReplacedWith: "resource-touched-monkey", Counter: map[string]uint{
					"MyDisk": 4,
				}},
			}},
		},
		{
			name: "URL-encoded resource names are replaced despite %22 digit adjacency",
			input: []string{
				`/subscriptions/aaaa-bbbb/resourceGroups/my-rg-123/providers/Microsoft.RedHatOpenShift/hcpOpenShiftClusters/basic-hcp-cluster`,
				`%22api.openshift.com%2Fname%22%3A%22basic-hcp-cluster%22`,
			},
			output: []string{
				`/subscriptions/subscription-feasible-magpie/resourcegroups/resourcegroup-generous-ostrich/providers/Microsoft.RedHatOpenShift/hcpOpenShiftClusters/resource-touched-monkey`,
				`%22api.openshift.com%2Fname%22%3A%22resource-touched-monkey%22`,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "aaaa-bbbb", ReplacedWith: "subscription-feasible-magpie", Counter: map[string]uint{
					"aaaa-bbbb": 1,
				}},
				{Canonical: "my-rg-123", ReplacedWith: "resourcegroup-generous-ostrich", Counter: map[string]uint{
					"my-rg-123": 1,
				}},
				// count=2: once in ARM path, once in URL-encoded free-text
				{Canonical: "basic-hcp-cluster", ReplacedWith: "resource-touched-monkey", Counter: map[string]uint{
					"basic-hcp-cluster": 2,
				}},
			}},
		},
		{
			name: "underscore-prefixed subscription ID is replaced",
			input: []string{
				`/subscriptions/aaaa-bbbb/resourceGroups/my-rg-123/providers/Microsoft.Compute/disks/MyDisk`,
				`"kubernetes.azure.com/managedby":"sub_aaaa-bbbb"`,
			},
			output: []string{
				`/subscriptions/subscription-feasible-magpie/resourcegroups/resourcegroup-generous-ostrich/providers/Microsoft.Compute/disks/resource-touched-monkey`,
				`"kubernetes.azure.com/managedby":"sub_subscription-feasible-magpie"`,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "aaaa-bbbb", ReplacedWith: "subscription-feasible-magpie", Counter: map[string]uint{
					"aaaa-bbbb": 2,
				}},
				{Canonical: "my-rg-123", ReplacedWith: "resourcegroup-generous-ostrich", Counter: map[string]uint{
					"my-rg-123": 1,
				}},
				{Canonical: "MyDisk", ReplacedWith: "resource-touched-monkey", Counter: map[string]uint{
					"MyDisk": 1,
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
