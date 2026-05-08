package obfuscator

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractClusterToken(t *testing.T) {
	for _, tc := range []struct {
		name     string
		files    []string
		expected string
	}{
		// --- Personal/development environments (mixed-alphanumeric cluster IDs) ---
		{
			name: "dev env: wx5r9ktl extracted",
			files: []string{
				"dev-wx5r9ktl-svc-acrpull-acrpull-controller.jsonl",
				"dev-wx5r9ktl-svc-aks-istio-ingress-istio-proxy.jsonl",
				"dev-wx5r9ktl-svc-clusters-service-clusters-service-server.jsonl",
				"dev-wx5r9ktl-svc-kube-system-coredns.jsonl",
			},
			expected: "wx5r9ktl",
		},
		{
			name:     "single file: prefix includes namespace, no structural match",
			files:    []string{"dev-wx5r9ktl-svc-foo.jsonl"},
			expected: "",
		},
		{
			name: "prow env: j-prefixed BUILD_ID extracted",
			files: []string{
				"prow-j1234567-svc-kube-system-coredns.jsonl",
				"prow-j1234567-svc-acrpull-acrpull-controller.jsonl",
			},
			expected: "j1234567",
		},
		{
			name: "perf env: p-prefixed user suffix extracted",
			files: []string{
				"perf-zk8m2nvq-svc-kube-system-coredns.jsonl",
				"perf-zk8m2nvq-svc-acrpull-acrpull-controller.jsonl",
			},
			expected: "zk8m2nvq",
		},
		// --- Staging/production environments (pure-letter region names → no token) ---
		{
			name: "tst env: northeu is pure letters, no token",
			files: []string{
				"tst-northeu-svc-1-acrpull-acrpull-controller.jsonl",
				"tst-northeu-svc-1-kube-system-coredns.jsonl",
			},
			expected: "",
		},
		{
			name: "prod env: eastus is pure letters, no token",
			files: []string{
				"prod-eastus-svc-2-acrpull-acrpull-controller.jsonl",
				"prod-eastus-svc-2-kube-system-coredns.jsonl",
			},
			expected: "",
		},
		{
			name: "int env: northeu is pure letters, no token",
			files: []string{
				"int-northeu-svc-1-kube-system-coredns.jsonl",
				"int-northeu-svc-1-acrpull-acrpull-controller.jsonl",
			},
			expected: "",
		},
		// --- Management clusters ---
		{
			name: "mgmt cluster: pers env with stamp",
			files: []string{
				"dev-wx5r9ktl-mgmt-1-kube-system-coredns.jsonl",
				"dev-wx5r9ktl-mgmt-1-acrpull-acrpull-controller.jsonl",
			},
			expected: "wx5r9ktl",
		},
		{
			name: "mgmt cluster: stg env with stamp",
			files: []string{
				"tst-northeu-mgmt-1-kube-system-coredns.jsonl",
				"tst-northeu-mgmt-1-acrpull-acrpull-controller.jsonl",
			},
			expected: "",
		},
		// --- Edge cases ---
		{
			name:     "empty file list",
			files:    []string{},
			expected: "",
		},
		{
			name: "no common prefix",
			files: []string{
				"alpha-foo.jsonl",
				"bravo-bar.jsonl",
			},
			expected: "",
		},
		{
			name: "prefix too short for structure",
			files: []string{
				"ab-xy.jsonl",
				"ab-xz.jsonl",
			},
			expected: "",
		},
		{
			name: "multi-segment cluster ID extracted",
			files: []string{
				"env-seg1-seg2-svc-kube-system-coredns.jsonl",
				"env-seg1-seg2-svc-acrpull-controller.jsonl",
			},
			expected: "seg1-seg2",
		},
		{
			name: "no svc or mgmt type marker",
			files: []string{
				"env-clusterid-other-foo.jsonl",
				"env-clusterid-other-bar.jsonl",
			},
			expected: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractClusterToken(tc.files)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestExtractClusterInfo(t *testing.T) {
	t.Run("dev env returns prefix and token", func(t *testing.T) {
		files := []string{
			"dev-wx5r9ktl-svc-acrpull-acrpull-controller.jsonl",
			"dev-wx5r9ktl-svc-kube-system-coredns.jsonl",
		}
		prefix, token := ExtractClusterInfo(files)
		require.Equal(t, "dev-wx5r9ktl-svc", prefix)
		require.Equal(t, "wx5r9ktl", token)
	})

	t.Run("staging env returns prefix but no token", func(t *testing.T) {
		files := []string{
			"tst-northeu-svc-1-acrpull-acrpull-controller.jsonl",
			"tst-northeu-svc-1-kube-system-coredns.jsonl",
		}
		prefix, token := ExtractClusterInfo(files)
		require.Equal(t, "tst-northeu-svc-1", prefix)
		require.Equal(t, "", token)
	})

	t.Run("prow env returns prefix and token", func(t *testing.T) {
		files := []string{
			"prow-j1234567-svc-kube-system-coredns.jsonl",
			"prow-j1234567-svc-acrpull-acrpull-controller.jsonl",
		}
		prefix, token := ExtractClusterInfo(files)
		require.Equal(t, "prow-j1234567-svc", prefix)
		require.Equal(t, "j1234567", token)
	})

	t.Run("mgmt cluster with stamp returns prefix and token", func(t *testing.T) {
		files := []string{
			"dev-wx5r9ktl-mgmt-1-kube-system-coredns.jsonl",
			"dev-wx5r9ktl-mgmt-1-acrpull-acrpull-controller.jsonl",
		}
		prefix, token := ExtractClusterInfo(files)
		require.Equal(t, "dev-wx5r9ktl-mgmt-1", prefix)
		require.Equal(t, "wx5r9ktl", token)
	})
}

func TestParseClusterPrefix(t *testing.T) {
	for _, tc := range []struct {
		name        string
		prefix      string
		expectedEnv string
		expectedID  string
		expectedTyp string
	}{
		{"dev svc", "dev-wx5r9ktl-svc", "dev", "wx5r9ktl", "svc"},
		{"tst svc with stamp", "tst-northeu-svc-1", "tst", "northeu", "svc"},
		{"prod svc with stamp", "prod-eastus-svc-2", "prod", "eastus", "svc"},
		{"dev mgmt with stamp", "dev-wx5r9ktl-mgmt-1", "dev", "wx5r9ktl", "mgmt"},
		{"prow svc", "prow-j1234567-svc", "prow", "j1234567", "svc"},
		{"int svc with stamp", "int-northeu-svc-1", "int", "northeu", "svc"},
		{"too short", "ab-xy", "", "", ""},
		{"no type marker", "env-cluster-other", "", "", ""},
		{"only two segments", "env-svc", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, id, typ := parseClusterPrefix(tc.prefix)
			require.Equal(t, tc.expectedEnv, env)
			require.Equal(t, tc.expectedID, id)
			require.Equal(t, tc.expectedTyp, typ)
		})
	}
}

func TestDiscoverSensitiveCompounds(t *testing.T) {
	for _, tc := range []struct {
		name     string
		text     string
		token    string
		expected []string
	}{
		{
			name:     "consumer_name with single quotes",
			text:     `consumer_name='hcp-underlay-wx5r9ktl-mgmt-1' and status=active`,
			token:    "wx5r9ktl",
			expected: []string{"hcp-underlay-wx5r9ktl-mgmt-1"},
		},
		{
			name:     "URL-encoded quotes",
			text:     `%27dev-wx5r9ktl-svc%27`,
			token:    "wx5r9ktl",
			expected: []string{"dev-wx5r9ktl-svc"},
		},
		{
			name:     "monitoring hostname with dots",
			text:     `msprom-westus3-dev-wx5r9ktl-svc-0vyy.westus3-1.metrics.ingest.monitor.azure.com`,
			token:    "wx5r9ktl",
			expected: []string{"msprom-westus3-dev-wx5r9ktl-svc-0vyy"},
		},
		{
			name:     "no false positive without hyphen boundary",
			text:     `xwx5r9ktly is not a match`,
			token:    "wx5r9ktl",
			expected: nil,
		},
		{
			name:     "double-quoted JSON value",
			text:     `"clusterName":"dev-wx5r9ktl-svc"`,
			token:    "wx5r9ktl",
			expected: []string{"dev-wx5r9ktl-svc"},
		},
		{
			name:     "multiple compounds in one line",
			text:     `source='arohcpdev-wx5r9ktl' consumer='hcp-underlay-wx5r9ktl-mgmt-1'`,
			token:    "wx5r9ktl",
			expected: []string{"arohcpdev-wx5r9ktl", "hcp-underlay-wx5r9ktl-mgmt-1"},
		},
		{
			name:     "token at start of compound",
			text:     `wx5r9ktl-svc is running`,
			token:    "wx5r9ktl",
			expected: []string{"wx5r9ktl-svc"},
		},
		{
			name:     "token at end of compound",
			text:     `cluster dev-wx5r9ktl ready`,
			token:    "wx5r9ktl",
			expected: []string{"dev-wx5r9ktl"},
		},
		{
			name:     "standalone token is also a compound",
			text:     `token wx5r9ktl alone`,
			token:    "wx5r9ktl",
			expected: []string{"wx5r9ktl"},
		},
		{
			name:     "token not present",
			text:     `nothing sensitive here`,
			token:    "wx5r9ktl",
			expected: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			segPat := regexp.MustCompile(`(?:^|-)` + regexp.QuoteMeta(tc.token) + `(?:-|$)`)
			result := DiscoverSensitiveCompounds(tc.text, tc.token, segPat)
			if tc.expected == nil {
				require.Empty(t, result)
			} else {
				require.ElementsMatch(t, tc.expected, result)
			}
		})
	}
}
