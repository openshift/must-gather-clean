package cli

import (
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/reporting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	configOpenshiftDefaultPath = "examples/openshift_default.yaml"
)

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
		runtime.NumCPU())
	require.NoError(t, err)

	// read reports
	truthReport := readReport(t, reportPath)
	generatedReport := readReport(t, filepath.Join(generatedReportDir, reportFileName))
	removeRelativePath(generatedReport, inputDir)
	// compare reports
	verifyReport(t, inputDir, truthReport, generatedReport)
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
	for _, obfuscator := range report.Replacements {
		for _, replacement := range obfuscator {
			for _, occurrence := range replacement.Occurrences {
				generatedMap[occurrence.Original] = replacement.ReplacedWith
			}
		}
	}
	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		for k, v := range generatedMap {
			assert.NotContainsf(t, path, k, "path should not contain secret IP %s, but rather its replacement %s", k, v)
		}

		if !info.IsDir() {
			file, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			content := string(file)
			for k, v := range generatedMap {
				assert.NotContainsf(t, content, k, "file '%s' should not contain secret IP %s, but rather its replacement %s", path, k, v)
			}
		}

		return nil
	})

	require.NoError(t, err)
}

func verifyReport(t *testing.T, inputDir string, truthReport *reporting.Report, generatedReport *reporting.Report) {
	verifyReplacements(t, inputDir, truthReport, generatedReport)
	verifyOmissions(t, inputDir, truthReport, generatedReport)
}

func verifyReplacements(t *testing.T, inputDir string, truthReport *reporting.Report, generatedReport *reporting.Report) {
	tr := reportInternalRepresentation(truthReport)
	gr := reportInternalRepresentation(generatedReport)
	replacementReportsMatch(t, tr, gr)
}

func verifyOmissions(t *testing.T, inputDir string, truthReport *reporting.Report, generatedReport *reporting.Report) {
	sort.Strings(truthReport.Omissions)
	sort.Strings(generatedReport.Omissions)
	assert.Equal(t, truthReport.Omissions, generatedReport.Omissions)
}

func reportInternalRepresentation(report *reporting.Report) obfuscator.ReplacementReport {
	var repls []obfuscator.Replacement
	for _, o := range report.Replacements {
		for _, r := range o {
			count := map[string]uint{}
			for _, c := range r.Occurrences {
				count[c.Original] = c.Count
			}
			repls = append(repls, obfuscator.Replacement{
				Canonical:    r.Canonical,
				ReplacedWith: r.ReplacedWith,
				Counter:      count,
			})
		}
	}
	return obfuscator.ReplacementReport{Replacements: repls}
}

func replacementReportsMatch(t *testing.T, want, got obfuscator.ReplacementReport) {
	assert.Equal(t, len(want.Replacements), len(got.Replacements))
	sort.Slice(want.Replacements, func(i, j int) bool {
		return want.Replacements[i].Canonical > want.Replacements[j].Canonical
	})
	sort.Slice(got.Replacements, func(i, j int) bool {
		return got.Replacements[i].Canonical > got.Replacements[j].Canonical
	})
	for i := range got.Replacements {
		w := want.Replacements[i]
		g := got.Replacements[i]
		assert.Equal(t, w.Canonical, g.Canonical)
		assert.Equal(t, w.Counter, g.Counter)
	}
}

func readReport(t *testing.T, path string) *reporting.Report {
	bytes, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	report := &reporting.Report{}
	require.NoError(t, yaml.Unmarshal(bytes, report))
	return report
}
