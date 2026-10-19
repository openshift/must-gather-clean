//go:build e2e
// +build e2e

package cli

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/reporting"
	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	configOpenshiftDefaultPath = "examples/openshift_default.yaml"
)

// End-to-end test suite that runs must-gather-clean on --input and compares
// the generated report with a known source-of-truth report --report
// See openshift/must-gather-clean/test/files/README.md for more info
func TestEndToEnd(t *testing.T) {
	var input, report string
	fs := flag.NewFlagSet("e2e-fs", flag.ContinueOnError)
	fs.StringVar(&input, "input", "", "")
	fs.StringVar(&report, "report", "", "")
	if err := fs.Parse(flag.Args()); err != nil {
		t.Fatal(err)
	}
	if input == "" || report == "" {
		t.Fatal("Expected arguments --input && --report")
	}

	// find the path to the project root of this file
	_, filename, _, _ := runtime.Caller(0)
	rootDir := path.Join(path.Dir(filename), "..", "..")
	// relative paths -> absolute paths
	inputDir := path.Join(rootDir, input)
	outputDir := path.Join(rootDir, fmt.Sprintf("%s.cleaned", input))
	configPath := path.Join(rootDir, configOpenshiftDefaultPath)
	generatedReportDir := path.Join(rootDir, fmt.Sprintf("%s-report", input))
	reportPath := path.Join(rootDir, report)

	err := Run(configPath,
		inputDir,
		outputDir,
		true,
		generatedReportDir,
		1)
	require.NoError(t, err)

	// read reports
	truthReport := readReport(t, reportPath)
	generatedReport := readReport(t, filepath.Join(generatedReportDir, reportFileName))
	removeRelativePath(generatedReport, inputDir)
	// compare reports
	verifyReport(t, inputDir, truthReport, generatedReport)
	verifyObfuscation(t, outputDir, generatedReport)
}

func TestEndToEndParallelWorkers(t *testing.T) {
	var input, report string
	fs := flag.NewFlagSet("e2e-fs", flag.ContinueOnError)
	fs.StringVar(&input, "input", "", "")
	fs.StringVar(&report, "report", "", "")
	if err := fs.Parse(flag.Args()); err != nil {
		t.Fatal(err)
	}
	if input == "" || report == "" {
		t.Fatal("Expected arguments --input && --report")
	}

	_, filename, _, _ := runtime.Caller(0)
	rootDir := path.Join(path.Dir(filename), "..", "..")
	inputDir := path.Join(rootDir, input)
	tempDir := t.TempDir()
	outputDir := path.Join(tempDir, "cleaned")
	configPath := path.Join(rootDir, configOpenshiftDefaultPath)
	generatedReportDir := path.Join(tempDir, "report")
	reportPath := path.Join(rootDir, report)
	workerCount := runtime.NumCPU()
	if workerCount < 2 {
		workerCount = 2
	}

	err := Run(configPath,
		inputDir,
		outputDir,
		true,
		generatedReportDir,
		workerCount)
	require.NoError(t, err)

	truthReport := readReport(t, reportPath)
	generatedReport := readReport(t, filepath.Join(generatedReportDir, reportFileName))
	removeRelativePath(generatedReport, inputDir)

	verifyParallelReport(t, truthReport, generatedReport)
	verifyObfuscation(t, outputDir, generatedReport)
}

func removeRelativePath(r *reporting.Report, path string) {
	for i := range r.Omissions {
		r.Omissions[i] = strings.TrimPrefix(r.Omissions[i], path)
	}
}

func verifyObfuscation(t *testing.T, dir string, report *reporting.Report) {
	// report should already be verified by verifyReport before to ensure it does contain correct information
	generatedMap := map[string]string{}
	for i, replacementGroup := range report.Replacements {
		azureResources := i < len(report.Config.Obfuscate) && report.Config.Obfuscate[i].Type == schema.ObfuscateTypeAzureResources
		for _, replacement := range replacementGroup {
			for _, occurrence := range replacement.Occurrences {
				// Azure resource names shorter than five characters are intentionally skipped.
				if azureResources && len(occurrence.Original) < 5 {
					continue
				}
				generatedMap[occurrence.Original] = replacement.ReplacedWith
			}
		}
	}
	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		for k, v := range generatedMap {
			assert.NotContainsf(t, path, k, "path should not contain secret info %s, but rather its replacement %s", k, v)
		}

		if !info.IsDir() {
			file, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			content := string(file)
			for k, v := range generatedMap {
				assert.NotContainsf(t, content, k, "file '%s' should not contain secret info %s, but rather its replacement %s", path, k, v)
			}
		}

		return nil
	})

	require.NoError(t, err)
}

func verifyReport(t *testing.T, inputDir string, truthReport *reporting.Report, generatedReport *reporting.Report) {
	verifyReplacements(t, inputDir, truthReport, generatedReport)
	verifyConfig(t, generatedReport)
	verifyOmissions(t, inputDir, truthReport, generatedReport)
}

func verifyReplacements(t *testing.T, inputDir string, truthReport *reporting.Report, generatedReport *reporting.Report) {
	tr := reportInternalRepresentation(truthReport)
	gr := reportInternalRepresentation(generatedReport)
	require.Equal(t, len(tr), len(gr))
	require.Equal(t, len(tr), len(truthReport.Config.Obfuscate))
	require.Equal(t, len(gr), len(generatedReport.Config.Obfuscate))

	for i := range tr {
		replacementReportsMatch(t, tr[i], gr[i])
	}
}

func verifyOmissions(t *testing.T, inputDir string, truthReport *reporting.Report, generatedReport *reporting.Report) {
	sort.Strings(truthReport.Omissions)
	sort.Strings(generatedReport.Omissions)
	assert.Equal(t, truthReport.Omissions, generatedReport.Omissions)
}

func verifyParallelReport(t *testing.T, truthReport *reporting.Report, generatedReport *reporting.Report) {
	require.Equal(t, len(truthReport.Replacements), len(generatedReport.Replacements))
	require.Equal(t, len(truthReport.Config.Obfuscate), len(generatedReport.Replacements))
	require.Equal(t, len(generatedReport.Config.Obfuscate), len(generatedReport.Replacements))

	truthReplacements := reportInternalRepresentation(truthReport)
	generatedReplacements := reportInternalRepresentation(generatedReport)
	for groupIndex := range generatedReport.Replacements {
		require.Equalf(
			t,
			truthReport.Config.Obfuscate[groupIndex].Type,
			generatedReport.Config.Obfuscate[groupIndex].Type,
			"replacement group type mismatch in group %d",
			groupIndex+1,
		)
		compareCounts := generatedReport.Config.Obfuscate[groupIndex].Type != schema.ObfuscateTypeAzureResources
		parallelReplacementReportsMatch(t, truthReplacements[groupIndex], generatedReplacements[groupIndex], compareCounts)
		verifyParallelConfig(t, generatedReport, groupIndex)
	}

	verifyOmissions(t, "", truthReport, generatedReport)
}

func verifyParallelConfig(t *testing.T, report *reporting.Report, groupIndex int) {
	replacementsByOriginal := make(map[string]string)
	for _, replacement := range report.Replacements[groupIndex] {
		for _, occurrence := range replacement.Occurrences {
			if existingReplacement, ok := replacementsByOriginal[occurrence.Original]; ok {
				require.Equalf(t, existingReplacement, replacement.ReplacedWith, "original %s maps to multiple replacements in group %d", occurrence.Original, groupIndex+1)
			}
			replacementsByOriginal[occurrence.Original] = replacement.ReplacedWith
		}
	}
	expectedConfig := make(schema.ObfuscateReplacement, len(replacementsByOriginal))
	for original, replacement := range replacementsByOriginal {
		expectedConfig[original] = replacement
	}
	actualConfig := make(schema.ObfuscateReplacement, len(report.Config.Obfuscate[groupIndex].Replacement))
	for original, replacement := range report.Config.Obfuscate[groupIndex].Replacement {
		actualConfig[original] = replacement
	}
	assert.Equalf(t, expectedConfig, actualConfig, "replacement config mismatch in group %d", groupIndex+1)
}

func parallelReplacementReportsMatch(t *testing.T, want, got []obfuscator.Replacement, compareCounts bool) {
	require.Equal(t, len(want), len(got))
	sort.Slice(want, func(i, j int) bool {
		return want[i].Canonical > want[j].Canonical
	})
	sort.Slice(got, func(i, j int) bool {
		return got[i].Canonical > got[j].Canonical
	})
	for i := range got {
		assert.Equal(t, want[i].Canonical, got[i].Canonical)
		if compareCounts {
			assert.Equal(t, want[i].Counter, got[i].Counter)
			continue
		}
		assert.Equal(t, replacementCounterKeys(want[i].Counter), replacementCounterKeys(got[i].Counter))
	}
}

func replacementCounterKeys(counter map[string]uint) []string {
	keys := make([]string, 0, len(counter))
	for key := range counter {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func reportInternalRepresentation(report *reporting.Report) [][]obfuscator.Replacement {
	repls := make([][]obfuscator.Replacement, len(report.Replacements))
	for i, replacementGroup := range report.Replacements {
		for _, r := range replacementGroup {
			count := map[string]uint{}
			for _, c := range r.Occurrences {
				count[c.Original] = c.Count
			}
			repls[i] = append(repls[i], obfuscator.Replacement{
				Canonical:    r.Canonical,
				ReplacedWith: r.ReplacedWith,
				Counter:      count,
			})
		}
	}
	return repls
}

func replacementReportsMatch(t *testing.T, want, got []obfuscator.Replacement) {
	require.Equal(t, len(want), len(got))
	sort.Slice(want, func(i, j int) bool {
		return want[i].Canonical > want[j].Canonical
	})
	sort.Slice(got, func(i, j int) bool {
		return got[i].Canonical > got[j].Canonical
	})
	for i := range got {
		w := want[i]
		g := got[i]
		assert.Equal(t, w.Canonical, g.Canonical)
		assert.Equal(t, w.ReplacedWith, g.ReplacedWith)
		assert.Equal(t, w.Counter, g.Counter)
	}
}

func verifyConfig(t *testing.T, report *reporting.Report) {
	replacementsMap := make(map[string]string)
	for _, obfuscator := range report.Replacements {
		for _, replacement := range obfuscator {
			for _, o := range replacement.Occurrences {
				replacementsMap[o.Original] = replacement.ReplacedWith
			}
		}
	}
	configMap := make(map[string]string)
	for _, obfuscator := range report.Config.Obfuscate {
		for k, v := range obfuscator.Replacement {
			configMap[k] = v
		}
	}
	assert.Equal(t, replacementsMap, configMap)
}

func readReport(t *testing.T, path string) *reporting.Report {
	bytes, err := os.ReadFile(path)
	require.NoError(t, err)
	report := &reporting.Report{}
	require.NoError(t, yaml.Unmarshal(bytes, report))
	return report
}
