package reporting

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestReportingHappyPath(t *testing.T) {
	r := NewSimpleReporter()
	r.CollectOmitterReport([]string{"some path"})
	multiObfuscator := obfuscator.NewMultiObfuscator([]obfuscator.ReportingObfuscator{
		obfuscator.NoopObfuscator{Replacements: map[string]string{
			"this": "that",
		}},
		obfuscator.NoopObfuscator{Replacements: map[string]string{
			"another": "something",
		}},
	})
	r.CollectObfuscatorReport(multiObfuscator.ReportPerObfuscator())

	tmpInputDir, err := os.MkdirTemp("", "reporter-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tmpInputDir)
	}()

	reportFile := filepath.Join(tmpInputDir, "report.yaml")
	err = r.WriteReport(reportFile)
	require.NoError(t, err)

	assertReportMatches(t, reportFile, Report{
		Replacements: [][]Replacement{
			{Replacement{Canonical: "this", ReplacedWith: "that", Occurrences: []Occurrence{{Original: "this", Count: 1}}}},
			{Replacement{Canonical: "another", ReplacedWith: "something", Occurrences: []Occurrence{{Original: "another", Count: 1}}}},
		},
		Omissions: []string{"some path"},
	})
}

func assertReportMatches(t *testing.T, file string, expectedReport Report) {
	bytes, err := ioutil.ReadFile(file)
	require.NoError(t, err)

	actualReport := &Report{}
	err = yaml.Unmarshal(bytes, actualReport)
	require.NoError(t, err)

	assert.Equal(t, expectedReport.Omissions, actualReport.Omissions)
	assert.Equal(t, expectedReport.Replacements, actualReport.Replacements)
}
