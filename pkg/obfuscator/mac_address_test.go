package obfuscator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/openshift/must-gather-clean/pkg/schema"
)

func TestMacStaticReplacement(t *testing.T) {
	o, _ := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeStatic)
	assert.Equal(t, staticMacReplacement, o.Contents("29-7E-8C-8C-60-C9"))
	assert.Equal(t, map[string]string{"29-7E-8C-8C-60-C9": staticMacReplacement}, o.Report())
}

func TestMacConsistentReplacement(t *testing.T) {
        o, _ := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeConsistent)
        assert.Equal(t, "x-mac-000001-x", o.Contents("29-7E-8C-8C-60-C9"))
        assert.Equal(t, map[string]string{"29-7E-8C-8C-60-C9": "x-mac-000001-x"}, o.Report())
}

func TestMacReplacementManyMatchLine(t *testing.T) {
	input := "ss eb:a1:2a:b2:09:bf as 29-7E-8C-8C-60-C9 with some stuff around it and lowercased eb-a1-2a-b2-09-bf"
	expected := "ss xx:xx:xx:xx:xx:xx as xx:xx:xx:xx:xx:xx with some stuff around it and lowercased xx:xx:xx:xx:xx:xx"
	o, _ := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeStatic)
	assert.Equal(t, expected, o.Contents(input))
	assert.Equal(t, map[string]string{
		"eb:a1:2a:b2:09:bf": staticMacReplacement,
		"29-7E-8C-8C-60-C9": staticMacReplacement,
		"eb-a1-2a-b2-09-bf": staticMacReplacement,
	}, o.Report())
}

func TestMacReplacementStatic(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		input                string
		expectedOutput       string
		expectedReportOutput map[string]string
	}{
		{name: "squashed", input: "69806FE67C05", expectedOutput: "69806FE67C05"},
		{name: "squashed-lowercase", input: "69806fe67c05", expectedOutput: "69806fe67c05"},
		{name: "uppercase-colon", input: "69:80:6F:E6:7C:05", expectedOutput: staticMacReplacement},
		{name: "lowercase-dash", input: "eb-a1-2a-b2-09-bf", expectedOutput: staticMacReplacement},
		{name: "lowercase-colon", input: "eb:a1:2a:b2:09:bf", expectedOutput: staticMacReplacement},
		{name: "multi-colon", input: "eb:a1:2a:b2:09:bf eb:a1:2a:b2:09:bf", expectedOutput: staticMacReplacement + " " + staticMacReplacement},
		{name: "multi-colon-dash", input: "16-7C-44-26-24-14 BF:51:A4:1B:7D:0B", expectedOutput: staticMacReplacement + " " + staticMacReplacement},
		{name: "mac surrounded", input: "mac 52:df:20:08:6c:ff caused some trouble", expectedOutput: fmt.Sprintf("mac %s caused some trouble", staticMacReplacement)},
		{name: "mac as guid", input: "4a5299ac-6104-479d-aed4-b79faedffcb4", expectedOutput: "4a5299ac-6104-479d-aed4-b79faedffcb4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, _ := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeStatic)
			assert.Equal(t, tc.expectedOutput, o.Contents(tc.input))
		})
	}
}

func TestMACConsistentObfuscator(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  []string
		output []string
		report map[string]string
	}{
		{
			name:   "valid MAC address",
			input:  []string{"received request from 29-7E-8C-8C-60-C9"},
			output: []string{"received request from x-mac-000001-x"},
			report: map[string]string{"29-7E-8C-8C-60-C9": "x-mac-000001-x"},
		},
		{
			name:   "MAC address mentioned in a sentence",
			input:  []string{"A MAC address of 2c549188c9e3 is typically displayed as 2C:54:91:88:C9:E3"},
			output: []string{"A MAC address of 2c549188c9e3 is typically displayed as x-mac-000001-x"},
			report: map[string]string{"2C:54:91:88:C9:E3": "x-mac-000001-x"},
		},
		{
			name:   "Multiple MAC addresses",
			input:  []string{"MAC addresses of the two network interfaces are 2C:54:91:88:C9:E3 and 2C:56:83:91:C9:E6"},
			output: []string{"MAC addresses of the two network interfaces are x-mac-000001-x and x-mac-000002-x"},
			report: map[string]string{
				"2C:54:91:88:C9:E3": "x-mac-000001-x",
				"2C:56:83:91:C9:E6":  "x-mac-000002-x",
			},
		},
		{
			name: "Multiple invocations",
			input: []string{
				"MAC addresses of the two network interfaces are 2C:54:91:88:C9:E3 and 2C:56:83:91:C9:E6",
				"MAC addresses of the two network interfaces are 2C:54:91:88:C9:E3 and 2C:56:83:91:C9:E6",
			},
			output: []string{
				"MAC addresses of the two network interfaces are x-mac-000001-x and x-mac-000002-x",
				"MAC addresses of the two network interfaces are x-mac-000001-x and x-mac-000002-x",
			},
			report: map[string]string{
				"2C:54:91:88:C9:E3": "x-mac-000001-x",
				"2C:56:83:91:C9:E6": "x-mac-000002-x",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeConsistent)
			require.NoError(t, err)
			for i := 0; i < len(tc.input); i++ {
				assert.Equal(t, tc.output[i], o.Contents(tc.input[i]))
			}
			assert.Equal(t, tc.report, o.Report())
		})
	}
}
