package obfuscator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMacReplacementHappyPath(t *testing.T) {
	o := NewMacAddressObfuscator()
	assert.Equal(t, StaticMacReplacement, o.Contents("29-7E-8C-8C-60-C9"))
	assert.Equal(t, map[string]string{"29-7E-8C-8C-60-C9": StaticMacReplacement}, o.Report())
}

func TestMacReplacementManyMatchLine(t *testing.T) {
	input := "ss eb:a1:2a:b2:09:bf as 29-7E-8C-8C-60-C9 with some stuff around it and lowecased eb-a1-2a-b2-09-bf"
	expected := "ss xx:xx:xx:xx:xx:xx as xx:xx:xx:xx:xx:xx with some stuff around it and lowecased xx:xx:xx:xx:xx:xx"
	o := NewMacAddressObfuscator()
	assert.Equal(t, expected, o.Contents(input))
	assert.Equal(t, map[string]string{
		"eb:a1:2a:b2:09:bf": StaticMacReplacement,
		"29-7E-8C-8C-60-C9": StaticMacReplacement,
		"eb-a1-2a-b2-09-bf": StaticMacReplacement,
	}, o.Report())
}

func TestMacReplacementSuper(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		input                string
		expectedOutput       string
		expectedReportOutput map[string]string
	}{
		{name: "uppercase-colon", input: "69:80:6F:E6:7C:05", expectedOutput: StaticMacReplacement},
		{name: "lowercase-dash", input: "eb-a1-2a-b2-09-bf", expectedOutput: StaticMacReplacement},
		{name: "lowercase-colon", input: "eb:a1:2a:b2:09:bf", expectedOutput: StaticMacReplacement},
		{name: "multi-colon", input: "eb:a1:2a:b2:09:bf eb:a1:2a:b2:09:bf", expectedOutput: StaticMacReplacement + " " + StaticMacReplacement},
		{name: "multi-colon-dash", input: "16-7C-44-26-24-14 BF:51:A4:1B:7D:0B", expectedOutput: StaticMacReplacement + " " + StaticMacReplacement},
		{name: "mac surrounded", input: "mac 52:df:20:08:6c:ff caused some trouble", expectedOutput: fmt.Sprintf("mac %s caused some trouble", StaticMacReplacement)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := NewMacAddressObfuscator()
			assert.Equal(t, tc.expectedOutput, o.Contents(tc.input))
		})
	}
}
