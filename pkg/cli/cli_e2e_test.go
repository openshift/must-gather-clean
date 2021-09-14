package cli

import (
	"io/fs"
	"io/ioutil"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/reporting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestEndToEndAWSWithExample(t *testing.T) {
	// find the path to the project root of this file
	_, filename, _, _ := runtime.Caller(0)
	rootDir := path.Join(path.Dir(filename), "..", "..")
	inputDir := filepath.Join(rootDir, "pkg/cli/testfiles/aws/")
	outputDir := filepath.Join(rootDir, "pkg/cli/testfiles/aws.cleaned/")

	err := Run(filepath.Join(rootDir, "examples/openshift_default.yaml"),
		inputDir,
		outputDir,
		true,
		rootDir,
		runtime.NumCPU())
	require.NoError(t, err)

	truthReport := readReport(t, filepath.Join(rootDir, "pkg/cli/testfiles/aws_expected_report.yaml"))
	generatedReport := readReport(t, filepath.Join(rootDir, reportFileName))
	verifyReport(t, inputDir, truthReport, generatedReport)
	verifyObfuscation(t, outputDir, generatedReport)
}

func verifyObfuscation(t *testing.T, dir string, report *reporting.Report) {
	// report should already be verified by verifyReport before to ensure it does contain correct information
	var generatedMap map[string]string
	for i := 0; i < len(report.Replacements); i++ {
		if len(report.Replacements[i]) > 0 {
			generatedMap = report.Replacements[i]
			break
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
	// the first filled map is assumed to be the IP mapping
	// TODO(thomas): this will be improved with CFE-106
	var truthMap map[string]string
	for i := 0; i < len(truthReport.Replacements); i++ {
		if len(truthReport.Replacements[i]) > 0 {
			truthMap = truthReport.Replacements[i]
			break
		}
	}

	assert.NotNilf(t, truthMap, "there must be at least an ip mapping in the truth report")

	// TODO(thomas): this will be improved with CFE-106
	var generatedMap map[string]string
	for i := 0; i < len(generatedReport.Replacements); i++ {
		if len(generatedReport.Replacements[i]) > 0 {
			generatedMap = generatedReport.Replacements[i]
			break
		}
	}
	assert.NotNilf(t, generatedMap, "there must be at least an ip mapping in the generated report")

	// important here is that we catch all the same IPs and that there is no content that equals the IP as the value
	for k, v := range generatedMap {
		assert.NotContains(t, v, k)
		assert.Contains(t, truthMap, k)
		// if we have dashes in IPs, we need to verify we map the dotted normalized version to the same obfuscated value
		if strings.Contains(k, "-") {
			dotted := strings.ReplaceAll(k, "-", ".")
			assert.Equal(t, v, generatedMap[dotted])
		}
	}

	for i := 0; i < len(generatedReport.Omissions); i++ {
		absPath := generatedReport.Omissions[i]
		relPath, err := filepath.Rel(inputDir, absPath)
		require.NoError(t, err)
		generatedReport.Omissions[i] = relPath
	}

	sort.Strings(truthReport.Omissions)
	sort.Strings(generatedReport.Omissions)
	assert.Equal(t, truthReport.Omissions, generatedReport.Omissions)
}

func readReport(t *testing.T, path string) *reporting.Report {
	bytes, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	report := &reporting.Report{}
	require.NoError(t, yaml.Unmarshal(bytes, report))
	return report
}
