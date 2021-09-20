package obfuscator

import (
	"fmt"
	"sort"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMacStaticReplacement(t *testing.T) {
	o, _ := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeStatic)
	assert.Equal(t, staticMacReplacement, o.Contents("29-7E-8C-8C-60-C9"))
	assert.Equal(t, map[string]string{"29-7E-8C-8C-60-C9": staticMacReplacement}, o.Report().AsMap())
}

func TestMacConsistentReplacement(t *testing.T) {
	o, _ := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeConsistent)
	assert.Equal(t, "x-mac-0000000001-x", o.Contents("29-7E-8C-8C-60-C9"))
	// This testcase reports both the original detected MAC address as well as the normalized MAC address
	assert.Equal(t, map[string]string{"29-7E-8C-8C-60-C9": "x-mac-0000000001-x"}, o.Report().AsMap())
}

func TestMacReplacementManyMatchLine(t *testing.T) {
	input := "ss eb:a1:2a:b2:09:bf as 29-7E-8C-8C-60-C9 with some stuff around it and lowercased eb-a1-2a-b2-09-bf"
	expected := "ss xx:xx:xx:xx:xx:xx as xx:xx:xx:xx:xx:xx with some stuff around it and lowercased xx:xx:xx:xx:xx:xx"
	o, _ := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeStatic)
	assert.Equal(t, expected, o.Contents(input))
	assert.Equal(t, map[string]string{
		"eb:a1:2a:b2:09:bf": staticMacReplacement,
		"eb-a1-2a-b2-09-bf": staticMacReplacement,
		"29-7E-8C-8C-60-C9": staticMacReplacement,
	}, o.Report().AsMap())
}

func TestMacReplacementStatic(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		input                string
		expectedOutput       string
		expectedReportOutput map[string]string
	}{
		{name: "squashed", input: "69806FE67C05", expectedOutput: "69806FE67C05"},
		{name: "squashed:lowercase", input: "69806fe67c05", expectedOutput: "69806fe67c05"},
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
func TestMACObfuscatorWithCount(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		input                string
		expectedOutput       string
		expectedReportOutput ReplacementReport
	}{
		{
			name:           "6 MACs each exactly once",
			input:          "mac bf-51-a4-1b-7d-0b 16-7C-44-26-24-14 BF:51:A4:1B:7D:0B 16:7C:44:26:24:14 BF-51-A4-1B-7D-0B bf:51:a4:1b:7d:0b",
			expectedOutput: fmt.Sprintf("mac %s %s %s %s %s %s", staticMacReplacement, staticMacReplacement, staticMacReplacement, staticMacReplacement, staticMacReplacement, staticMacReplacement),
			expectedReportOutput: ReplacementReport{
				[]Replacement{
					{Canonical: "16:7C:44:26:24:14", ReplacedWith: staticMacReplacement, Occurrences: []Occurrence{
						{Original: "16:7C:44:26:24:14", Count: 1},
						{Original: "16-7C-44-26-24-14", Count: 1},
					}},
					{Canonical: "BF:51:A4:1B:7D:0B", ReplacedWith: staticMacReplacement, Occurrences: []Occurrence{
						{Original: "BF:51:A4:1B:7D:0B", Count: 1},
						{Original: "BF-51-A4-1B-7D-0B", Count: 1},
						{Original: "bf:51:a4:1b:7d:0b", Count: 1},
						{Original: "bf-51-a4-1b-7d-0b", Count: 1},
					}},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeStatic)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedOutput, o.Contents(tc.input))
			report := o.Report()
			assert.Equal(t, len(tc.expectedReportOutput.Replacements), len(report.Replacements))
			sort.Slice(tc.expectedReportOutput.Replacements, func(i, j int) bool {
				return tc.expectedReportOutput.Replacements[i].Canonical > tc.expectedReportOutput.Replacements[j].Canonical
			})
			sort.Slice(report.Replacements, func(i, j int) bool {
				return report.Replacements[i].Canonical > report.Replacements[j].Canonical
			})
			for i := range report.Replacements {
				want := tc.expectedReportOutput.Replacements[i]
				got := report.Replacements[i]
				assert.Equal(t, want.Canonical, got.Canonical)
				assert.Equal(t, want.ReplacedWith, got.ReplacedWith)
				assert.ElementsMatch(t, want.Occurrences, got.Occurrences)
			}
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
			output: []string{"received request from x-mac-0000000001-x"},
			report: map[string]string{"29-7E-8C-8C-60-C9": "x-mac-0000000001-x"},
		},
		{
			name:   "MAC address mentioned in a sentence",
			input:  []string{"A MAC address of 2c549188c9e3 is typically displayed as 2C:54:91:88:C9:E3"},
			output: []string{"A MAC address of 2c549188c9e3 is typically displayed as x-mac-0000000001-x"},
			report: map[string]string{"2C:54:91:88:C9:E3": "x-mac-0000000001-x"},
		},
		{
			name:   "Same MAC address in different notations",
			input:  []string{"A MAC address 2C:54:91:88:C9:E3 can be displayed as 2C-54-91-88-C9-E3 in a filename"},
			output: []string{"A MAC address x-mac-0000000001-x can be displayed as x-mac-0000000001-x in a filename"},
			report: map[string]string{"2C:54:91:88:C9:E3": "x-mac-0000000001-x", "2C-54-91-88-C9-E3": "x-mac-0000000001-x"},
		},
		{
			name:   "MAC Address mentioned as case sensitive strings",
			input:  []string{"A MAC address 2C:54:91:88:C9:E3 can also be displayed as 2c:54:91:88:c9:e3"},
			output: []string{"A MAC address x-mac-0000000001-x can also be displayed as x-mac-0000000001-x"},
			report: map[string]string{"2C:54:91:88:C9:E3": "x-mac-0000000001-x", "2c:54:91:88:c9:e3": "x-mac-0000000001-x"},
		},
		{
			name:   "Multiple MAC addresses",
			input:  []string{"MAC addresses of the two network interfaces are 2C:54:91:88:C9:E3 and 2C:56:83:91:C9:E6"},
			output: []string{"MAC addresses of the two network interfaces are x-mac-0000000001-x and x-mac-0000000002-x"},
			report: map[string]string{
				"2C:54:91:88:C9:E3": "x-mac-0000000001-x",
				"2C:56:83:91:C9:E6": "x-mac-0000000002-x",
			},
		},
		{
			name: "Multiple invocations",
			input: []string{
				"MAC addresses of the two network interfaces are 2C:54:91:88:C9:E3 and 2C:56:83:91:C9:E6",
				"MAC addresses of the two network interfaces are 2C:54:91:88:C9:E3 and 2C:56:83:91:C9:E6",
			},
			output: []string{
				"MAC addresses of the two network interfaces are x-mac-0000000001-x and x-mac-0000000002-x",
				"MAC addresses of the two network interfaces are x-mac-0000000001-x and x-mac-0000000002-x",
			},
			report: map[string]string{
				"2C:54:91:88:C9:E3": "x-mac-0000000001-x",
				"2C:56:83:91:C9:E6": "x-mac-0000000002-x",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeConsistent)
			require.NoError(t, err)
			for i := 0; i < len(tc.input); i++ {
				assert.Equal(t, tc.output[i], o.Contents(tc.input[i]))
			}
			assert.Equal(t, tc.report, o.Report().AsMap())
		})
	}
}
