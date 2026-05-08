package obfuscator

import (
	"regexp"
	"strings"
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

func TestIsPlainWord(t *testing.T) {
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
		{"a", true},           // single lowercase letter
		{"Z", true},           // single uppercase letter
		{"1", false},          // single digit
	} {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, isPlainWord(tc.input))
		})
	}
}

func TestReplaceStandalone(t *testing.T) {
	for _, tc := range []struct {
		name          string
		input         string
		old           string
		repl          string
		expectedCount uint
		expectedOut   string
	}{
		{
			name:          "standalone with spaces",
			input:         "stg-east-vnet is ready",
			old:           "stg-east-vnet",
			repl:          "REPLACED",
			expectedCount: 1,
			expectedOut:   "REPLACED is ready",
		},
		{
			name:          "after equals sign",
			input:         "config=stg-east-vnet ok",
			old:           "stg-east-vnet",
			repl:          "REPLACED",
			expectedCount: 1,
			expectedOut:   "config=REPLACED ok",
		},
		{
			name:          "at end of string",
			input:         "use stg-east-vnet",
			old:           "stg-east-vnet",
			repl:          "REPLACED",
			expectedCount: 1,
			expectedOut:   "use REPLACED",
		},
		{
			name:          "at start of string",
			input:         "stg-east-vnet starts",
			old:           "stg-east-vnet",
			repl:          "REPLACED",
			expectedCount: 1,
			expectedOut:   "REPLACED starts",
		},
		{
			name:          "embedded between letters on both sides",
			input:         "mystg-east-vnetHandler",
			old:           "stg-east-vnet",
			repl:          "REPLACED",
			expectedCount: 0,
			expectedOut:   "mystg-east-vnetHandler",
		},
		{
			name:          "left neighbor is letter",
			input:         "xstg-east-vnet ready",
			old:           "stg-east-vnet",
			repl:          "REPLACED",
			expectedCount: 0,
			expectedOut:   "xstg-east-vnet ready",
		},
		{
			name:          "right neighbor is letter",
			input:         "use stg-east-vnetz",
			old:           "stg-east-vnet",
			repl:          "REPLACED",
			expectedCount: 0,
			expectedOut:   "use stg-east-vnetz",
		},
		{
			name:          "multiple standalone occurrences",
			input:         "two stg-east-vnet refs stg-east-vnet",
			old:           "stg-east-vnet",
			repl:          "REPLACED",
			expectedCount: 2,
			expectedOut:   "two REPLACED refs REPLACED",
		},
		{
			name:          "hyphen is a boundary so prefix match replaces",
			input:         "service-foo is ready",
			old:           "service",
			repl:          "REPLACED",
			expectedCount: 1,
			expectedOut:   "REPLACED-foo is ready",
		},
		{
			name:          "digit neighbor does not block replacement",
			input:         "%22basic-hcp-cluster%22",
			old:           "basic-hcp-cluster",
			repl:          "REPLACED",
			expectedCount: 1,
			expectedOut:   "%22REPLACED%22",
		},
		{
			name:          "replacement contains the old string (no infinite loop)",
			input:         "find abc here",
			old:           "abc",
			repl:          "abc-obfuscated",
			expectedCount: 1,
			expectedOut:   "find abc-obfuscated here",
		},
		{
			name:          "old is entire string",
			input:         "abc",
			old:           "abc",
			repl:          "XYZ",
			expectedCount: 1,
			expectedOut:   "XYZ",
		},
		{
			name:          "consecutive matches with hyphen separator",
			input:         "abc-abc",
			old:           "abc",
			repl:          "XYZ",
			expectedCount: 2,
			expectedOut:   "XYZ-XYZ",
		},
		{
			name:          "empty old string returns original",
			input:         "some text here",
			old:           "",
			repl:          "REPLACED",
			expectedCount: 0,
			expectedOut:   "some text here",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			count, result := replaceStandalone(tc.input, tc.old, tc.repl, nil)
			assert.Equal(t, tc.expectedCount, count)
			assert.Equal(t, tc.expectedOut, result)
		})
	}
}

func TestCountStandalone(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    string
		old      string
		expected uint
	}{
		{name: "standalone with spaces", input: "stg-east-vnet is ready", old: "stg-east-vnet", expected: 1},
		{name: "at end of string", input: "use stg-east-vnet", old: "stg-east-vnet", expected: 1},
		{name: "at start of string", input: "stg-east-vnet starts", old: "stg-east-vnet", expected: 1},
		{name: "multiple occurrences", input: "stg-east-vnet and stg-east-vnet again", old: "stg-east-vnet", expected: 2},
		{name: "embedded between letters on both sides", input: "mystg-east-vnetHandler", old: "stg-east-vnet", expected: 0},
		{name: "left neighbor is letter", input: "xstg-east-vnet ready", old: "stg-east-vnet", expected: 0},
		{name: "right neighbor is letter", input: "use stg-east-vnetz", old: "stg-east-vnet", expected: 0},
		{name: "not present", input: "nothing here", old: "stg-east-vnet", expected: 0},
		{name: "empty old", input: "something", old: "", expected: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := countStandalone(tc.input, tc.old, nil)
			assert.Equal(t, tc.expected, got)
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
			name: "capitalized resource name bypasses isPlainWord and gets replaced globally",
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
				// count=2: once in the ARM path, once in free-text (mixed case bypasses isPlainWord)
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
		{
			name: "Azure region preserved in location subresource path",
			input: []string{
				`/subscriptions/aaaa-bbbb/resourceGroups/my-rg/providers/Microsoft.RedHatOpenShift/locations/uksouth/hcpOpenShiftVersions/4.19.18`,
				`cluster is in uksouth region`,
			},
			output: []string{
				`/subscriptions/subscription-generous-ostrich/resourcegroups/resourcegroup-touched-monkey/providers/Microsoft.RedHatOpenShift/locations/uksouth/hcpOpenShiftVersions/4.19.18`,
				`cluster is in uksouth region`,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "aaaa-bbbb", ReplacedWith: "subscription-generous-ostrich", Counter: map[string]uint{
					"aaaa-bbbb": 1,
				}},
				{Canonical: "my-rg", ReplacedWith: "resourcegroup-touched-monkey", Counter: map[string]uint{
					"my-rg": 1,
				}},
			}},
		},
		{
			name: "Azure regions preserved regardless of region name",
			input: []string{
				`/subscriptions/aaaa-bbbb/providers/Microsoft.RedHatOpenShift/locations/eastus/hcpOpenShiftVersions/4.20.1`,
				`/subscriptions/aaaa-bbbb/providers/Microsoft.RedHatOpenShift/locations/westeurope/hcpOpenShiftVersions/4.19.0`,
				`deployed in eastus and westeurope regions`,
			},
			output: []string{
				`/subscriptions/subscription-touched-monkey/providers/Microsoft.RedHatOpenShift/locations/eastus/hcpOpenShiftVersions/4.20.1`,
				`/subscriptions/subscription-touched-monkey/providers/Microsoft.RedHatOpenShift/locations/westeurope/hcpOpenShiftVersions/4.19.0`,
				`deployed in eastus and westeurope regions`,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "aaaa-bbbb", ReplacedWith: "subscription-touched-monkey", Counter: map[string]uint{
					"aaaa-bbbb": 2,
				}},
			}},
		},
		{
			name: "Azure region preserved in location-only path",
			input: []string{
				`/subscriptions/aaaa-bbbb/providers/Microsoft.RedHatOpenShift/locations/uksouth`,
			},
			output: []string{
				`/subscriptions/subscription-touched-monkey/providers/Microsoft.RedHatOpenShift/locations/uksouth`,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "aaaa-bbbb", ReplacedWith: "subscription-touched-monkey", Counter: map[string]uint{
					"aaaa-bbbb": 1,
				}},
			}},
		},
		{
			name: "common word resource name not replaced outside ARM path",
			input: []string{
				`/subscriptions/aaaa-bbbb/providers/Microsoft.ManagedIdentity/userAssignedIdentities/service`,
				`the service is running`,
			},
			output: []string{
				`/subscriptions/subscription-generous-ostrich/providers/Microsoft.ManagedIdentity/userAssignedIdentities/resource-touched-monkey`,
				`the service is running`,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "aaaa-bbbb", ReplacedWith: "subscription-generous-ostrich", Counter: map[string]uint{
					"aaaa-bbbb": 1,
				}},
				{Canonical: "service", ReplacedWith: "resource-touched-monkey", Counter: map[string]uint{
					"service": 1,
				}},
			}},
		},
		{
			name: "common word resource name does not corrupt hyphenated compounds",
			input: []string{
				`/subscriptions/aaaa-bbbb/providers/Microsoft.ManagedIdentity/userAssignedIdentities/service`,
				`service-foo is ready and clusters-service is healthy`,
			},
			output: []string{
				`/subscriptions/subscription-generous-ostrich/providers/Microsoft.ManagedIdentity/userAssignedIdentities/resource-touched-monkey`,
				`service-foo is ready and clusters-service is healthy`,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "aaaa-bbbb", ReplacedWith: "subscription-generous-ostrich", Counter: map[string]uint{
					"aaaa-bbbb": 1,
				}},
				{Canonical: "service", ReplacedWith: "resource-touched-monkey", Counter: map[string]uint{
					"service": 1,
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

func TestAzureResourceSeeding(t *testing.T) {
	t.Run("seeded canonical is replaced in free text", func(t *testing.T) {
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		seedable, ok := o.(SeedableObfuscator)
		require.True(t, ok, "obfuscator must implement SeedableObfuscator")

		seedable.SeedCanonical("dev-wx5r9ktl-svc")
		output := o.Contents(`cluster dev-wx5r9ktl-svc is running`)
		require.NotContains(t, output, "dev-wx5r9ktl-svc")
	})

	t.Run("cluster token discovers compounds during prescan", func(t *testing.T) {
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		seedable, ok := o.(SeedableObfuscator)
		require.True(t, ok)

		seedable.SetClusterToken("wx5r9ktl")

		// Prescan pass: Contents() discovers compounds containing the token
		o.Contents(`consumer_name='hcp-underlay-wx5r9ktl-mgmt-1' active`)
		o.Contents(`cluster dev-wx5r9ktl-svc is running`)

		// Main pass: discovered compounds are now replaced
		output1 := o.Contents(`checking hcp-underlay-wx5r9ktl-mgmt-1 status`)
		require.NotContains(t, output1, "hcp-underlay-wx5r9ktl-mgmt-1",
			"compound discovered via token should be replaced")

		output2 := o.Contents(`cluster dev-wx5r9ktl-svc is running`)
		require.NotContains(t, output2, "dev-wx5r9ktl-svc",
			"compound discovered via token should be replaced")
	})

	t.Run("concatenated prefix without hyphens is replaced", func(t *testing.T) {
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		seedable, ok := o.(SeedableObfuscator)
		require.True(t, ok)

		seedable.SeedCanonical("devwx5r9ktlsvc")
		output := o.Contents(`k8s:io.cilium.k8s.policy.cluster=devwx5r9ktlsvc,k8s:io.cilium.k8s.policy.serviceaccount=arobit`)
		require.NotContains(t, output, "devwx5r9ktlsvc",
			"concatenated cluster prefix should be replaced in Cilium policy labels")
	})

	t.Run("concatenated plain-word prefix is skipped", func(t *testing.T) {
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		seedable, ok := o.(SeedableObfuscator)
		require.True(t, ok)

		seedable.SeedCanonical("stguksouth")
		output := o.Contents(`cluster stguksouth is running`)
		require.Contains(t, output, "stguksouth",
			"plain-word concatenated prefix should be skipped by shouldSkipFreeTextReplacement")
	})

	t.Run("seeded canonical inside hyphenated compound is not replaced", func(t *testing.T) {
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		seedable, ok := o.(SeedableObfuscator)
		require.True(t, ok)

		// "myDisk123" passes shouldSkipFreeTextReplacement (mixed case+digit, length>=5).
		// When it appears as a prefix in a hyphenated compound it must NOT be replaced,
		// because replacing just the prefix corrupts the compound name.
		seedable.SeedCanonical("myDisk123")
		output := o.Contents("log shows myDisk123-handler is running")
		assert.Equal(t, "log shows myDisk123-handler is running", output,
			"canonical that is a strict subset of a compound should not be replaced")
	})

	t.Run("seeded canonical respects shouldSkipFreeTextReplacement", func(t *testing.T) {
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		seedable, ok := o.(SeedableObfuscator)
		require.True(t, ok)

		// "service" is a plain word and should be skipped
		seedable.SeedCanonical("service")
		output := o.Contents(`the service is running`)
		require.Contains(t, output, "service",
			"plain word canonical should be skipped even when seeded")
	})

	t.Run("pure-lowercase ARM-path resource name is replaced in free-text field on same line", func(t *testing.T) {
		// Reproduces the real-world "johndoe" scenario: a developer's personal cluster name
		// (all lowercase, e.g. "johndoe") appears both inside an ARM path and in a separate
		// JSON field ("resource_name") on the same log line.
		// Phase 1 replaces it inside the ARM path. Phase 2 must also replace the standalone
		// occurrence in the JSON field — the isPlainWord guard must NOT block ARM-path
		// discoveries, only seeded-only canonicals.
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		line := `{"resource_name":"johndoe","resource_id":"/subscriptions/da057d84-6570-41ea-83f7-f0f61a70542f/resourcegroups/johndoe-net-rg/providers/microsoft.redhatopenshift/hcpopenshiftclusters/johndoe"}`
		out := o.Contents(line)

		// ARM path must be replaced (Phase 1):
		require.NotContains(t, out, "/hcpopenshiftclusters/johndoe",
			"ARM path resource name must be obfuscated by Phase 1")
		// Free-text field must also be replaced (Phase 2) — currently failing:
		require.NotContains(t, out, `"resource_name":"johndoe"`,
			"pure-lowercase ARM-path resource name must be replaced in free-text JSON fields")
	})

	t.Run("genericSkipWords are still protected even when found in ARM paths", func(t *testing.T) {
		// "service" is in genericSkipWords. Even when found as an ARM resource name,
		// it must NOT be replaced in free-text lines like "the service is running".
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		// Trigger Phase 1 to discover "service" as a resource name
		o.Contents(`/subscriptions/da057d84-6570-41ea-83f7-f0f61a70542f/resourcegroups/myrg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/service`)
		// Free text must still contain "service" untouched
		out := o.Contents(`the service is running`)
		require.Contains(t, out, "service",
			"genericSkipWords must never be replaced in free text, even after Phase 1 discovery")
	})

	t.Run("clientId UUID field is obfuscated", func(t *testing.T) {
		// Azure API responses include "clientId" fields that contain MSI (Managed
		// Service Identity) client UUIDs. These are persistent identifiers tied to
		// a specific managed identity and must not appear in shared must-gather bundles.
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		line := `{"clientId": "c1d2e3f4-5a6b-7c8d-9e0f-1a2b3c4d5e6f", "name": "test"}`
		out := o.Contents(line)
		require.NotContains(t, out, "c1d2e3f4-5a6b-7c8d-9e0f-1a2b3c4d5e6f",
			"MSI clientId UUID must be obfuscated")
	})

	t.Run("principalId UUID field is obfuscated", func(t *testing.T) {
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		line := `{"principalId": "7f3b1a22-4c9d-4e8f-a1b2-d3e4f5061728"}`
		out := o.Contents(line)
		require.NotContains(t, out, "7f3b1a22-4c9d-4e8f-a1b2-d3e4f5061728",
			"MSI principalId UUID must be obfuscated")
	})

	t.Run("tenantId UUID field is obfuscated", func(t *testing.T) {
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		line := `{"tenantId": "a9b8c7d6-e5f4-3a2b-1c0d-9e8f7a6b5c4d"}`
		out := o.Contents(line)
		require.NotContains(t, out, "a9b8c7d6-e5f4-3a2b-1c0d-9e8f7a6b5c4d",
			"tenantId UUID must be obfuscated")
	})

	t.Run("clientId UUID obfuscated consistently across lines", func(t *testing.T) {
		// The same UUID appearing in multiple log lines must map to the same
		// replacement value so correlation across obfuscated lines is preserved.
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		uuid := "c1d2e3f4-5a6b-7c8d-9e0f-1a2b3c4d5e6f"
		line1 := `{"clientId": "` + uuid + `", "action": "create"}`
		line2 := `{"clientId": "` + uuid + `", "action": "update"}`
		out1 := o.Contents(line1)
		out2 := o.Contents(line2)

		require.NotContains(t, out1, uuid)
		require.NotContains(t, out2, uuid)

		// Extract the replacement value from line1 and verify line2 uses the same one.
		re := regexp.MustCompile(`"clientId":\s*"([^"]+)"`)
		m1 := re.FindStringSubmatch(out1)
		m2 := re.FindStringSubmatch(out2)
		require.NotNil(t, m1, "clientId field must be present in obfuscated output")
		require.NotNil(t, m2, "clientId field must be present in obfuscated output")
		require.Equal(t, m1[1], m2[1], "same UUID must map to the same replacement across calls")
	})

	t.Run("objectId UUID field is obfuscated", func(t *testing.T) {
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)
		line := `{"objectId": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"}`
		out := o.Contents(line)
		assert.NotContains(t, out, "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			"objectId UUID must be obfuscated")
	})

	t.Run("appId UUID field is obfuscated", func(t *testing.T) {
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)
		line := `{"appId": "12345678-abcd-ef01-2345-678901234567"}`
		out := o.Contents(line)
		assert.NotContains(t, out, "12345678-abcd-ef01-2345-678901234567",
			"appId UUID must be obfuscated")
	})

	t.Run("kubernetes.azure.com label UUIDs are obfuscated", func(t *testing.T) {
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)
		line := `kubernetes.azure.com/network-subscription=aabbccdd-1122-3344-5566-778899001122,kubernetes.azure.com/podnetwork-subscription=aabbccdd-1122-3344-5566-778899001122`
		out := o.Contents(line)
		assert.NotContains(t, out, "aabbccdd-1122-3344-5566-778899001122",
			"subscription UUID in k8s label must be obfuscated")
		assert.Contains(t, out, "kubernetes.azure.com/network-subscription=",
			"label key must be preserved")
		assert.Contains(t, out, "kubernetes.azure.com/podnetwork-subscription=",
			"label key must be preserved")
	})

	t.Run("kubernetes.azure.com client-id and vnetguid labels are obfuscated", func(t *testing.T) {
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)
		line := `kubernetes.azure.com/kubelet-identity-client-id=00112233-aabb-ccdd-eeff-445566778899,kubernetes.azure.com/nodenetwork-vnetguid=99887766-5544-3322-1100-ffeeddccbbaa`
		out := o.Contents(line)
		assert.NotContains(t, out, "00112233-aabb-ccdd-eeff-445566778899",
			"client-id UUID must be obfuscated")
		assert.NotContains(t, out, "99887766-5544-3322-1100-ffeeddccbbaa",
			"vnetguid UUID must be obfuscated")
	})

	t.Run("kubernetes.azure.com label UUIDs obfuscated consistently", func(t *testing.T) {
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)
		uuid := "aabbccdd-1122-3344-5566-778899001122"
		line1 := `kubernetes.azure.com/network-subscription=` + uuid
		line2 := `kubernetes.azure.com/podnetwork-subscription=` + uuid
		out1 := o.Contents(line1)
		out2 := o.Contents(line2)
		re := regexp.MustCompile(`kubernetes\.azure\.com/[\w-]+=(.+)`)
		m1 := re.FindStringSubmatch(out1)
		m2 := re.FindStringSubmatch(out2)
		require.NotNil(t, m1)
		require.NotNil(t, m2)
		require.Equal(t, m1[1], m2[1], "same UUID must map to same replacement")
	})

	t.Run("knownInfraComponents are still protected even when found in ARM paths", func(t *testing.T) {
		// "disk-csi-driver" is in knownInfraComponents. Even when discovered as an
		// ARM resource name (foundViaPhase1=true), it must NOT be replaced in free text
		// because it is also a standard OpenShift component name.
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		// Phase 1: discover "disk-csi-driver" as a resource name
		o.Contents(`/subscriptions/da057d84-6570-41ea-83f7-f0f61a70542f/resourcegroups/myrg/providers/Microsoft.Compute/disks/disk-csi-driver`)
		// Free text must still contain "disk-csi-driver" untouched
		out := o.Contents(`the disk-csi-driver pod is running`)
		require.Contains(t, out, "disk-csi-driver",
			"knownInfraComponents must not be replaced in free text, even after Phase 1 ARM-path discovery")
	})
}

func TestAzureResourceRegexBoundaries(t *testing.T) {
	t.Run("trailing colon causes sensitive cluster name to leak in free text", func(t *testing.T) {
		// Real pattern from staging must-gather (clusters-service-server.jsonl):
		// Azure 404 error puts a colon right after the cluster name in the ARM path.
		// With the old regex the canonical becomes "basic-cluster:" (with colon),
		// so bare "basic-cluster" elsewhere goes un-obfuscated — sensitive data leaks.
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		// Line 1: real error format — colon follows the cluster name directly
		o.Contents(`/subscriptions/99399281-aaaa-bbbb-cccc-b2645bbbdb93/resourceGroups/rg-cluster-nsg-subnet-reuse-vr9hbh/providers/Microsoft.RedHatOpenShift/HCPOpenShiftClusters/basic-cluster: The resource 'HCPOpenShiftClusters/basic-cluster' under resource group 'rg-cluster-nsg-subnet-reuse-vr9hbh' was not found.`)

		// Line 2: the same cluster name appears in URL-encoded free text — MUST be obfuscated
		output := o.Contents(`%22api.openshift.com%2Fname%22%3A%22basic-cluster%22`)
		require.NotContains(t, output, "basic-cluster",
			"sensitive cluster name leaked because trailing colon was captured as part of the canonical")
	})

	t.Run("trailing bracket causes sensitive deny assignment ID to leak in free text", func(t *testing.T) {
		// Real pattern from staging must-gather (clusters-service-server.jsonl):
		// Deny assignment creation log puts the ARM path with ] directly after
		// the subresource UUID. With the old regex the canonical becomes
		// "b32549d3-f2e7-5c12-9306-7d6265fa0e73]" (with bracket), so bare UUID
		// elsewhere goes un-obfuscated.
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		// Line 1: real log format — bracket follows the deny assignment UUID
		o.Contents(`/subscriptions/99399281-aaaa-bbbb-cccc-b2645bbbdb93/resourceGroups/disabled-image-registry-n7b4j2--managed/providers/Microsoft.Authorization/denyAssignments/b32549d3-f2e7-5c12-9306-7d6265fa0e73] created successfully for resource group`)

		// Line 2: the same UUID appears without bracket — MUST be obfuscated
		output := o.Contents(`checking deny assignment b32549d3-f2e7-5c12-9306-7d6265fa0e73 status`)
		require.NotContains(t, output, "b32549d3-f2e7-5c12-9306-7d6265fa0e73",
			"sensitive deny assignment ID leaked because trailing bracket was captured as part of the canonical")
	})

	t.Run("escaped newline causes vnet name plus error body to become the canonical", func(t *testing.T) {
		// Real pattern from staging must-gather (aro-hcp-backend.jsonl line 255413):
		// Azure 429 error response is JSON-escaped into a log line. The literal
		// characters \n appear after the vnet name. With the old regex the canonical
		// becomes "customer-vnet\n---\nRESPONSE 429..." instead of "customer-vnet",
		// so the real vnet name goes un-obfuscated — sensitive data leaks.
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		// Line 1: real log format — JSON-escaped \n follows the vnet name
		o.Contents(`/subscriptions/99399281-aaaa-bbbb-cccc-b2645bbbdb93/resourceGroups/oidc-wi-hfhmjt/providers/Microsoft.Network/virtualNetworks/customer-vnet\n--------------------------------------------------------------------------------\nRESPONSE 429: 429 Too Many Requests\nERROR CODE UNAVAILABLE\n--`)

		// The canonical in the report must be just "customer-vnet", not the
		// garbage string "customer-vnet\n---\nRESPONSE..."
		report := o.Report()
		for _, r := range report.Replacements {
			require.NotContains(t, r.Canonical, `\n`,
				"report canonical contains escaped newline garbage — regex captured past the resource name")
		}

		// Line 2: the same vnet name appears cleanly elsewhere — MUST be obfuscated
		output := o.Contents(`"vnetID":"/subscriptions/99399281-aaaa-bbbb-cccc-b2645bbbdb93/resourceGroups/oidc-wi-hfhmjt/providers/Microsoft.Network/virtualNetworks/customer-vnet/subnets/customer-subnet-1"`)
		require.NotContains(t, output, "customer-vnet",
			"sensitive vnet name leaked because escaped newline was captured as part of the canonical")
	})

	t.Run("colon after cluster name eats punctuation and breaks formatting", func(t *testing.T) {
		// Real pattern: when the colon is captured, the replacement removes
		// the colon from the output, turning "basic-cluster: The resource"
		// into "resource-pet-name The resource" — broken punctuation.
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		output := o.Contents(`/subscriptions/99399281-aaaa-bbbb-cccc-b2645bbbdb93/resourceGroups/rg-nsg-reuse-vr9hbh/providers/Microsoft.RedHatOpenShift/HCPOpenShiftClusters/basic-cluster: The resource was not found.`)

		require.Contains(t, output, ": The resource was not found.",
			"colon after cluster name was swallowed into the replacement")
	})

	t.Run("bracket after deny assignment ID in log preserved", func(t *testing.T) {
		// Real pattern: the bracket must be preserved as log formatting,
		// not captured as part of the deny assignment UUID.
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		output := o.Contents(`/subscriptions/99399281-aaaa-bbbb-cccc-b2645bbbdb93/resourceGroups/my-rg--managed/providers/Microsoft.Authorization/denyAssignments/b32549d3-f2e7-5c12-9306-7d6265fa0e73] created successfully`)

		require.Contains(t, output, "] created successfully",
			"trailing bracket was swallowed into the deny assignment ID capture")
		require.NotContains(t, output, "b32549d3-f2e7-5c12-9306-7d6265fa0e73",
			"deny assignment ID was not obfuscated")
	})

	t.Run("generated pet name is not re-obfuscated by a later regex pass", func(t *testing.T) {
		// When a resource group name is replaced with a pet name like
		// "resourcegroup-generous-ostrich", a later regex pass could match
		// that pet name as a new resource name. The generatedReplacements
		// guard prevents this double-obfuscation.
		o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
		require.NoError(t, err)

		// Process two ARM paths — the first generates replacements, the
		// second must not re-obfuscate those replacements.
		output1 := o.Contents(`/subscriptions/aaaa-bbbb/resourceGroups/my-rg/providers/Microsoft.Compute/disks/my-disk`)
		output2 := o.Contents(`/subscriptions/aaaa-bbbb/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet`)

		// Extract the subscription and RG replacements from the first output
		// and verify the second output reuses the same replacements
		// (not re-obfuscated into different pet names).
		require.NotContains(t, output1, "aaaa-bbbb")
		require.NotContains(t, output1, "my-rg")
		require.NotContains(t, output2, "aaaa-bbbb")
		require.NotContains(t, output2, "my-rg")

		// Both must produce the same subscription/RG prefix — extract and compare
		prefix1 := output1[:strings.Index(output1, "/providers/")]
		prefix2 := output2[:strings.Index(output2, "/providers/")]
		require.Equal(t, prefix1, prefix2,
			"subscription/RG replacements changed between calls — pet names may have been re-obfuscated")
	})
}

func TestAzureResourceObfuscator_Path(t *testing.T) {
	for _, tc := range []struct {
		name           string
		seedCanonicals []string
		armPaths       []string
		inputPath      string
		expectContains []string
		expectMissing  []string
	}{
		{
			name:           "service folder preserved",
			inputPath:      "service/foo.jsonl",
			expectContains: []string{"service/foo.jsonl"},
		},
		{
			name:           "cluster folder preserved",
			inputPath:      "cluster/events.json",
			expectContains: []string{"cluster/events.json"},
		},
		{
			name:           "system folder preserved",
			inputPath:      "system/logs.json",
			expectContains: []string{"system/logs.json"},
		},
		{
			name: "discovered resource name in path is obfuscated",
			armPaths: []string{
				"/subscriptions/aaaa-bbbb/resourceGroups/my-rg/providers/Microsoft.Compute/disks/my-cluster",
			},
			inputPath:     "service/my-cluster-events.jsonl",
			expectMissing: []string{"my-cluster"},
		},
		{
			name:           "seeded canonical in path is obfuscated",
			seedCanonicals: []string{"dev-wx5r9ktl-svc"},
			inputPath:      "service/dev-wx5r9ktl-svc-kube-system-coredns.jsonl",
			expectMissing:  []string{"dev-wx5r9ktl-svc"},
		},
		{
			name:           "seeded canonical preserves service folder name",
			seedCanonicals: []string{"dev-wx5r9ktl-svc"},
			inputPath:      "service/dev-wx5r9ktl-svc-clusters-service-clusters-service-server.jsonl",
			expectContains: []string{"service/"},
			expectMissing:  []string{"dev-wx5r9ktl-svc"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
			require.NoError(t, err)

			// Seed canonicals if specified
			if seedable, ok := o.(SeedableObfuscator); ok {
				for _, c := range tc.seedCanonicals {
					seedable.SeedCanonical(c)
				}
			}

			// Process ARM paths to discover canonicals
			for _, arm := range tc.armPaths {
				o.Contents(arm)
			}

			result := o.Path(tc.inputPath)

			for _, want := range tc.expectContains {
				require.Contains(t, result, want)
			}
			for _, notWant := range tc.expectMissing {
				require.NotContains(t, result, notWant)
			}
		})
	}
}
