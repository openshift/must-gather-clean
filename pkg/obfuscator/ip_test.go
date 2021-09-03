package obfuscator

import (
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
			name:   "invalid ipv4 address",
			input:  "value 910.218.98.1 is not an ipv4",
			output: "value 910.218.98.1 is not an ipv4",
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
			// This testcase reports both the detected IP address as well as the Normalized/Cleaned IP address
			name:   "non standard ipv4",
			input:  "ip-10-0-129-220.ec2.aws.yaml",
			output: "ip-xxx.xxx.xxx.xxx.ec2.aws.yaml",
			report: map[string]string{
				"10-0-129-220": obfuscatedStaticIPv4,
				"10.0.129.220": obfuscatedStaticIPv4,
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
				"10.0.129.220": "xxx.xxx.xxx.xxx",
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewIPObfuscator(schema.ObfuscateReplacementTypeStatic)
			require.NoError(t, err)
			output := o.Contents(tc.input)
			assert.Equal(t, tc.output, output)
			assert.Equal(t, tc.report, o.Report())
		})
	}
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
			output: []string{"received request from x-ipv4-000001-x"},
			report: map[string]string{"192.168.1.10": "x-ipv4-000001-x"},
		},
		{
			name:   "ipv4 in words",
			input:  []string{"calling https://192.168.1.20/metrics for values"},
			output: []string{"calling https://x-ipv4-000001-x/metrics for values"},
			report: map[string]string{"192.168.1.20": "x-ipv4-000001-x"},
		},
		{
			name:   "multiple ipv4s",
			input:  []string{"received request from 192.168.1.20 proxied through 192.168.1.3"},
			output: []string{"received request from x-ipv4-000001-x proxied through x-ipv4-000002-x"},
			report: map[string]string{
				"192.168.1.20": "x-ipv4-000001-x",
				"192.168.1.3":  "x-ipv4-000002-x",
			},
		},
		{
			name:   "valid ipv6 address",
			input:  []string{"received request from 2001:db8::ff00:42:8329"},
			output: []string{"received request from xxxxxxxxxxxxx-ipv6-000001-xxxxxxxxxxxxx"},
			report: map[string]string{
				"2001:db8::ff00:42:8329": "xxxxxxxxxxxxx-ipv6-000001-xxxxxxxxxxxxx",
			},
		},
		{
			name:   "mixed ipv4 and ipv6",
			input:  []string{"tunneling ::2fa:bf9 as 192.168.1.30"},
			output: []string{"tunneling xxxxxxxxxxxxx-ipv6-000001-xxxxxxxxxxxxx as x-ipv4-000001-x"},
			report: map[string]string{
				"192.168.1.30": "x-ipv4-000001-x",
				"::2fa:bf9":    "xxxxxxxxxxxxx-ipv6-000001-xxxxxxxxxxxxx",
			},
		},
		{
			name: "multiple invocations",
			input: []string{
				"received request from 192.168.1.20 for 192.168.1.30",
				"received request from 192.168.1.20 for 192.168.1.30",
			},
			output: []string{
				"received request from x-ipv4-000001-x for x-ipv4-000002-x",
				"received request from x-ipv4-000001-x for x-ipv4-000002-x",
			},
			report: map[string]string{
				"192.168.1.20": "x-ipv4-000001-x",
				"192.168.1.30": "x-ipv4-000002-x",
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
				"received request from x-ipv4-000001-x for x-ipv4-000002-x",
				"received request from x-ipv4-000003-x for x-ipv4-000004-x",
				"received request from x-ipv4-000005-x for x-ipv4-000006-x",
				"received request from x-ipv4-000007-x for x-ipv4-000008-x",
				"received request from x-ipv4-000009-x for x-ipv4-000010-x",
			},
			report: map[string]string{
				"192.168.1.20": "x-ipv4-000001-x",
				"192.168.1.21": "x-ipv4-000003-x",
				"192.168.1.22": "x-ipv4-000005-x",
				"192.168.1.23": "x-ipv4-000007-x",
				"192.168.1.24": "x-ipv4-000009-x",
				"192.168.1.30": "x-ipv4-000002-x",
				"192.168.1.31": "x-ipv4-000004-x",
				"192.168.1.32": "x-ipv4-000006-x",
				"192.168.1.33": "x-ipv4-000008-x",
				"192.168.1.34": "x-ipv4-000010-x",
			},
		},
		{
			name:   "standard ipv4 with colons and standard ipv4 with dashes between should map to the same obfuscation value",
			input:  []string{"obfuscate 10.0.129.220 and 10-0-129-220"},
			output: []string{"obfuscate x-ipv4-000001-x and x-ipv4-000001-x"},
			report: map[string]string{
				"10.0.129.220": "x-ipv4-000001-x",
				"10-0-129-220": "x-ipv4-000001-x",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent)
			require.NoError(t, err)
			for i := 0; i < len(tc.input); i++ {
				assert.Equal(t, tc.output[i], o.Contents(tc.input[i]))
			}
			assert.Equal(t, tc.report, o.Report())
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
			output: "quay-io-release/namespaces/openshift-kube-apiserver/pods/installer-4-ip-x-ipv4-000001-x.ec2.internal/installer-4-ip-x-ipv4-000001-x.ec2.internal.yaml",
		}, {
			name:   "revision pruner path",
			input:  "quay-io-release/namespaces/openshift-kube-apiserver/pods/revision-pruner-9-ip-10-0-189-142.ec2.internal/revision-pruner-9-ip-10-0-189-142.ec2.internal.yaml",
			output: "quay-io-release/namespaces/openshift-kube-apiserver/pods/revision-pruner-9-ip-x-ipv4-000001-x.ec2.internal/revision-pruner-9-ip-x-ipv4-000001-x.ec2.internal.yaml",
		},
		{
			name:   "etcd pod logs",
			input:  "quay-io-release/namespaces/openshift-etcd/pods/etcd-ip-10-0-189-142.ec2.internal/etcd/etcd/logs/current.log",
			output: "quay-io-release/namespaces/openshift-etcd/pods/etcd-ip-x-ipv4-000001-x.ec2.internal/etcd/etcd/logs/current.log",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent)
			require.NoError(t, err)
			obfuscated := o.Path(tc.input)
			assert.Equal(t, tc.output, obfuscated)
		})
	}

}
