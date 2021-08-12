package obfuscator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIPObfuscator(t *testing.T) {
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
			o := NewIPObfuscator()
			output := o.Contents(tc.input)
			assert.Equal(t, tc.output, output)
			assert.Equal(t, tc.report, o.ReportingResult())
		})
	}
}
