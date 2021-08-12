package obfuscator

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewIPObfuscator(schema.ObfuscateReplacementTypeStatic)
			assert.NoError(t, err)
			output := o.Contents(tc.input)
			assert.Equal(t, tc.output, output)
			assert.Equal(t, tc.report, o.ReportingResult())
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent)
			assert.NoError(t, err)
			for i := 0; i < len(tc.input); i++ {
				assert.Equal(t, tc.output[i], o.Contents(tc.input[i]))
			}
			assert.Equal(t, tc.report, o.ReportingResult())
		})
	}
}

func TestPanicMaximumReplacements(t *testing.T) {
	o, err := NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent)
	assert.NoError(t, err)
	iobf := o.(*ipObfuscator)
	iobf.replacements[ipv4Pattern] = ipGenerator{
		count: maximumSupportedObfuscations,
	}
	assert.Panicsf(t, func() {
		o.Contents("192.168.1.1")
	}, "did not panic")
}
