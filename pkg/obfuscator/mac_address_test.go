package obfuscator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMacStaticReplacement(t *testing.T) {
	o, _ := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeStatic, NewSimpleTracker())
	assert.Equal(t, staticMacReplacement, o.Contents("29-7E-8C-8C-60-C9"))
	assert.Equal(t, map[string]string{"29-7E-8C-8C-60-C9": staticMacReplacement}, o.Report().AsMap())
}

func TestMacConsistentReplacement(t *testing.T) {
	o, _ := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	assert.Equal(t, "x-mac-0000000001-x", o.Contents("29-7E-8C-8C-60-C9"))
	// This testcase reports both the original detected MAC address as well as the normalized MAC address
	assert.Equal(t, map[string]string{"29-7E-8C-8C-60-C9": "x-mac-0000000001-x"}, o.Report().AsMap())
}

func TestMACObfuscatorReplacesPercentEncodedMAC(t *testing.T) {
	obfuscator, err := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	output := obfuscator.Contents("mac=52%3Adf%3A20%3A08%3A6c%3Aff")

	assert.Equal(t, "mac=x-mac-0000000001-x", output)
	assert.Equal(t, map[string]string{
		"52%3Adf%3A20%3A08%3A6c%3Aff": "x-mac-0000000001-x",
	}, obfuscator.Report().AsMap())
}

func TestMACObfuscatorRawAndEncodedFormsShareReplacement(t *testing.T) {
	obfuscator, err := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	forms := []string{
		"52:df:20:08:6c:ff",
		"52-df-20-08-6c-ff",
		"52%3Adf%3A20%3A08%3A6c%3Aff",
		"%35%32:df:20:08:6c:ff",
	}
	output := obfuscator.Contents(strings.Join(forms, " | "))
	reports := obfuscator.Report().Replacements

	require.Len(t, reports, 1)
	assert.Equal(t, "52:DF:20:08:6C:FF", reports[0].Canonical)
	assert.Equal(t, len(forms), strings.Count(output, reports[0].ReplacedWith))
	for _, form := range forms {
		assert.Equal(t, uint(1), reports[0].Counter[form])
	}
}

func TestMACObfuscatorDoesNotStartInsidePercentEncoding(t *testing.T) {
	obfuscator, err := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	input := "%252:df:20:08:6c:ff"
	assert.Equal(t, input, obfuscator.Contents(input))
	assert.Empty(t, obfuscator.Report().Replacements)
}

func TestMacReplacementManyMatchLine(t *testing.T) {
	input := "ss eb:a1:2a:b2:09:bf as 29-7E-8C-8C-60-C9 with some stuff around it and lowercased eb-a1-2a-b2-09-bf"
	expected := "ss xx:xx:xx:xx:xx:xx as xx:xx:xx:xx:xx:xx with some stuff around it and lowercased xx:xx:xx:xx:xx:xx"
	o, _ := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeStatic, NewSimpleTracker())
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
		{name: "mixed separators", input: "52-df:20-08:6c-ff", expectedOutput: staticMacReplacement},
		{name: "seven groups", input: "00:11:22:33:44:55:66", expectedOutput: staticMacReplacement + ":66"},
		{name: "overlong run", input: "00:11:22:33:44:55:66:77:88:99:aa:bb", expectedOutput: staticMacReplacement + ":" + staticMacReplacement},
		{name: "multi-colon", input: "eb:a1:2a:b2:09:bf eb:a1:2a:b2:09:bf", expectedOutput: staticMacReplacement + " " + staticMacReplacement},
		{name: "multi-colon-dash", input: "16-7C-44-26-24-14 BF:51:A4:1B:7D:0B", expectedOutput: staticMacReplacement + " " + staticMacReplacement},
		{name: "mac surrounded", input: "mac 52:df:20:08:6c:ff caused some trouble", expectedOutput: fmt.Sprintf("mac %s caused some trouble", staticMacReplacement)},
		{name: "mac as guid", input: "4a5299ac-6104-479d-aed4-b79faedffcb4", expectedOutput: "4a5299ac-6104-479d-aed4-b79faedffcb4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, _ := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeStatic, NewSimpleTracker())
			assert.Equal(t, tc.expectedOutput, o.Contents(tc.input))
		})
	}
}
func TestMACObfuscatorWithCount(t *testing.T) {
	for _, tc := range []struct {
		name           string
		input          string
		expectedOutput string
		report         ReplacementReport
	}{
		{
			name:           "6 MACs each exactly once",
			input:          "mac bf-51-a4-1b-7d-0b 16-7C-44-26-24-14 BF:51:A4:1B:7D:0B 16:7C:44:26:24:14 BF-51-A4-1B-7D-0B bf:51:a4:1b:7d:0b",
			expectedOutput: fmt.Sprintf("mac %s %s %s %s %s %s", staticMacReplacement, staticMacReplacement, staticMacReplacement, staticMacReplacement, staticMacReplacement, staticMacReplacement),
			report: ReplacementReport{
				[]Replacement{
					{Canonical: "16:7C:44:26:24:14", ReplacedWith: staticMacReplacement,
						Counter: map[string]uint{
							"16:7C:44:26:24:14": 1,
							"16-7C-44-26-24-14": 1,
						}},
					{Canonical: "BF:51:A4:1B:7D:0B", ReplacedWith: staticMacReplacement,
						Counter: map[string]uint{
							"BF:51:A4:1B:7D:0B": 1,
							"BF-51-A4-1B-7D-0B": 1,
							"bf:51:a4:1b:7d:0b": 1,
							"bf-51-a4-1b-7d-0b": 1,
						}},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeStatic, NewSimpleTracker())
			require.NoError(t, err)
			assert.Equal(t, tc.expectedOutput, o.Contents(tc.input))
			replacementReportsMatch(t, tc.report, o.Report())
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
			o, err := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)
			for i := 0; i < len(tc.input); i++ {
				assert.Equal(t, tc.output[i], o.Contents(tc.input[i]))
			}
			assert.Equal(t, tc.report, o.Report().AsMap())
		})
	}
}

func BenchmarkMACObfuscatorNoMatches(b *testing.B) {
	input := strings.Repeat("this line contains hexadecimal words but no MAC address\n", 10000)
	obfuscator, err := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
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

func BenchmarkMACObfuscatorManyMatches(b *testing.B) {
	input := strings.Repeat("52:df:20:08:6c:ff ", 10000)
	obfuscator, err := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
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

func BenchmarkMACObfuscatorUUIDHeavy(b *testing.B) {
	input := strings.Repeat("4a5299ac-6104-479d-aed4-b79faedffcb4 ", 10000)
	obfuscator, err := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
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
