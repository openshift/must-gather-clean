package obfuscator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewKeywordsObfuscator(t *testing.T) {
	for _, tc := range []struct {
		name           string
		replacements   map[string]string
		input          string
		expectedOutput string
		expectLegend   ReplacementReport
	}{
		{
			name: "basic",
			replacements: map[string]string{
				"unique-word": "replacement",
			},
			input:          "input with unique-word",
			expectedOutput: "input with replacement",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "unique-word", ReplacedWith: "replacement",
					Counter: map[string]uint{
						"unique-word": 1,
					}},
			}},
		},
		{
			name: "no replacement",
			replacements: map[string]string{
				"unique-word": "replacement",
			},
			input:          "input with common words",
			expectedOutput: "input with common words",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "unique-word", ReplacedWith: "replacement",
					Counter: map[string]uint{
						"unique-word": 0,
					}},
			}},
		},
		{
			name: "partial replacement",
			replacements: map[string]string{
				"first-unique":  "first-replacement",
				"second-unique": "second-replacement",
			},
			input:          "input with first-unique word",
			expectedOutput: "input with first-replacement word",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "first-unique", ReplacedWith: "first-replacement",
					Counter: map[string]uint{
						"first-unique": 1,
					}},
				{Canonical: "second-unique", ReplacedWith: "second-replacement",
					Counter: map[string]uint{
						"second-unique": 0,
					}},
			}},
		},
		{
			name: "partial replacement with repetition",
			replacements: map[string]string{
				"foo": "four",
				"bar": "zero",
			},
			input:          "input with foo foo foo times foo",
			expectedOutput: "input with four four four times four",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "foo", ReplacedWith: "four",
					Counter: map[string]uint{
						"foo": 4,
					}},
				{Canonical: "bar", ReplacedWith: "zero",
					Counter: map[string]uint{
						"bar": 0,
					}},
			}},
		},
		{
			name: "should not match substrings within words",
			replacements: map[string]string{
				"server": "redacted",
			},
			// "maestro-server" matches: hyphen is a word boundary
			// "grpc_server" does NOT match: underscore is a word character (\w), no boundary
			// "containerd.service" does NOT match: "server" is not present
			// "observability" does NOT match: no "server" substring
			input:          "maestro-server grpc_server containerd.service observability",
			expectedOutput: "maestro-redacted grpc_server containerd.service observability",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "server", ReplacedWith: "redacted",
					Counter: map[string]uint{
						"server": 1,
					}},
			}},
		},
		{
			name: "should not match keyword as substring of longer word",
			replacements: map[string]string{
				"dns": "redacted",
			},
			input:          "coredns is running on the dns server",
			expectedOutput: "coredns is running on the redacted server",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "dns", ReplacedWith: "redacted",
					Counter: map[string]uint{
						"dns": 1,
					}},
			}},
		},
		{
			name:           "dot is a word boundary: service matches in containerd.service",
			replacements:   map[string]string{"service": "REDACTED"},
			input:          `"systemd_unit":"containerd.service"`,
			expectedOutput: `"systemd_unit":"containerd.REDACTED"`,
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "service", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"service": 1,
					}},
			}},
		},
		{
			name:           "slash is a word boundary: network matches in /var/log/network.log",
			replacements:   map[string]string{"network": "REDACTED"},
			input:          "/var/log/network.log",
			expectedOutput: "/var/log/REDACTED.log",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "network", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"network": 1,
					}},
			}},
		},
		{
			name:           "hyphen is a word boundary: agent matches in node-agent-xyz",
			replacements:   map[string]string{"agent": "REDACTED"},
			input:          "node-agent-xyz",
			expectedOutput: "node-REDACTED-xyz",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "agent", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"agent": 1,
					}},
			}},
		},
		{
			name:           "underscore is NOT a word boundary: agent does not match in node_agent_xyz",
			replacements:   map[string]string{"agent": "REDACTED"},
			input:          "node_agent_xyz",
			expectedOutput: "node_agent_xyz",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "agent", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"agent": 0,
					}},
			}},
		},
		{
			name:           "camelCase: no boundary between lowercase and uppercase",
			replacements:   map[string]string{"server": "REDACTED"},
			input:          "myServer handles requests",
			expectedOutput: "myServer handles requests",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "server", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"server": 0,
					}},
			}},
		},
		{
			name:           "numeric suffix prevents match: node vs node01",
			replacements:   map[string]string{"node": "REDACTED"},
			input:          "node01 is healthy, node is not",
			expectedOutput: "node01 is healthy, REDACTED is not",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "node", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"node": 1,
					}},
			}},
		},
		{
			name:           "colon is a word boundary: proxy matches after colon",
			replacements:   map[string]string{"proxy": "REDACTED"},
			input:          `"component":"kube-proxy"`,
			expectedOutput: `"component":"kube-REDACTED"`,
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "proxy", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"proxy": 1,
					}},
			}},
		},
		{
			name:           "equals sign is a word boundary",
			replacements:   map[string]string{"admin": "REDACTED"},
			input:          "user=admin role=viewer",
			expectedOutput: "user=REDACTED role=viewer",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "admin", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"admin": 1,
					}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := NewKeywordsObfuscator(tc.replacements)
			require.Equal(t, tc.expectedOutput, o.Contents(tc.input))
			replacementReportsMatch(t, tc.expectLegend, o.Report())
		})
	}
}
