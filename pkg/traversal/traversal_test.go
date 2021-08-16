package traversal

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/openshift/must-gather-clean/pkg/input"
	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/omitter"
	"github.com/openshift/must-gather-clean/pkg/output"
)

type noopObfuscator struct {
	replacements map[string]string
}

func (d noopObfuscator) GetReplacement(original string) string {
	return original
}

func (d noopObfuscator) FileName(input string) string {
	return input
}

func (d noopObfuscator) Contents(input string) string {
	return input
}

func (d noopObfuscator) ReportingResult() map[string]string {
	return d.replacements
}

func (d noopObfuscator) ReportReplacement(original string, replacement string) {

}

type inMemfile struct {
	Contents    string      `yaml:"contents"`
	Permissions os.FileMode `yaml:"permissions"`
}

type testContents struct {
	Files map[string]string `yaml:"files"`
}

type memoryOutputter struct {
	t     *testing.T
	Files map[string]inMemfile
}

func (m *memoryOutputter) Writer(parent string, name string, permissions os.FileMode) (output.Closer, io.StringWriter, error) {
	filePath := filepath.Join(parent, name)
	require.NotContains(m.t, m.Files, filePath)
	buffer := &bytes.Buffer{}
	return func() error {
		m.Files[filePath] = inMemfile{Contents: buffer.String(), Permissions: permissions.Perm()}
		return nil
	}, buffer, nil
}

func testOutputter(t *testing.T) *memoryOutputter {
	return &memoryOutputter{t: t, Files: map[string]inMemfile{}}
}

func TestFileWalker(t *testing.T) {
	for _, tc := range []struct {
		name     string
		inputDir string
	}{
		{
			name:     "basic",
			inputDir: "testfiles/test1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fileOmitter, err := omitter.NewFilenamePatternOmitter("*.log")
			require.NoError(t, err)
			writer := testOutputter(t)
			reader, err := input.NewFSInput(filepath.Join(tc.inputDir, "mg"))
			require.NoError(t, err)
			walker, err := NewFileWalker(reader, writer, []obfuscator.Obfuscator{
				noopObfuscator{replacements: map[string]string{"secret": "xxxxxx"}},
			}, []omitter.Omitter{fileOmitter})
			require.NoError(t, err)
			err = walker.Traverse()
			require.NoError(t, err)
			contentBytes, err := ioutil.ReadFile(filepath.Join(tc.inputDir, "contents.yaml"))
			require.NoError(t, err)
			var contents testContents
			err = yaml.Unmarshal(contentBytes, &contents)
			require.NoError(t, err)
			verifyFiles(t, tc.inputDir, writer.Files, contents.Files)

			var report Report
			f, err := os.Open(filepath.Join(tc.inputDir, "report.yaml"))
			require.NoError(t, err)
			d := yaml.NewDecoder(f)
			err = d.Decode(&report)
			require.NoError(t, err)
			require.Equal(t, &report, walker.GenerateReport())
		})
	}
}

func verifyFiles(t *testing.T, inputDir string, actualFiles map[string]inMemfile, expectedFiles map[string]string) {
	t.Helper()
	require.Len(t, actualFiles, len(expectedFiles))
	for expectedPath, file := range expectedFiles {
		require.Contains(t, actualFiles, expectedPath)
		actualFile := actualFiles[expectedPath]
		require.Equal(t, file, actualFile.Contents)
		fileInfo, err := os.Stat(filepath.Join(inputDir, "mg", expectedPath))
		require.NoError(t, err)
		require.Equal(t, actualFile.Permissions, fileInfo.Mode().Perm())
	}
}
