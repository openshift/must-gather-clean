package obfuscator

import (
	"testing"

	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/assert"
)

func TestExactReplacementObfuscatorContents(t *testing.T) {
	for _, tc := range []struct {
		name              string
		exactReplacements []schema.ObfuscateExactReplacementsElem
		input             []string
		output            []string
		report            ReplacementReport
	}{
		{
			name: "basic",
			exactReplacements: []schema.ObfuscateExactReplacementsElem{
				{Original: "simple", Replacement: "XXX"},
				{Original: ".*", Replacement: "ZZZ"},
				{Original: "*", Replacement: "YYY"},
			},
			input: []string{
				"starting simple and clean",
				"catching fake regexes with * and .*",
			},
			output: []string{
				"starting XXX and clean",
				"catching fake regexes with YYY and ZZZ",
			},
			report: ReplacementReport{[]Replacement{}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := NewExactReplacementObfuscator(tc.exactReplacements, NewSimpleTracker())
			for idx, i := range tc.input {
				output := o.Contents(i)
				assert.Equal(t, tc.output[idx], output)
			}
			replacementReportsMatch(t, tc.report, o.Report())
		})
	}
}
