package obfuscator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/must-gather-clean/pkg/schema"
)

func TestIPObfuscatorStatic(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  string
		output string
		report map[string]string
	}{
		{
			name:   "valid ipv4 address",
			input:  "received request from 192.168.1.10",
			output: "received request from xxx.xxx.xxx.xxx",
			report: map[string]string{"192.168.1.10": obfuscatedStaticIPv4},
		},
		{
			name:   "max ipv4 address",
			input:  "received request from 255.255.255.255",
			output: "received request from xxx.xxx.xxx.xxx",
			report: map[string]string{"255.255.255.255": obfuscatedStaticIPv4},
		},
		{
			name:   "ipv4 in aws etcd pathing",
			input:  "must-gather/etcd-ip-10-0-187-218.ec2.internal/some.yaml",
			output: "must-gather/etcd-ip-xxx.xxx.xxx.xxx.ec2.internal/some.yaml",
			report: map[string]string{"10-0-187-218": obfuscatedStaticIPv4},
		},
		{
			name:   "invalid ipv4 address does not match submatch",
			input:  "value 910.218.98.1 is not an ipv4",
			output: "value 910.218.98.1 is not an ipv4",
			report: map[string]string{},
		},
		{
			name:   "invalid ipv4 address II",
			input:  "value 333.125.22.640 is not an ipv4",
			output: "value 333.125.22.640 is not an ipv4",
			report: map[string]string{},
		},
		{
			name:   "ipv4 in words",
			input:  "calling https://192.168.1.20/metrics for values",
			output: "calling https://xxx.xxx.xxx.xxx/metrics for values",
			report: map[string]string{"192.168.1.20": obfuscatedStaticIPv4},
		},
		{
			name:   "multiple ipv4s",
			input:  "received request from 192.168.1.20 proxied through 192.168.1.3",
			output: "received request from xxx.xxx.xxx.xxx proxied through xxx.xxx.xxx.xxx",
			report: map[string]string{
				"192.168.1.20": obfuscatedStaticIPv4,
				"192.168.1.3":  obfuscatedStaticIPv4,
			},
		},
		{
			name:   "valid ipv6 address",
			input:  "received request from 2001:db8::ff00:42:8329",
			output: "received request from xxxx:xxxx:xxxx:xxxx:xxxx:xxxx:xxxx:xxxx",
			report: map[string]string{
				"2001:db8::ff00:42:8329": obfuscatedStaticIPv6,
			},
		},
		{
			name:   "mixed ipv4 and ipv6",
			input:  "tunneling ::2fa:bf9 as 192.168.1.30",
			output: "tunneling xxxx:xxxx:xxxx:xxxx:xxxx:xxxx:xxxx:xxxx as xxx.xxx.xxx.xxx",
			report: map[string]string{
				"192.168.1.30": obfuscatedStaticIPv4,
				"::2fa:bf9":    obfuscatedStaticIPv6,
			},
		},
		{
			// This testcase reports both the detected IP address and the Normalized/Cleaned IP address
			name:   "non standard ipv4",
			input:  "ip-10-0-129-220.ec2.aws.yaml",
			output: "ip-xxx.xxx.xxx.xxx.ec2.aws.yaml",
			report: map[string]string{
				"10-0-129-220": obfuscatedStaticIPv4,
			},
		},
		{
			name:   "non standard ipv4 underscore",
			input:  "NODE_ip_10_0_195_177_ec2_internal_IP",
			output: "NODE_ip_xxx.xxx.xxx.xxx_ec2_internal_IP",
			report: map[string]string{
				"10_0_195_177": obfuscatedStaticIPv4,
			},
		},
		{
			name:   "non-standard ipv4 with bad separator",
			input:  "ip+10+0+129+220.ec2.aws.yaml",
			output: "ip+10+0+129+220.ec2.aws.yaml",
			report: map[string]string{},
		},
		{
			name:   "standard ipv4 and standard ipv4",
			input:  "obfuscate 10.0.129.220 and 10-0-129-220",
			output: "obfuscate xxx.xxx.xxx.xxx and xxx.xxx.xxx.xxx",
			report: map[string]string{
				"10.0.129.220": obfuscatedStaticIPv4,
				"10-0-129-220": obfuscatedStaticIPv4,
			},
		},
		{
			name:   "OCP nightly version false positive",
			input:  "version: 4.8.0-0.nightly-2021-07-31-065602",
			output: "version: 4.8.0-0.nightly-2021-07-31-065602",
			report: map[string]string{},
		},
		{
			name:   "OCP version x.y.z",
			input:  "version: 4.8.12",
			output: "version: 4.8.12",
			report: map[string]string{},
		},
		{
			name:   "excluded ipv4 address",
			input:  "Listening on 0.0.0.0:8080",
			output: "Listening on 0.0.0.0:8080",
			report: map[string]string{},
		},
		{
			name:   "excluded ipv6 address",
			input:  "Listening on [::1]:8080",
			output: "Listening on [::1]:8080",
			report: map[string]string{},
		},
		{
			name:   "excluded ipv6 address in full form",
			input:  "Listening on [0:0:0:0:0:0:0:1]:8080",
			output: "Listening on [0:0:0:0:0:0:0:1]:8080",
			report: map[string]string{},
		},
		{
			name:   "excluded ipv4-mapped ipv6 address",
			input:  "Listening on [::ffff:127.0.0.1]:8080",
			output: "Listening on [::ffff:127.0.0.1]:8080",
			report: map[string]string{},
		},
		{
			name:   "inside url encoded yaml",
			input:  "data:,kind%3A%20KubeletConfiguration%0AapiVersion%3A%20kubelet.config.k8s.io%2Fv1beta1%0Aauthentication%3A%0A%20%20x509%3A%0A%20%20%20%20clientCAFile%3A%20%2Fetc%2Fkubernetes%2Fkubelet-ca.crt%0A%20%20anonymous%3A%0A%20%20%20%20enabled%3A%20false%0AcgroupDriver%3A%20systemd%0AcgroupRoot%3A%20%2F%0AclusterDNS%3A%0A%20%20-%20172.30.0.10%0AclusterDomain%3A%20cluster.local%0AcontainerLogMaxSize%3A%2050Mi%0AmaxPods%3A%20250%0AkubeAPIQPS%3A%2050%0AkubeAPIBurst%3A%20100%0ArotateCertificates%3A%20true%0AserializeImagePulls%3A%20false%0AstaticPodPath%3A%20%2Fetc%2Fkubernetes%2Fmanifests%0AsystemCgroups%3A%20%2Fsystem.slice%0AsystemReserved%3A%0A%20%20ephemeral-storage%3A%201Gi%0AfeatureGates%3A%0A%20%20APIPriorityAndFairness%3A%20true%0A%20%20LegacyNodeRoleBehavior%3A%20false%0A%20%20NodeDisruptionExclusion%3A%20true%0A%20%20RotateKubeletServerCertificate%3A%20true%0A%20%20ServiceNodeExclusion%3A%20true%0A%20%20SupportPodPidsLimit%3A%20true%0A%20%20DownwardAPIHugePages%3A%20true%0AserverTLSBootstrap%3A%20true%0AtlsMinVersion%3A%20VersionTLS12%0AtlsCipherSuites%3A%0A%20%20-%20TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256%0A%20%20-%20TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256%0A%20%20-%20TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384%0A%20%20-%20TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384%0A%20%20-%20TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256%0A%20%20-%20TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256%0A",
			output: "data:,kind%3A%20KubeletConfiguration%0AapiVersion%3A%20kubelet.config.k8s.io%2Fv1beta1%0Aauthentication%3A%0A%20%20x509%3A%0A%20%20%20%20clientCAFile%3A%20%2Fetc%2Fkubernetes%2Fkubelet-ca.crt%0A%20%20anonymous%3A%0A%20%20%20%20enabled%3A%20false%0AcgroupDriver%3A%20systemd%0AcgroupRoot%3A%20%2F%0AclusterDNS%3A%0A%20%20-%20xxx.xxx.xxx.xxx%0AclusterDomain%3A%20cluster.local%0AcontainerLogMaxSize%3A%2050Mi%0AmaxPods%3A%20250%0AkubeAPIQPS%3A%2050%0AkubeAPIBurst%3A%20100%0ArotateCertificates%3A%20true%0AserializeImagePulls%3A%20false%0AstaticPodPath%3A%20%2Fetc%2Fkubernetes%2Fmanifests%0AsystemCgroups%3A%20%2Fsystem.slice%0AsystemReserved%3A%0A%20%20ephemeral-storage%3A%201Gi%0AfeatureGates%3A%0A%20%20APIPriorityAndFairness%3A%20true%0A%20%20LegacyNodeRoleBehavior%3A%20false%0A%20%20NodeDisruptionExclusion%3A%20true%0A%20%20RotateKubeletServerCertificate%3A%20true%0A%20%20ServiceNodeExclusion%3A%20true%0A%20%20SupportPodPidsLimit%3A%20true%0A%20%20DownwardAPIHugePages%3A%20true%0AserverTLSBootstrap%3A%20true%0AtlsMinVersion%3A%20VersionTLS12%0AtlsCipherSuites%3A%0A%20%20-%20TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256%0A%20%20-%20TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256%0A%20%20-%20TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384%0A%20%20-%20TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384%0A%20%20-%20TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256%0A%20%20-%20TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256%0A",
			report: map[string]string{
				"172.30.0.10": obfuscatedStaticIPv4,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewIPObfuscator(schema.ObfuscateReplacementTypeStatic, NewSimpleTracker())
			require.NoError(t, err)
			output := o.Contents(tc.input)
			assert.Equal(t, tc.output, output)
			assert.Equal(t, tc.report, o.Report().AsMap())
		})
	}
}

func TestIPObfuscatorReplacesPercentEncodedAddress(t *testing.T) {
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	for _, tc := range []struct {
		input  string
		output string
	}{
		{
			input:  "ip=%31%39%32%2E%30%2E%32%2E%31",
			output: "ip=x-ipv4-0000000001-x",
		},
		{
			input:  "ip=%31%39%32%5F%30%5F%32%5F%31",
			output: "ip=x-ipv4-0000000001-x",
		},
		{
			input:  "ip=%31%39%32%2D%30%2D%32%2D%31",
			output: "ip=x-ipv4-0000000001-x",
		},
		{
			input:  "host=fe80%3A%3A1",
			output: "host=x-ipv6-0000000001-x",
		},
		{
			input:  "value%20%31%39%32%2E%30%2E%32%2E%31",
			output: "value%20x-ipv4-0000000001-x",
		},
		{
			input:  "192%2E0%2E2%2E1",
			output: "x-ipv4-0000000001-x",
		},
	} {
		assert.Equal(t, tc.output, obfuscator.Contents(tc.input))
	}
	assert.Equal(t, "ip=127%2E0%2E0%2E1", obfuscator.Contents("ip=127%2E0%2E0%2E1"))
	assert.Equal(t, map[string]string{
		"%31%39%32%2E%30%2E%32%2E%31": "x-ipv4-0000000001-x",
		"%31%39%32%5F%30%5F%32%5F%31": "x-ipv4-0000000001-x",
		"%31%39%32%2D%30%2D%32%2D%31": "x-ipv4-0000000001-x",
		"fe80%3A%3A1":                 "x-ipv6-0000000001-x",
		"192%2E0%2E2%2E1":             "x-ipv4-0000000001-x",
	}, obfuscator.Report().AsMap())
}

func TestIPObfuscatorRawAndEncodedFormsShareReplacement(t *testing.T) {
	for _, tc := range []struct {
		name      string
		canonical string
		forms     []string
		ipv6      bool
	}{
		{
			name:      "IPv4 dotted, alternate, and encoded",
			canonical: "192.0.2.1",
			forms: []string{
				"192.0.2.1",
				"192_0_2_1",
				"192%2E0%2E2%2E1",
				"%31%39%32%2E%30%2E%32%2E%31",
			},
		},
		{
			name:      "IPv6 raw and encoded",
			canonical: "FE80::1",
			forms: []string{
				"fe80::1",
				"fe80%3A%3A1",
			},
			ipv6: true,
		},
		{
			name:      "IPv6 with embedded IPv4 raw and encoded",
			canonical: "::FFFF:192.0.2.1",
			forms: []string{
				"::ffff:192.0.2.1",
				"%3A%3Affff%3A192%2E0%2E2%2E1",
			},
			ipv6: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			output := obfuscator.Contents(strings.Join(tc.forms, " | "))
			reports := obfuscator.Report().Replacements
			require.Len(t, reports, 1)
			assert.Equal(t, tc.canonical, reports[0].Canonical)
			assert.Equal(t, len(tc.forms), strings.Count(output, reports[0].ReplacedWith))
			for _, form := range tc.forms {
				assert.Equal(t, uint(1), reports[0].Counter[form])
			}
			if tc.ipv6 {
				assert.Contains(t, reports[0].ReplacedWith, "ipv6")
			} else {
				assert.Contains(t, reports[0].ReplacedWith, "ipv4")
			}
		})
	}
}

func TestIPObfuscatorKeepsDistinctIPv6TextualFormsSeparate(t *testing.T) {
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	output := obfuscator.Contents("full=FE80:0000:0000:0000:0000:0000:0000:0001 compressed=fe80::1")
	reports := obfuscator.Report().Replacements

	require.Len(t, reports, 2)
	assert.Equal(t, "FE80:0000:0000:0000:0000:0000:0000:0001", reports[0].Canonical)
	assert.Equal(t, "FE80::1", reports[1].Canonical)
	assert.NotEqual(t, reports[0].ReplacedWith, reports[1].ReplacedWith)
	assert.Equal(t, 2, strings.Count(output, "x-ipv6-"))
}

func TestIPObfuscatorExcludesEncodedEquivalentIPs(t *testing.T) {
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	for _, input := range []string{
		"ip=%31%32%37%2E0%2E0%2E1",
		"ip=%3A%3Affff%3A127%2E0%2E0%2E1",
	} {
		t.Run(input, func(t *testing.T) {
			assert.Equal(t, input, obfuscator.Contents(input))
		})
	}
	assert.Empty(t, obfuscator.Report().Replacements)
}

func TestIPObfuscatorHandlesIPv4WithZeroFirstOctet(t *testing.T) {
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	assert.Equal(t, "x-ipv4-0000000001-x", obfuscator.Contents("0.0.0.1"))
	assert.Equal(t, "x-ipv4-0000000002-x", obfuscator.Contents("0.1.2.3"))
	assert.Equal(t, "0.0.0.0", obfuscator.Contents("0.0.0.0"))
	assert.Equal(t, "x-ipv4-0000000001-x", obfuscator.Contents("%30%2E0%2E0%2E1"))
	assert.Equal(t, map[string]string{
		"0.0.0.1":         "x-ipv4-0000000001-x",
		"0.1.2.3":         "x-ipv4-0000000002-x",
		"%30%2E0%2E0%2E1": "x-ipv4-0000000001-x",
	}, obfuscator.Report().AsMap())
}

func TestIPObfuscatorHandlesEncodedAddressFollowedByEncodedDelimiter(t *testing.T) {
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	for _, tc := range []struct {
		input  string
		output string
	}{
		{
			input:  "%31%39%32%2E%30%2E%32%2E%31%2Fpath",
			output: "x-ipv4-0000000001-x%2Fpath",
		},
		{
			input:  "ip=%31%39%32%2E%30%2E%32%2E%31%0A",
			output: "ip=x-ipv4-0000000001-x%0A",
		},
		{
			input:  "%66%64%36%39%3A%3A%35%2Fpath",
			output: "x-ipv6-0000000001-x%2Fpath",
		},
		{
			input:  "http%3A%2F%2F%31%39%32%2E%30%2E%32%2E%31%2Fmetrics",
			output: "http%3A%2F%2Fx-ipv4-0000000001-x%2Fmetrics",
		},
	} {
		assert.Equal(t, tc.output, obfuscator.Contents(tc.input))
	}
	assert.Equal(t, map[string]string{
		"%31%39%32%2E%30%2E%32%2E%31": "x-ipv4-0000000001-x",
		"%66%64%36%39%3A%3A%35":       "x-ipv6-0000000001-x",
	}, obfuscator.Report().AsMap())
}

func TestIPObfuscatorReplacesIPv6PrefixBeforeTrailingLabel(t *testing.T) {
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	input := "%66%64%36%39%3A%3A%35%73"
	assert.Equal(t, "value=x-ipv6-0000000001-xs", obfuscator.Contents("value=fd69::5s"))
	assert.Equal(t, "x-ipv6-0000000001-x%73", obfuscator.Contents(input))
	assert.Equal(t, map[string]string{
		"fd69::5":               "x-ipv6-0000000001-x",
		"%66%64%36%39%3A%3A%35": "x-ipv6-0000000001-x",
	}, obfuscator.Report().AsMap())
}

func TestIPObfuscatorHandlesIPv6WithEmbeddedIPv4(t *testing.T) {
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	for _, tc := range []struct {
		input  string
		output string
	}{
		{input: "64:ff9b::192.0.2.1", output: "x-ipv6-0000000001-x"},
		{input: "2001:db8::192.0.2.1", output: "x-ipv6-0000000002-x"},
		{input: "::ffff:192.0.2.1", output: "x-ipv6-0000000003-x"},
		{input: "0:0:0:0:0:ffff:192.0.2.1", output: "x-ipv6-0000000004-x"},
		{input: "%36%34%3A%66%66%39%62%3A%3A%31%39%32%2E%30%2E%32%2E%31", output: "x-ipv6-0000000001-x"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.output, obfuscator.Contents(tc.input))
		})
	}
	assert.Equal(t, map[string]string{
		"64:ff9b::192.0.2.1":       "x-ipv6-0000000001-x",
		"2001:db8::192.0.2.1":      "x-ipv6-0000000002-x",
		"::ffff:192.0.2.1":         "x-ipv6-0000000003-x",
		"0:0:0:0:0:ffff:192.0.2.1": "x-ipv6-0000000004-x",
		"%36%34%3A%66%66%39%62%3A%3A%31%39%32%2E%30%2E%32%2E%31": "x-ipv6-0000000001-x",
	}, obfuscator.Report().AsMap())
}

func TestIPObfuscatorHandlesAlternateSeparatorIPv4InIPv6(t *testing.T) {
	for _, input := range []string{
		"fd00:abcd:1234::192_0_2_1",
		"fd00:abcd:1234::192-0-2-1",
		"fd00:abcd:1234::192%5F0%5F2%5F1",
		"fd00:abcd:1234::192%2D0%2D2%2D1",
		"::ffff:192_0_2_1",
		"::ffff:192-0-2-1",
		"64:ff9b::192_0_2_1",
		"64:ff9b::192-0-2-1",
	} {
		t.Run(input, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			assert.Equal(t, "x-ipv6-0000000001-x", obfuscator.Contents(input))
			assert.Equal(t, map[string]string{input: "x-ipv6-0000000001-x"}, obfuscator.Report().AsMap())
		})
	}
}

func TestIPObfuscatorHandlesIPv6WithZeroPrefixedIPv4Tail(t *testing.T) {
	for _, tc := range []struct {
		input     string
		canonical string
	}{
		{input: "::0.1.2.3"},
		{input: "1:2:3:4:5:6:0.1.2.3"},
		{input: "1:2:3:4:5:6:0.0.0.0"},
		{input: "::ffff:0.0.0.1"},
		{input: "64:ff9b::0.1.2.3"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			output := obfuscator.Contents(tc.input)
			assert.Equal(t, "x-ipv6-0000000001-x", output)
			assert.Equal(t, map[string]string{tc.input: output}, obfuscator.Report().AsMap())
		})
	}

	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)
	assert.Equal(t, "::0.0.0.1", obfuscator.Contents("::0.0.0.1"))
	assert.Empty(t, obfuscator.Report().Replacements)
}

func TestIPObfuscatorMasksIPv4TailOfInvalidIPv6(t *testing.T) {
	for _, tc := range []struct {
		input   string
		encoded string
		output  string
		report  map[string]string
	}{
		{
			input:   "1:2:3:4:5:6:7:192.0.2.1",
			encoded: "1%3A2%3A3%3A4%3A5%3A6%3A7%3A192%2E0%2E2%2E1",
			output:  "1:2:3:4:5:6:7:x-ipv4-0000000001-x",
			report:  map[string]string{"192.0.2.1": "x-ipv4-0000000001-x"},
		},
		{
			input:   "x:1:2:3:4:5:6:192.0.2.1",
			encoded: "x%3A1%3A2%3A3%3A4%3A5%3A6%3A192%2E0%2E2%2E1",
			output:  "x:1:2:3:4:5:6:x-ipv4-0000000001-x",
			report:  map[string]string{"192.0.2.1": "x-ipv4-0000000001-x"},
		},
	} {
		t.Run(tc.input, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			assert.Equal(t, tc.output, obfuscator.Contents(tc.input))
			assert.Equal(t, tc.report, obfuscator.Report().AsMap())
			encodedOutput := obfuscator.Contents(tc.encoded)
			assert.NotContains(t, encodedOutput, "192%2E0%2E2%2E1")
			assert.Contains(t, encodedOutput, "x-ipv4-0000000001-x")
			assert.Equal(t, map[string]string{
				"192.0.2.1":       "x-ipv4-0000000001-x",
				"192%2E0%2E2%2E1": "x-ipv4-0000000001-x",
			}, obfuscator.Report().AsMap())
		})
	}
}

func TestIPObfuscatorConsistentOrderFollowsTextualOrder(t *testing.T) {
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	input := "raw=192.0.2.1 encoded=%31%39%38%2E%35%31%2E%31%30%30%2E%37"
	expected := "raw=x-ipv4-0000000001-x encoded=x-ipv4-0000000002-x"
	assert.Equal(t, expected, obfuscator.Contents(input))
	assert.Equal(t, map[string]string{
		"192.0.2.1":                            "x-ipv4-0000000001-x",
		"%31%39%38%2E%35%31%2E%31%30%30%2E%37": "x-ipv4-0000000002-x",
	}, obfuscator.Report().AsMap())
}

func TestIPObfuscatorUsesDecodedIPv6Context(t *testing.T) {
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	input := "value%202001:db8::1"
	assert.Equal(t, "value%20x-ipv6-0000000001-x", obfuscator.Contents(input))
	assert.Equal(t, map[string]string{
		"2001:db8::1": "x-ipv6-0000000001-x",
	}, obfuscator.Report().AsMap())
}

func TestIPObfuscatorPreservesCompletePercentEncoding(t *testing.T) {
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	for _, tc := range []struct {
		input  string
		output string
	}{
		{input: "%251.168.0.1", output: "%25x-ipv4-0000000001-x"},
		{input: "%25::abcd", output: "%25x-ipv6-0000000001-x"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.output, obfuscator.Contents(tc.input))
		})
	}
	assert.Equal(t, map[string]string{
		"1.168.0.1": "x-ipv4-0000000001-x",
		"::abcd":    "x-ipv6-0000000001-x",
	}, obfuscator.Report().AsMap())
}

func TestIPObfuscatorHandlesUppercaseIPv6(t *testing.T) {
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	assert.Equal(t, "x-ipv6-0000000001-x", obfuscator.Contents("FE80::1"))
	assert.Equal(t, "x-ipv6-0000000002-x", obfuscator.Contents("FE80:0:0:0:0:0:0:1"))
	assert.Equal(t, map[string]string{
		"FE80::1":            "x-ipv6-0000000001-x",
		"FE80:0:0:0:0:0:0:1": "x-ipv6-0000000002-x",
	}, obfuscator.Report().AsMap())
}

func TestIPObfuscatorPreservesLeadingLabelColon(t *testing.T) {
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	assert.Equal(t, "IP:x-ipv6-0000000001-x", obfuscator.Contents("IP:fd69::5"))
	assert.Equal(t, "host%3Ax-ipv6-0000000002-x", obfuscator.Contents("host%3Afe80%3A%3A1"))
	assert.Equal(t, "x-ipv6-0000000003-x", obfuscator.Contents("1::"))
	assert.Equal(t, map[string]string{
		"fd69::5":     "x-ipv6-0000000001-x",
		"fe80%3A%3A1": "x-ipv6-0000000002-x",
		"1::":         "x-ipv6-0000000003-x",
	}, obfuscator.Report().AsMap())
}

func TestIPObfuscatorReplacesSingleGroupCompressedIPv6(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  string
		output string
	}{
		{name: "one group", input: "1::", output: "x-ipv6-0000000001-x"},
		{name: "hexadecimal group", input: "a::", output: "x-ipv6-0000000001-x"},
		{name: "zero group", input: "0::", output: "x-ipv6-0000000001-x"},
		{name: "field label", input: "IP:a::b-foo", output: "IP:x-ipv6-0000000001-x-foo"},
		{name: "long field label", input: "IP:a::b-suffix", output: "IP:x-ipv6-0000000001-x-suffix"},
		{name: "numeric field label", input: "IP:1::2-suffix", output: "IP:x-ipv6-0000000001-x-suffix"},
		{name: "plain suffix", input: "a::b_suffix", output: "x-ipv6-0000000001-x_suffix"},
		{name: "encoded field label", input: "IP%3Aa%3A%3Ab-foo", output: "IP%3Ax-ipv6-0000000001-x-foo"},
		{name: "encoded suffix", input: "IP%3Aa%3A%3Ab-suffix", output: "IP%3Ax-ipv6-0000000001-x-suffix"},
		{name: "non hexadecimal word", input: "foo::bar", output: "foo::bar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			assert.Equal(t, tc.output, obfuscator.Contents(tc.input))
		})
	}
}

func TestIPObfuscatorPreservesEncodedTextualWordContainingColons(t *testing.T) {
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	for _, input := range []string{
		"foo::bar",
		"foo%3A%3Abar",
		"%66oo%3A%3Abar",
	} {
		assert.Equal(t, input, obfuscator.Contents(input))
	}
	assert.Empty(t, obfuscator.Report().Replacements)
}

func TestIPObfuscatorPreservesTerminalEmptyCPEField(t *testing.T) {
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	for _, input := range []string{
		"cpe:/o:redhat:enterprise_linux:9::",
		"cpe:/o:redhat:enterprise_linux:9::,",
		"cpe:/o:redhat:enterprise_linux:9%3A%3A",
		"cpe:/o:redhat:enterprise_linux:1::",
	} {
		assert.Equal(t, input, obfuscator.Contents(input))
	}
	assert.Empty(t, obfuscator.Report().Replacements)
}

func TestIPObfuscatorPreservesCPEIdentifierSuffixes(t *testing.T) {
	for _, input := range []string{
		"cpe:/o:redhat:enterprise_linux:9::baseos",
		"cpe:/o:redhat:enterprise_linux:99::baseos",
		"cpe:/o:redhat:enterprise_linux:999::baseos",
		"cpe:/o:redhat:enterprise_linux:9::baseos:foo",
		"cpe:/o:redhat:enterprise_linux:9%3A%3Abaseos",
		"cpe%3A%2Fo%3Aredhat%3Aenterprise_linux%3A9%3A%3Abaseos",
	} {
		t.Run(input, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			assert.Equal(t, input, obfuscator.Contents(input))
			assert.Empty(t, obfuscator.Report().Replacements)
		})
	}
}

func TestIPObfuscatorPreservesDecimalIPv6FieldsInsideStructuredIdentifiers(t *testing.T) {
	for _, input := range []string{
		"cpe:/a:1:2:3:4:5:6:7:8",
		"crn:v1:bluemix:public:1:2:3:4:5:6:7:8",
		"cpe%3A%2Fa%3A1%3A2%3A3%3A4%3A5%3A6%3A7%3A8",
		"crn%3Av1%3Abluemix%3Apublic%3A1%3A2%3A3%3A4%3A5%3A6%3A7%3A8",
		"cpe:/a:1:2:3:4:5:6:7:8:label",
		"cpe:/a:1:2:3:4:5:6:7:8/path",
	} {
		t.Run(input, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			assert.Equal(t, input, obfuscator.Contents(input))
			assert.Empty(t, obfuscator.Report().Replacements)
		})
	}
}

func TestIPObfuscatorMasksIPv6AfterTextualStructuredField(t *testing.T) {
	for _, tc := range []struct {
		input  string
		output string
	}{
		{
			input:  "cpe:/a:vendor:product:version:2001:db8::1",
			output: "cpe:/a:vendor:product:version:x-ipv6-0000000001-x",
		},
		{
			input:  "crn:v1:bluemix:public:resource:2001:db8::1",
			output: "crn:v1:bluemix:public:resource:x-ipv6-0000000001-x",
		},
		{
			input:  "crn%3Av1%3Abluemix%3Apublic%3Aresource%3A2001%3Adb8%3A%3A1",
			output: "crn%3Av1%3Abluemix%3Apublic%3Aresource%3Ax-ipv6-0000000001-x",
		},
	} {
		t.Run(tc.input, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			assert.Equal(t, tc.output, obfuscator.Contents(tc.input))
		})
	}
}

func TestIPObfuscatorPreservesStructuredFieldSuffixes(t *testing.T) {
	for _, input := range []string{
		"x:9::baseos",
		"x:99::baseos",
		"x:999::baseos",
		"field:9::label",
		"foo:9::baseos",
		"9::baseos",
		"x=9::baseos",
		"x/9::baseos",
		"x,9::baseos",
		"x%3A9%3A%3Abaseos",
		"x%3D9%3A%3Abaseos",
	} {
		t.Run(input, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			assert.Equal(t, input, obfuscator.Contents(input))
			assert.Empty(t, obfuscator.Report().Replacements)
		})
	}
}

func TestIPObfuscatorPreservesFullFormIPv6SuffixOfInvalidRun(t *testing.T) {
	for _, input := range []string{
		"x:1:2:3:4:5:6:7:8",
		"x%3A1%3A2%3A3%3A4%3A5%3A6%3A7%3A8",
	} {
		t.Run(input, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			assert.Equal(t, input, obfuscator.Contents(input))
			assert.Empty(t, obfuscator.Report().Replacements)
		})
	}
}

func TestIPObfuscatorMasksIPv4TailAfterStructuredIdentifier(t *testing.T) {
	for _, tc := range []struct {
		input  string
		output string
	}{
		{
			input:  "cpe:/o:redhat:enterprise_linux:9::baseos:1.2.3.4",
			output: "cpe:/o:redhat:enterprise_linux:9::baseos:x-ipv4-0000000001-x",
		},
		{
			input:  "cpe:/o:redhat:enterprise_linux:9::baseos:1%2E2%2E3%2E4",
			output: "cpe:/o:redhat:enterprise_linux:9::baseos:x-ipv4-0000000001-x",
		},
	} {
		t.Run(tc.input, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			assert.Equal(t, tc.output, obfuscator.Contents(tc.input))
		})
	}
}

func TestIPObfuscatorIPv6MatchStartsAfterLeadingNonIPText(t *testing.T) {
	input := "BEBzAz7d4y7x957Fe81-9BdxzFf_F9b/69/F8/daaxyBfFDy12bB1/CF12B3ze.D352:DF::62_e09B4_f7az_:AAd:zA/d67A60."
	expected := "BEBzAz7d4y7x957Fe81-9BdxzFf_F9b/69/F8/daaxyBfFDy12bB1/CF12B3ze.x-ipv6-0000000001-x_e09B4_f7az_:AAd:zA/d67A60."
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	assert.Equal(t, expected, obfuscator.Contents(input))
	assert.Equal(t, map[string]string{"D352:DF::62": "x-ipv6-0000000001-x"}, obfuscator.Report().AsMap())
}

func TestIPObfuscatorReplacesCompleteBareCompressedIPv6AtWordBoundary(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  string
		output string
	}{
		{name: "after a non-hexadecimal word letter", input: "openapi::2", output: "openapix-ipv6-0000000001-x"},
		{name: "after an underscore", input: "foo_::2", output: "foo_x-ipv6-0000000001-x"},
		{name: "after an encoded delimiter", input: "host%3D::2", output: "host%3Dx-ipv6-0000000001-x"},
		{name: "followed by a field separator", input: "openapi::2_suffix", output: "openapix-ipv6-0000000001-x_suffix"},
		{name: "terminal compression after identifier punctuation with CIDR", input: "foo-a::/64", output: "foo-a::/64"},
		{name: "terminal compression after encoded identifier punctuation with CIDR", input: "foo-a%3A%3A%2F64", output: "foo-a%3A%3A%2F64"},
		{name: "terminal compression after identifier punctuation", input: "foo-a::", output: "foo-a::"},
		{name: "numeric terminal compression with CIDR", input: "IP:1::/64", output: "IP:x-ipv6-0000000001-x/64"},
		{name: "embedded IPv4 after a word letter", input: "x::ffff:192.0.2.1", output: "xx-ipv6-0000000001-x"},
		{name: "encoded embedded IPv4 after a word letter", input: "x%3A%3Affff%3A192%2E0%2E2%2E1", output: "xx-ipv6-0000000001-x"},
		{name: "textual word suffix remains unchanged", input: "foo::bar", output: "foo::bar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			assert.Equal(t, tc.output, obfuscator.Contents(tc.input))
		})
	}
}

func TestIPObfuscatorHandlesFullFormIPv4EmbeddedIPv6(t *testing.T) {
	for _, tc := range []struct {
		input  string
		output string
	}{
		{input: "0:0:0:0:0:ffff:192.0.2.1", output: "x-ipv6-0000000001-x"},
		{input: "0:0:0:0:0:0:192.0.2.1", output: "x-ipv6-0000000001-x"},
		{input: "0%3A0%3A0%3A0%3A0%3Affff%3A192%2E0%2E2%2E1", output: "x-ipv6-0000000001-x"},
		{input: "0%3A0%3A0%3A0%3A0%3A0%3A192%2E0%2E2%2E1", output: "x-ipv6-0000000001-x"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			assert.Equal(t, tc.output, obfuscator.Contents(tc.input))
		})
	}
}

func TestIPObfuscatorRecoversAddressesJoinedByGreedyIPv6Match(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  string
		output string
		report map[string]string
	}{
		{
			name:   "raw",
			input:  "1:2:3:4:5:6:7:8:db8::1",
			output: "x-ipv6-0000000001-x:x-ipv6-0000000002-x",
			report: map[string]string{
				"1:2:3:4:5:6:7:8": "x-ipv6-0000000001-x",
				"db8::1":          "x-ipv6-0000000002-x",
			},
		},
		{
			name:   "two compressed addresses",
			input:  "2001:db8::1:db8::1",
			output: "x-ipv6-0000000001-x:x-ipv6-0000000002-x",
			report: map[string]string{
				"2001:db8::1": "x-ipv6-0000000001-x",
				"db8::1":      "x-ipv6-0000000002-x",
			},
		},
		{
			name:   "raw with encoded suffix delimiter",
			input:  "1:2:3:4:5:6:7:8:db8::1%20suffix",
			output: "x-ipv6-0000000001-x:x-ipv6-0000000002-x%20suffix",
			report: map[string]string{
				"1:2:3:4:5:6:7:8": "x-ipv6-0000000001-x",
				"db8::1":          "x-ipv6-0000000002-x",
			},
		},
		{
			name:   "encoded",
			input:  "1%3A2%3A3%3A4%3A5%3A6%3A7%3A8%3Adb8%3A%3A1",
			output: "x-ipv6-0000000001-x%3Ax-ipv6-0000000002-x",
			report: map[string]string{
				"1%3A2%3A3%3A4%3A5%3A6%3A7%3A8": "x-ipv6-0000000001-x",
				"db8%3A%3A1":                    "x-ipv6-0000000002-x",
			},
		},
		{
			name:   "fully encoded",
			input:  "%31%3A%32%3A%33%3A%34%3A%35%3A%36%3A%37%3A%38%3A%64%62%38%3A%3A%31",
			output: "x-ipv6-0000000001-x%3Ax-ipv6-0000000002-x",
			report: map[string]string{
				"%31%3A%32%3A%33%3A%34%3A%35%3A%36%3A%37%3A%38": "x-ipv6-0000000001-x",
				"%64%62%38%3A%3A%31":                            "x-ipv6-0000000002-x",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			output := obfuscator.Contents(tc.input)
			assert.Equal(t, tc.output, output)
			assert.Empty(t, findRawIPv6Matches(output))
			assert.Equal(t, tc.report, obfuscator.Report().AsMap())
		})
	}
}

func TestIPObfuscatorRejectsColonRunFalsePositives(t *testing.T) {
	for _, input := range []string{
		"crn:v1:bluemix:public:iam::::role",
		"client-ca-bundle::/etc/kubernetes/pki",
		"client-ca::/etc/tls/client/client-ca.crt",
		"client-ca::kube-system::extension-apiserver-authentication::client-ca-file",
		"client-ca::kube-system::extension-apiserver-authentication::requestheader-client-ca-file",
		"2021-09-14T14:14:11.242994815Z I0914 14:14:11.242883    7748 dynamic_cafile_content.go:167] Starting client-ca::/etc/tls/client/client-ca.crt",
		"CSISnapshotWebhookController_InitalSync::_InitalSync",
		"bind :::9445",
		"cpe:/o:redhat:enterprise_linux:9::baseos",
		"empty::::fields",
		"/org/freedesktop/NetworkManager/ActiveConnection/1::/run/NetworkManager/system-connections/lo.nmconnection",
		"::",
		"::::",
	} {
		t.Run(input, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			assert.Equal(t, input, obfuscator.Contents(input))
			assert.Empty(t, obfuscator.Report().Replacements)
		})
	}
}

func TestIPObfuscatorPreservesEncodedIdentifiersEndingInDoubleColon(t *testing.T) {
	for _, input := range []string{
		"client-ca-bundle%3A%3A%2Fetc%2Fkubernetes%2Fpki",
		"client-ca%3A%3A%2Fetc%2Ftls%2Fclient%2Fclient-ca.crt",
		"client-ca%3A%3Akube-system%3A%3Aextension-apiserver-authentication%3A%3Arequestheader-client-ca-file",
		"2021-09-14T14:14:11.242994815Z I0914 14:14:11.242883    7748 dynamic_cafile_content.go:167] Starting client-ca%3A%3A%2Fetc%2Ftls%2Fclient%2Fclient-ca.crt",
		"CSISnapshotWebhookController_InitalSync%3A%3A_InitalSync",
		"/org/freedesktop/NetworkManager/ActiveConnection/1%3A%3A%2Frun%2FNetworkManager%2Fsystem-connections%2Flo.nmconnection",
	} {
		t.Run(input, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			assert.Equal(t, input, obfuscator.Contents(input))
			assert.Empty(t, obfuscator.Report().Replacements)
		})
	}
}

func TestIPObfuscatorPreservesFieldLabelBeforeIPv6(t *testing.T) {
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	assert.Equal(t, "IP:x-ipv6-0000000001-x", obfuscator.Contents("IP:fd69::5"))
	assert.Equal(t, "V6OVNServiceHairpinMasqueradeIP:x-ipv6-0000000001-x", obfuscator.Contents("V6OVNServiceHairpinMasqueradeIP:fd69::5"))
	assert.Equal(t, "field_name:x-ipv6-0000000001-x", obfuscator.Contents("field_name:fd69::5"))
	assert.Equal(t, "IP:x-ipv6-0000000002-x", obfuscator.Contents("IP:::2"))
	assert.Equal(t, "abcdef:x-ipv6-0000000003-x", obfuscator.Contents("abcdef:2001:db8::1"))
	assert.Equal(t, "12345:x-ipv6-0000000003-x", obfuscator.Contents("12345:2001:db8::1"))
	assert.Equal(t, "field_name%3Ax-ipv6-0000000001-x", obfuscator.Contents("field_name%3Afd69%3A%3A5"))
	assert.Equal(t, map[string]string{
		"fd69::5":     "x-ipv6-0000000001-x",
		"fd69%3A%3A5": "x-ipv6-0000000001-x",
		"::2":         "x-ipv6-0000000002-x",
		"2001:db8::1": "x-ipv6-0000000003-x",
	}, obfuscator.Report().AsMap())
}

func TestIPObfuscatorSkipsInvalidPercentEncoding(t *testing.T) {
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	assert.Equal(t, "bad=%zzx-ipv4-0000000001-x", obfuscator.Contents("bad=%zz192.0.2.1"))
	assert.Equal(t, map[string]string{
		"192.0.2.1": "x-ipv4-0000000001-x",
	}, obfuscator.Report().AsMap())
}

func TestIPObfuscatorConsistent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  []string
		output []string
		report map[string]string
	}{
		{
			name:   "valid ipv4 address",
			input:  []string{"received request from 192.168.1.10"},
			output: []string{"received request from x-ipv4-0000000001-x"},
			report: map[string]string{"192.168.1.10": "x-ipv4-0000000001-x"},
		},
		{
			name:   "ipv4 in words",
			input:  []string{"calling https://192.168.1.20/metrics for values"},
			output: []string{"calling https://x-ipv4-0000000001-x/metrics for values"},
			report: map[string]string{"192.168.1.20": "x-ipv4-0000000001-x"},
		},
		{
			name:   "multiple ipv4s",
			input:  []string{"received request from 192.168.1.20 proxied through 192.168.1.3"},
			output: []string{"received request from x-ipv4-0000000001-x proxied through x-ipv4-0000000002-x"},
			report: map[string]string{
				"192.168.1.20": "x-ipv4-0000000001-x",
				"192.168.1.3":  "x-ipv4-0000000002-x",
			},
		},
		{
			name:   "valid ipv6 address",
			input:  []string{"received request from 2001:db8::ff00:42:8329"},
			output: []string{"received request from x-ipv6-0000000001-x"},
			report: map[string]string{
				"2001:db8::ff00:42:8329": "x-ipv6-0000000001-x",
			},
		},
		{
			name:   "mixed ipv4 and ipv6",
			input:  []string{"tunneling ::2fa:bf9 as 192.168.1.30"},
			output: []string{"tunneling x-ipv6-0000000001-x as x-ipv4-0000000001-x"},
			report: map[string]string{
				"192.168.1.30": "x-ipv4-0000000001-x",
				"::2fa:bf9":    "x-ipv6-0000000001-x",
			},
		},
		{
			name: "multiple invocations",
			input: []string{
				"received request from 192.168.1.20 for 192.168.1.30",
				"received request from 192.168.1.20 for 192.168.1.30",
			},
			output: []string{
				"received request from x-ipv4-0000000001-x for x-ipv4-0000000002-x",
				"received request from x-ipv4-0000000001-x for x-ipv4-0000000002-x",
			},
			report: map[string]string{
				"192.168.1.20": "x-ipv4-0000000001-x",
				"192.168.1.30": "x-ipv4-0000000002-x",
			},
		},
		{
			name: "multiple invocations with different IPs",
			input: []string{
				"received request from 192.168.1.20 for 192.168.1.30",
				"received request from 192.168.1.21 for 192.168.1.31",
				"received request from 192.168.1.22 for 192.168.1.32",
				"received request from 192.168.1.23 for 192.168.1.33",
				"received request from 192.168.1.24 for 192.168.1.34",
			},
			output: []string{
				"received request from x-ipv4-0000000001-x for x-ipv4-0000000002-x",
				"received request from x-ipv4-0000000003-x for x-ipv4-0000000004-x",
				"received request from x-ipv4-0000000005-x for x-ipv4-0000000006-x",
				"received request from x-ipv4-0000000007-x for x-ipv4-0000000008-x",
				"received request from x-ipv4-0000000009-x for x-ipv4-0000000010-x",
			},
			report: map[string]string{
				"192.168.1.20": "x-ipv4-0000000001-x",
				"192.168.1.21": "x-ipv4-0000000003-x",
				"192.168.1.22": "x-ipv4-0000000005-x",
				"192.168.1.23": "x-ipv4-0000000007-x",
				"192.168.1.24": "x-ipv4-0000000009-x",
				"192.168.1.30": "x-ipv4-0000000002-x",
				"192.168.1.31": "x-ipv4-0000000004-x",
				"192.168.1.32": "x-ipv4-0000000006-x",
				"192.168.1.33": "x-ipv4-0000000008-x",
				"192.168.1.34": "x-ipv4-0000000010-x",
			},
		},
		{
			name:   "standard ipv4 with colons and standard ipv4 with dashes between should map to the same obfuscation value",
			input:  []string{"obfuscate 10.0.129.220 and 10-0-129-220"},
			output: []string{"obfuscate x-ipv4-0000000001-x and x-ipv4-0000000001-x"},
			report: map[string]string{
				"10.0.129.220": "x-ipv4-0000000001-x",
				"10-0-129-220": "x-ipv4-0000000001-x",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)
			for i := 0; i < len(tc.input); i++ {
				assert.Equal(t, tc.output[i], o.Contents(tc.input[i]))
			}
			assert.Equal(t, tc.report, o.Report().AsMap())
		})
	}
}

func TestIPObfuscationInPaths(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "installer path",
			input:  "quay-io-release/namespaces/openshift-kube-apiserver/pods/installer-4-ip-10-0-187-218.ec2.internal/installer-4-ip-10-0-187-218.ec2.internal.yaml",
			output: "quay-io-release/namespaces/openshift-kube-apiserver/pods/installer-4-ip-x-ipv4-0000000001-x.ec2.internal/installer-4-ip-x-ipv4-0000000001-x.ec2.internal.yaml",
		}, {
			name:   "revision pruner path",
			input:  "quay-io-release/namespaces/openshift-kube-apiserver/pods/revision-pruner-9-ip-10-0-189-142.ec2.internal/revision-pruner-9-ip-10-0-189-142.ec2.internal.yaml",
			output: "quay-io-release/namespaces/openshift-kube-apiserver/pods/revision-pruner-9-ip-x-ipv4-0000000001-x.ec2.internal/revision-pruner-9-ip-x-ipv4-0000000001-x.ec2.internal.yaml",
		},
		{
			name:   "etcd pod logs",
			input:  "quay-io-release/namespaces/openshift-etcd/pods/etcd-ip-10-0-189-142.ec2.internal/etcd/etcd/logs/current.log",
			output: "quay-io-release/namespaces/openshift-etcd/pods/etcd-ip-x-ipv4-0000000001-x.ec2.internal/etcd/etcd/logs/current.log",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)
			obfuscated := o.Path(tc.input)
			assert.Equal(t, tc.output, obfuscated)
		})
	}

}

func TestIPv6CanonicalKeyConsistent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  string
		output string
		report ReplacementReport
	}{
		{
			name:   "valid ipv6 address",
			input:  "received request from 2001:db8::ff00:42:8329",
			output: "received request from x-ipv6-0000000001-x",
			report: ReplacementReport{[]Replacement{
				{Canonical: "2001:DB8::FF00:42:8329", ReplacedWith: "x-ipv6-0000000001-x", Counter: map[string]uint{
					"2001:db8::ff00:42:8329": 1,
				}},
			}},
		},
		{
			name:   "mixed ipv4 and ipv6",
			input:  "tunneling ::2fa:bf9 as 192.168.1.30",
			output: "tunneling x-ipv6-0000000001-x as x-ipv4-0000000001-x",
			report: ReplacementReport{[]Replacement{
				{Canonical: "192.168.1.30", ReplacedWith: "x-ipv4-0000000001-x",
					Counter: map[string]uint{
						"192.168.1.30": 1,
					}},
				{Canonical: "::2FA:BF9", ReplacedWith: "x-ipv6-0000000001-x",
					Counter: map[string]uint{
						"::2fa:bf9": 1,
					}},
			}},
		},
		{
			name:   "mixed ipv4 and ipv6 logline",
			input:  "2021-08-03T09:35:59.743794348Z ::ffff:10.130.0.1 - - [03/Aug/2021 09:25:59] \"GET / HTTP/1.1\" 200 - 2021-08-03T09:35:59.743794348Z ::ffff:10.130.0.1 - - [03/Aug/2021 09:25:59] \"GET / HTTP/1.1\" 200 - 2021-08-03T09:35:59.743794348Z ::ffff:10.130.0.1 - - [03/Aug/2021 09:25:59] \"GET / HTTP/1.1\" 200 - 2021-08-03T09:35:59.743794348Z ::ffff:10.130.0.1 - - [03/Aug/2021 09:25:59] \"GET / HTTP/1.1\" 200 - 2021-08-03T09:35:59.743794348Z ::ffff:10.130.0.1 - - [03/Aug/2021 09:25:59] \"GET / HTTP/1.1\" 200 - 2021-08-03T09:35:59.743794348Z ::ffff:10.130.0.1 - - [03/Aug/2021 09:25:59] \"GET / HTTP/1.1\" 200 -",
			output: "2021-08-03T09:35:59.743794348Z x-ipv6-0000000001-x - - [03/Aug/2021 09:25:59] \"GET / HTTP/1.1\" 200 - 2021-08-03T09:35:59.743794348Z x-ipv6-0000000001-x - - [03/Aug/2021 09:25:59] \"GET / HTTP/1.1\" 200 - 2021-08-03T09:35:59.743794348Z x-ipv6-0000000001-x - - [03/Aug/2021 09:25:59] \"GET / HTTP/1.1\" 200 - 2021-08-03T09:35:59.743794348Z x-ipv6-0000000001-x - - [03/Aug/2021 09:25:59] \"GET / HTTP/1.1\" 200 - 2021-08-03T09:35:59.743794348Z x-ipv6-0000000001-x - - [03/Aug/2021 09:25:59] \"GET / HTTP/1.1\" 200 - 2021-08-03T09:35:59.743794348Z x-ipv6-0000000001-x - - [03/Aug/2021 09:25:59] \"GET / HTTP/1.1\" 200 -",
			report: ReplacementReport{[]Replacement{
				{Canonical: "::FFFF:10.130.0.1", ReplacedWith: "x-ipv6-0000000001-x",
					Counter: map[string]uint{
						"::ffff:10.130.0.1": 6,
					}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)
			output := o.Contents(tc.input)
			assert.Equal(t, tc.output, output)
			replacementReportsMatch(t, tc.report, o.Report())
		})
	}
}

func TestIPv6CanonicalKeyStatic(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  string
		output string
		report ReplacementReport
	}{
		{
			name:   "valid ipv6 address",
			input:  "received request from 2001:db8::ff00:42:8329",
			output: "received request from xxxx:xxxx:xxxx:xxxx:xxxx:xxxx:xxxx:xxxx",
			report: ReplacementReport{[]Replacement{
				{Canonical: "2001:DB8::FF00:42:8329", ReplacedWith: obfuscatedStaticIPv6,
					Counter: map[string]uint{
						"2001:db8::ff00:42:8329": 1,
					},
				},
			}},
		},
		{
			name:   "mixed ipv4 and ipv6",
			input:  "tunneling ::2fa:bf9 as 192.168.1.30",
			output: "tunneling xxxx:xxxx:xxxx:xxxx:xxxx:xxxx:xxxx:xxxx as xxx.xxx.xxx.xxx",
			report: ReplacementReport{[]Replacement{
				{Canonical: "::2FA:BF9", ReplacedWith: obfuscatedStaticIPv6,
					Counter: map[string]uint{
						"::2fa:bf9": 1,
					}},
				{Canonical: "192.168.1.30", ReplacedWith: obfuscatedStaticIPv4,
					Counter: map[string]uint{
						"192.168.1.30": 1,
					}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewIPObfuscator(schema.ObfuscateReplacementTypeStatic, NewSimpleTracker())
			require.NoError(t, err)
			output := o.Contents(tc.input)
			assert.Equal(t, tc.output, output)
			replacementReportsMatch(t, tc.report, o.Report())
		})
	}
}

func TestIPObfuscatorWithCount(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  string
		output string
		report ReplacementReport
	}{
		{
			name:   "valid ipv4 address four times",
			input:  "same IP 192.168.1.10 address 192.168.1.10 repeated 192.168.1.10 four times 192.168.1.10",
			output: "same IP xxx.xxx.xxx.xxx address xxx.xxx.xxx.xxx repeated xxx.xxx.xxx.xxx four times xxx.xxx.xxx.xxx",
			report: ReplacementReport{[]Replacement{
				{Canonical: "192.168.1.10", ReplacedWith: obfuscatedStaticIPv4,
					Counter: map[string]uint{
						"192.168.1.10": 4,
					}},
			}},
		},
		{
			name:   "valid ipv4 address dots and dashes",
			input:  "same IP 192.168.1.10 address 192-168-1-10 repeated 192.168.1.10 four times 192-168-1-10",
			output: "same IP xxx.xxx.xxx.xxx address xxx.xxx.xxx.xxx repeated xxx.xxx.xxx.xxx four times xxx.xxx.xxx.xxx",
			report: ReplacementReport{[]Replacement{
				{Canonical: "192.168.1.10", ReplacedWith: obfuscatedStaticIPv4,
					Counter: map[string]uint{
						"192.168.1.10": 2,
						"192-168-1-10": 2,
					},
				},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewIPObfuscator(schema.ObfuscateReplacementTypeStatic, NewSimpleTracker())
			require.NoError(t, err)
			output := o.Contents(tc.input)
			assert.Equal(t, tc.output, output)
			replacementReportsMatch(t, tc.report, o.Report())
		})
	}
}

func TestIPObfuscatorSkipsImpossibleRegexInputs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
	}{
		{
			name:  "typical log line",
			input: strings.Repeat("2026-10-23T12:34:56.789Z level=info msg=\"something happened\" component=example value=42\n", 10000),
		},
		{
			name:  "colon-heavy run",
			input: strings.Repeat("x:", 100000),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			assert.Equal(t, tc.input, obfuscator.Contents(tc.input))
			assert.Empty(t, obfuscator.Report().Replacements)
		})
	}
}

func TestIPObfuscatorRejectsIPv4SubstringInsideNumericToken(t *testing.T) {
	input := "85a8e.2-gx:0EDA4C4E2dx.A1ecCg429bgc3.8427_87_4-3_bAgc2fF03864aDbc7"
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	assert.Equal(t, input, obfuscator.Contents(input))
	assert.Empty(t, obfuscator.Report().Replacements)
}

func TestIPObfuscatorIPv4ScannerBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  string
		output string
		report map[string]string
	}{
		{
			name:   "mixed underscore and hyphen separators",
			input:  "host=1_2-3_4",
			output: "host=x-ipv4-0000000001-x",
			report: map[string]string{"1_2-3_4": "x-ipv4-0000000001-x"},
		},
		{
			name:   "invalid final octet keeps a contiguous numeric run",
			input:  "1.2.3.999.4.5.6.7",
			output: "1.2.3.999.4.5.6.7",
			report: map[string]string{},
		},
		{
			name:   "zero-padded middle octet keeps a contiguous numeric run",
			input:  "1.02.3.4.5.6.7",
			output: "1.02.3.4.5.6.7",
			report: map[string]string{},
		},
		{
			name:   "long final numeric run is not an address",
			input:  "1.2.3.9994567890",
			output: "1.2.3.9994567890",
			report: map[string]string{},
		},
		{
			name:   "four digits after the final dot are not an address prefix",
			input:  "version 0.37.4.10188",
			output: "version 0.37.4.10188",
			report: map[string]string{},
		},
		{
			name:   "invalid final octet is not recovered as an address prefix",
			input:  "x=192.0.2.999",
			output: "x=192.0.2.999",
			report: map[string]string{},
		},
		{
			name:   "continued dotted run is not recovered as an address prefix",
			input:  "x=192.0.2.256.7",
			output: "x=192.0.2.256.7",
			report: map[string]string{},
		},
		{
			name:   "suffix starting inside an invalid octet is not an address",
			input:  "x=192.0.2.256.1.2.3",
			output: "x=192.0.2.256.1.2.3",
			report: map[string]string{},
		},
		{
			name:   "adjacent complete addresses remain independent",
			input:  "0.0.0.1.10.0.0.1",
			output: "x-ipv4-0000000001-x.x-ipv4-0000000002-x",
			report: map[string]string{
				"0.0.0.1":  "x-ipv4-0000000001-x",
				"10.0.0.1": "x-ipv4-0000000002-x",
			},
		},
		{
			name:   "repeated dots delimit complete addresses",
			input:  "0.0.0.1..10.0.0.1",
			output: "x-ipv4-0000000001-x..x-ipv4-0000000002-x",
			report: map[string]string{
				"0.0.0.1":  "x-ipv4-0000000001-x",
				"10.0.0.1": "x-ipv4-0000000002-x",
			},
		},
		{
			name:   "encoded suffix starting inside an invalid octet is not an address",
			input:  "x=192%2E0%2E2%2E256%2E1%2E2%2E3",
			output: "x=192%2E0%2E2%2E256%2E1%2E2%2E3",
			report: map[string]string{},
		},
		{
			name:   "mixed encoded suffix starting inside an invalid octet is not an address",
			input:  "x=192%2E0.2.256.1%2E2.3",
			output: "x=192%2E0.2.256.1%2E2.3",
			report: map[string]string{},
		},
		{
			name:   "dotted numeric token after a textual label is not an address",
			input:  "node1.10.0.0.2",
			output: "node1.10.0.0.2",
			report: map[string]string{},
		},
		{
			name:   "alternate separator does not start inside a numeric token",
			input:  "timestamp=2021_09_13_15_00_14.222807563",
			output: "timestamp=2021_09_13_15_00_14.222807563",
			report: map[string]string{},
		},
		{
			name:   "alternate separator does not start inside a nonzero numeric token",
			input:  "/config/..2021_09_13_14_57_42.353369777/kube-proxy-config.yaml",
			output: "/config/..2021_09_13_14_57_42.353369777/kube-proxy-config.yaml",
			report: map[string]string{},
		},
		{
			name:   "match cannot begin inside a longer middle octet",
			input:  "1.1234.5.6.7",
			output: "1.1234.5.6.7",
			report: map[string]string{},
		},
		{
			name:   "non-standard address after a textual label remains supported",
			input:  "ip-10-0-129-220.ec2.internal",
			output: "ip-x-ipv4-0000000001-x.ec2.internal",
			report: map[string]string{"10-0-129-220": "x-ipv4-0000000001-x"},
		},
		{
			name:   "address after a textual label ending in a digit remains supported",
			input:  "node1-10.0.0.1 node1_10.0.0.2",
			output: "node1-x-ipv4-0000000001-x node1_x-ipv4-0000000002-x",
			report: map[string]string{
				"10.0.0.1": "x-ipv4-0000000001-x",
				"10.0.0.2": "x-ipv4-0000000002-x",
			},
		},
		{
			name:   "address after a regex whitespace escape remains supported",
			input:  "awk '/\\s192.168.79.2\\// {print $2}'",
			output: "awk '/\\sx-ipv4-0000000001-x\\// {print $2}'",
			report: map[string]string{"192.168.79.2": "x-ipv4-0000000001-x"},
		},
		{
			name:   "numeric separator after an invalid octet keeps the suffix in the token",
			input:  "192.0.2.256-1.2.3 192.0.2.256_4.5.6",
			output: "192.0.2.256-1.2.3 192.0.2.256_4.5.6",
			report: map[string]string{},
		},
		{
			name:   "adjacent dotted addresses joined by underscore remain independent",
			input:  "1.2.3.4_5.6.7.8",
			output: "x-ipv4-0000000001-x_x-ipv4-0000000002-x",
			report: map[string]string{
				"1.2.3.4": "x-ipv4-0000000001-x",
				"5.6.7.8": "x-ipv4-0000000002-x",
			},
		},
		{
			name:   "adjacent dotted addresses joined by hyphen remain independent",
			input:  "1.2.3.4-5.6.7.8",
			output: "x-ipv4-0000000001-x-x-ipv4-0000000002-x",
			report: map[string]string{
				"1.2.3.4": "x-ipv4-0000000001-x",
				"5.6.7.8": "x-ipv4-0000000002-x",
			},
		},
		{
			name:   "valid address after malformed dotted token joined by underscore",
			input:  "192.0.2.256_4.5.6.7",
			output: "192.0.2.256_x-ipv4-0000000001-x",
			report: map[string]string{"4.5.6.7": "x-ipv4-0000000001-x"},
		},
		{
			name:   "valid address after malformed dotted token joined by hyphen",
			input:  "192.0.2.256-4.5.6.7",
			output: "192.0.2.256-x-ipv4-0000000001-x",
			report: map[string]string{"4.5.6.7": "x-ipv4-0000000001-x"},
		},
		{
			name:   "valid address after malformed dotted run and underscore",
			input:  "192.0.2.256.1.2.3_192.0.2.1",
			output: "192.0.2.256.1.2.3_x-ipv4-0000000001-x",
			report: map[string]string{"192.0.2.1": "x-ipv4-0000000001-x"},
		},
		{
			name:   "encoded adjacent dotted addresses joined by underscore remain independent",
			input:  "1%2E2%2E3%2E4_5%2E6%2E7%2E8",
			output: "x-ipv4-0000000001-x_x-ipv4-0000000002-x",
			report: map[string]string{
				"1%2E2%2E3%2E4": "x-ipv4-0000000001-x",
				"5%2E6%2E7%2E8": "x-ipv4-0000000002-x",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			assert.Equal(t, tc.output, obfuscator.Contents(tc.input))
			assert.Equal(t, tc.report, obfuscator.Report().AsMap())
		})
	}
}

func TestIPObfuscatorFullIPv6WithoutEllipsis(t *testing.T) {
	input := "received request from fe80:0:0:0:0:0:0:1"
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	assert.Equal(t, "received request from x-ipv6-0000000001-x", obfuscator.Contents(input))
	assert.Equal(t, map[string]string{
		"fe80:0:0:0:0:0:0:1": "x-ipv6-0000000001-x",
	}, obfuscator.Report().AsMap())
}

func TestIPObfuscatorRejectsIPv6WithTooManyGroups(t *testing.T) {
	for _, input := range []string{
		"x=1:2:3:4:5:6:7:8:9",
		"x=1:2:3:4:5:6:7:8:9:10",
	} {
		t.Run(input, func(t *testing.T) {
			obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			assert.Equal(t, input, obfuscator.Contents(input))
			assert.Empty(t, obfuscator.Report().Replacements)
		})
	}
}

func TestIPObfuscatorIPv6PrefilterBoundaries(t *testing.T) {
	assert.False(t, mightContainIPv6("aaaa:bbbb:cccc:dddd:eeee:ffff:1111:zzzz"))
	assert.True(t, mightContainIPv6("aaaa:bbbb:cccc:dddd:eeee:ffff:1111:2222"))
	assert.True(t, mightContainIPv6("0:0:0:0:0:ffff:192.0.2.1"))
	assert.True(t, mightContainIPv6("0:0:0:0:0:ffff:192_0_2_1"))
	assert.True(t, mightContainIPv6("fe80::1"))
}

func BenchmarkIPObfuscatorColonHeavyRun(b *testing.B) {
	input := strings.Repeat("x:", 100000)
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		_ = obfuscator.Contents(input)
	}
}

func BenchmarkIPObfuscatorTypicalLog(b *testing.B) {
	line := "2026-10-23T12:34:56.789Z level=info msg=\"something happened\" component=example value=42\n"
	input := strings.Repeat(line, 10000)
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		_ = obfuscator.Contents(input)
	}
}
