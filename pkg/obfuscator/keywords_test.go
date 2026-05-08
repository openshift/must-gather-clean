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
		usePath        bool
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
			name: "should not match inside protected compounds or non-boundary positions",
			replacements: map[string]string{
				"maestro": "redacted",
			},
			// "maestro-server" is protected (\w+-server), "grpc_maestro" has no \b boundary
			input:          "maestro-server grpc_maestro maestro is running",
			expectedOutput: "maestro-server grpc_maestro redacted is running",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "maestro", ReplacedWith: "redacted",
					Counter: map[string]uint{
						"maestro": 1,
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
			name:           "generic skip word: service is not replaced anywhere",
			replacements:   map[string]string{"service": "REDACTED"},
			input:          `"systemd_unit":"containerd.service" and service is running`,
			expectedOutput: `"systemd_unit":"containerd.service" and service is running`,
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "service", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"service": 0,
					}},
			}},
		},
		{
			name:           "generic skip word: network is not replaced",
			replacements:   map[string]string{"network": "REDACTED"},
			input:          "/var/log/network.log",
			expectedOutput: "/var/log/network.log",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "network", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"network": 0,
					}},
			}},
		},
		{
			name:           "generic skip word: agent is not replaced",
			replacements:   map[string]string{"agent": "REDACTED"},
			input:          "node-agent-xyz",
			expectedOutput: "node-agent-xyz",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "agent", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"agent": 0,
					}},
			}},
		},
		{
			name:           "underscore is NOT a word boundary: maestro does not match in node_maestro_xyz",
			replacements:   map[string]string{"maestro": "REDACTED"},
			input:          "node_maestro_xyz",
			expectedOutput: "node_maestro_xyz",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "maestro", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"maestro": 0,
					}},
			}},
		},
		{
			name:           "camelCase: no boundary between lowercase and uppercase",
			replacements:   map[string]string{"maestro": "REDACTED"},
			input:          "myMaestro handles requests",
			expectedOutput: "myMaestro handles requests",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "maestro", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"maestro": 0,
					}},
			}},
		},
		{
			name:           "numeric suffix prevents match",
			replacements:   map[string]string{"maestro": "REDACTED"},
			input:          "maestro01 is healthy, maestro is not",
			expectedOutput: "maestro01 is healthy, REDACTED is not",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "maestro", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"maestro": 1,
					}},
			}},
		},
		{
			name:           "generic skip word: proxy is not replaced",
			replacements:   map[string]string{"proxy": "REDACTED"},
			input:          `"component":"kube-proxy"`,
			expectedOutput: `"component":"kube-proxy"`,
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "proxy", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"proxy": 0,
					}},
			}},
		},
		{
			name:           "equals sign is a word boundary",
			replacements:   map[string]string{"stg-east-ch": "REDACTED"},
			input:          "cluster=stg-east-ch env=staging",
			expectedOutput: "cluster=REDACTED env=staging",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "stg-east-ch", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"stg-east-ch": 1,
					}},
			}},
		},
		{
			name:           "protected context: systemd unit not broken",
			replacements:   map[string]string{"crio": "REDACTED"},
			input:          `started crio.service successfully, crio is ready`,
			expectedOutput: `started crio.service successfully, REDACTED is ready`,
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "crio", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"crio": 1,
					}},
			}},
		},
		{
			name:           "protected context: multiple systemd units not broken",
			replacements:   map[string]string{"kubelet": "REDACTED"},
			input:          `kubelet.service started, kubelet is ready, NetworkManager.service active`,
			expectedOutput: `kubelet.service started, REDACTED is ready, NetworkManager.service active`,
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "kubelet", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"kubelet": 1,
					}},
			}},
		},
		{
			name:           "protected context: kube-compound not broken",
			replacements:   map[string]string{"apiserver": "REDACTED"},
			input:          `kube-apiserver is running, apiserver logs`,
			expectedOutput: `kube-apiserver is running, REDACTED logs`,
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "apiserver", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"apiserver": 1,
					}},
			}},
		},
		{
			name:           "Path: keyword is replaced in file paths",
			replacements:   map[string]string{"stg-east-ch": "REDACTED"},
			input:          "/logs/stg-east-ch/pods.log",
			expectedOutput: "/logs/REDACTED/pods.log",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "stg-east-ch", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"stg-east-ch": 1,
					}},
			}},
			usePath: true,
		},
		{
			name: "overlapping keywords: longest match wins",
			replacements: map[string]string{
				"east":        "R1",
				"stg-east-ch": "R2",
			},
			input:          "cluster stg-east-ch region east done",
			expectedOutput: "cluster R2 region R1 done",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "east", ReplacedWith: "R1",
					Counter: map[string]uint{
						"east": 1,
					}},
				{Canonical: "stg-east-ch", ReplacedWith: "R2",
					Counter: map[string]uint{
						"stg-east-ch": 1,
					}},
			}},
		},
		{
			name:           "keyword with dot: QuoteMeta prevents regex injection",
			replacements:   map[string]string{"v1.2": "REDACTED"},
			input:          "version v1.2 is old, v1x2 is not",
			expectedOutput: "version REDACTED is old, v1x2 is not",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "v1.2", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"v1.2": 1,
					}},
			}},
		},
		{
			name:           "compound keyword still matches",
			replacements:   map[string]string{"kube-proxy": "REDACTED"},
			input:          `component "kube-proxy" started`,
			expectedOutput: `component "REDACTED" started`,
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "kube-proxy", ReplacedWith: "REDACTED",
					Counter: map[string]uint{
						"kube-proxy": 1,
					}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := NewKeywordsObfuscator(tc.replacements)
			if tc.usePath {
				require.Equal(t, tc.expectedOutput, o.Path(tc.input))
			} else {
				require.Equal(t, tc.expectedOutput, o.Contents(tc.input))
			}
			replacementReportsMatch(t, tc.expectLegend, o.Report())
		})
	}
}
