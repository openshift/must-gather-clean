package reporting

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestReportingHappyPath(t *testing.T) {
	pattern := "random-pattern"
	config := &schema.SchemaJson{
		Config: schema.SchemaJsonConfig{
			Obfuscate: []schema.Obfuscate{
				{
					DomainNames: []string{"sample-domain.com", "example-domain.com"},
					Target:      schema.ObfuscateTargetAll,
					Type:        schema.ObfuscateTypeDomain,
				},
				{Type: schema.ObfuscateTypeKeywords},
			},
			Omit: []schema.Omit{
				{
					Type:    schema.OmitTypeFile,
					Pattern: &pattern,
				},
			},
		},
	}
	r := NewSimpleReporter(config)
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
		Config:    config.Config,
	})
}

func assertReportMatches(t *testing.T, file string, expectedReport Report) {
	bytes, err := os.ReadFile(file)
	require.NoError(t, err)

	actualReport := &Report{}
	err = yaml.Unmarshal(bytes, actualReport)
	require.NoError(t, err)

	assert.Equal(t, expectedReport.Omissions, actualReport.Omissions)
	assert.Equal(t, expectedReport.Replacements, actualReport.Replacements)
	assert.Equal(t, expectedReport.Config, actualReport.Config)
}

func TestReporterSortsReplacementsAndOccurrences(t *testing.T) {
	config := &schema.SchemaJson{
		Config: schema.SchemaJsonConfig{
			Obfuscate: []schema.Obfuscate{{Type: schema.ObfuscateTypeDomain}},
		},
	}
	reporter := NewSimpleReporter(config)
	reporter.CollectObfuscatorReport([]obfuscator.ReplacementReport{{
		Replacements: []obfuscator.Replacement{
			{Canonical: "z.example", ReplacedWith: "z-replacement", Counter: map[string]uint{"z-original": 1, "a-original": 2}},
			{Canonical: "a.example", ReplacedWith: "a-replacement", Counter: map[string]uint{"z-original": 1, "a-original": 1}},
		},
	}})

	assert.Equal(t, [][]Replacement{{
		{Canonical: "a.example", ReplacedWith: "a-replacement", Occurrences: []Occurrence{{Original: "a-original", Count: 1}, {Original: "z-original", Count: 1}}},
		{Canonical: "z.example", ReplacedWith: "z-replacement", Occurrences: []Occurrence{{Original: "a-original", Count: 2}, {Original: "z-original", Count: 1}}},
	}}, reporter.(*SimpleReporter).replacements)
}

func TestReporterSortsOmissions(t *testing.T) {
	config := &schema.SchemaJson{Config: schema.SchemaJsonConfig{}}
	reporter := NewSimpleReporter(config)
	reporter.CollectOmitterReport([]string{"z-path", "a-path"})

	assert.Equal(t, []string{"a-path", "z-path"}, reporter.(*SimpleReporter).omissions)
}

func TestReporterRejectsMismatchedReplacementGroups(t *testing.T) {
	config := &schema.SchemaJson{Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{
		{Type: schema.ObfuscateTypeIP},
		{Type: schema.ObfuscateTypeMAC},
	}}}
	reporter := NewSimpleReporter(config)
	reporter.CollectObfuscatorReport([]obfuscator.ReplacementReport{{}})

	reportPath := filepath.Join(t.TempDir(), "report.yaml")
	err := reporter.WriteReport(reportPath)
	require.ErrorContains(t, err, "1 replacement groups for 2 configured obfuscators")
	assert.NoFileExists(t, reportPath)
}

func TestReporterRejectsMissingConfiguration(t *testing.T) {
	reporter := NewSimpleReporter(nil)
	reporter.CollectObfuscatorReport(nil)

	err := reporter.WriteReport(filepath.Join(t.TempDir(), "report.yaml"))
	require.ErrorContains(t, err, "without a configuration")
}

func TestReporterUsesUpstreamFileAndDirectoryPermissions(t *testing.T) {
	root := t.TempDir()
	controlFile, err := os.OpenFile(filepath.Join(root, "control-report.yaml"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	require.NoError(t, err)
	require.NoError(t, controlFile.Close())
	controlFileInfo, err := os.Stat(filepath.Join(root, "control-report.yaml"))
	require.NoError(t, err)

	controlDirPath := filepath.Join(root, "control-report-dir")
	require.NoError(t, os.Mkdir(controlDirPath, 0700))
	controlDirInfo, err := os.Stat(controlDirPath)
	require.NoError(t, err)

	reportDir := filepath.Join(root, "report-dir")
	reportPath := filepath.Join(reportDir, "report.yaml")
	reporter := NewSimpleReporter(&schema.SchemaJson{})
	require.NoError(t, reporter.WriteReport(reportPath))

	reportInfo, err := os.Stat(reportPath)
	require.NoError(t, err)
	assert.Equal(t, controlFileInfo.Mode().Perm(), reportInfo.Mode().Perm())
	reportDirInfo, err := os.Stat(reportDir)
	require.NoError(t, err)
	assert.Equal(t, controlDirInfo.Mode().Perm(), reportDirInfo.Mode().Perm())
}

func TestReporterPreservesExistingFilePermissions(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), "report.yaml")
	require.NoError(t, os.WriteFile(reportPath, []byte("old report"), 0640))
	require.NoError(t, os.Chmod(reportPath, 0640))

	reporter := NewSimpleReporter(&schema.SchemaJson{})
	require.NoError(t, reporter.WriteReport(reportPath))

	info, err := os.Stat(reportPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0640), info.Mode().Perm())
}

func TestReporterWritesThroughReportSymlink(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target-report.yaml")
	reportPath := filepath.Join(dir, "report.yaml")
	require.NoError(t, os.WriteFile(targetPath, []byte("old report"), 0600))
	require.NoError(t, os.Symlink(filepath.Base(targetPath), reportPath))

	reporter := NewSimpleReporter(&schema.SchemaJson{})
	require.NoError(t, reporter.WriteReport(reportPath))

	linkInfo, err := os.Lstat(reportPath)
	require.NoError(t, err)
	assert.NotZero(t, linkInfo.Mode()&os.ModeSymlink)
	contents, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	assert.NotContains(t, string(contents), "old report")
}
