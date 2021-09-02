package cli

import (
	"io/ioutil"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/reporting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestEndToEndWithExample(t *testing.T) {
	// find the path to the project root of this file
	_, filename, _, _ := runtime.Caller(0)
	rootDir := path.Join(path.Dir(filename), "..", "..")
	inputDir := filepath.Join(rootDir, "pkg/cli/testfiles/release-4.8/")
	outputDir := filepath.Join(rootDir, "pkg/cli/testfiles/release-4.8.cleaned/")
	truthDir := filepath.Join(rootDir, "pkg/cli/testfiles/release-4.8.cleaned-truth/")
	err := Run(filepath.Join(rootDir, "examples/openshift_default.yaml"),
		inputDir,
		outputDir,
		true,
		rootDir,
		1)
	require.NoError(t, err)

	truthReport := readReport(t, filepath.Join(truthDir, reportFileName))
	generatedReport := readReport(t, filepath.Join(rootDir, reportFileName))
	verifyReport(t, rootDir, truthReport, generatedReport)
	verifyOmissions(t, inputDir, outputDir, generatedReport)
}

func verifyReport(t *testing.T, inputDir string, truthReport *reporting.Report, generatedReport *reporting.Report) {
	// the first filled map is assumed to be the IP mapping
	var truthMap map[string]string
	for i := 0; i < len(truthReport.Replacements); i++ {
		if len(truthReport.Replacements[i]) > 0 {
			truthMap = truthReport.Replacements[i]
			break
		}
	}

	assert.NotNilf(t, truthMap, "there must be at least an ip mapping in the truth report")

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
	}

	var absOmissions []string
	for i := 0; i < len(truthReport.Omissions); i++ {
		absOmissions = append(absOmissions, filepath.Join(inputDir, truthReport.Omissions[i]))
	}

	assert.Equal(t, absOmissions, generatedReport.Omissions)
}

func readReport(t *testing.T, path string) *reporting.Report {
	bytes, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	report := &reporting.Report{}
	require.NoError(t, yaml.Unmarshal(bytes, report))
	return report
}

// this verifies the rules to omit kubernetes resources in "namespaces/kube-system/core/"
func verifyOmissions(t *testing.T, inputDir string, outputDir string, report *reporting.Report) {
	const searchPath = "namespaces/kube-system/core/"
	reportedOmissions := report.Omissions
	// we only expect the secret and configmap to vanish
	assert.Equalf(t, 2, len(reportedOmissions), "unexpected omissions found: %v", reportedOmissions)

	inputDirEntries, err := ioutil.ReadDir(filepath.Join(inputDir, searchPath))
	require.NoError(t, err)
	inputAsMap := map[string]interface{}{}
	for _, entry := range inputDirEntries {
		inputAsMap[entry.Name()] = true
	}

	outputDirEntries, err := ioutil.ReadDir(filepath.Join(outputDir, searchPath))
	require.NoError(t, err)
	for _, entry := range outputDirEntries {
		delete(inputAsMap, entry.Name())
	}

	// the remainder should equal the reported omissions
	var remainder []string
	for k, _ := range inputAsMap {
		k = filepath.Join(inputDir, searchPath, k)
		remainder = append(remainder, k)
	}

	sort.Strings(remainder)
	sort.Strings(reportedOmissions)

	assert.Equal(t, reportedOmissions, remainder)
}
