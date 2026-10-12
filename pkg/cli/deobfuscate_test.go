package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/deobfuscator"
	"github.com/stretchr/testify/require"
)

const validDeobfuscationReport = `config:
  obfuscate:
    - type: IP
      replacementType: Consistent
      replacement:
        original: token
replacements:
  - - canonical: original
      replacedWith: token
      occurrences:
        - original: original
`

func TestRunDeobfuscate(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), "report.yaml")
	require.NoError(t, os.WriteFile(reportPath, []byte(validDeobfuscationReport), 0600))

	var output bytes.Buffer
	err := RunDeobfuscate(reportPath, bytes.NewBufferString("node token unknown\n"), &output)
	require.NoError(t, err)
	require.Equal(t, "node original unknown\n", output.String())
}

func TestRunDeobfuscateReportsReadAndWriteErrors(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), "report.yaml")
	require.NoError(t, os.WriteFile(reportPath, []byte(validDeobfuscationReport), 0600))

	readErr := RunDeobfuscate(reportPath, failingReader{err: io.ErrUnexpectedEOF}, &bytes.Buffer{})
	require.ErrorIs(t, readErr, io.ErrUnexpectedEOF)

	writeErr := RunDeobfuscate(reportPath, bytes.NewBufferString("token"), failingWriter{err: io.ErrClosedPipe})
	require.ErrorIs(t, writeErr, io.ErrClosedPipe)

	shortWriteErr := RunDeobfuscate(reportPath, bytes.NewBufferString("token"), shortWriter{})
	require.ErrorIs(t, shortWriteErr, io.ErrShortWrite)
}

func TestRunDeobfuscateFile(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "report.yaml")
	inputPath := filepath.Join(dir, "response.txt")
	outputPath := filepath.Join(dir, "restored.txt")
	require.NoError(t, os.WriteFile(reportPath, []byte(validDeobfuscationReport), 0600))
	require.NoError(t, os.WriteFile(inputPath, []byte("token\n"), 0600))

	require.NoError(t, RunDeobfuscateFile(reportPath, inputPath, outputPath))
	restored, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Equal(t, "original\n", string(restored))
}

func TestRunDeobfuscateFilePreservesOutputWhenInputReadFails(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "report.yaml")
	inputPath := filepath.Join(dir, "input-directory")
	require.NoError(t, os.WriteFile(reportPath, []byte(validDeobfuscationReport), 0600))
	require.NoError(t, os.Mkdir(inputPath, 0700))

	outputPath := filepath.Join(dir, "existing-output.txt")
	outputBefore := []byte("sentinel output")
	require.NoError(t, os.WriteFile(outputPath, outputBefore, 0600))
	err := RunDeobfuscateFile(reportPath, inputPath, outputPath)
	require.Error(t, err)
	outputAfter, readErr := os.ReadFile(outputPath)
	require.NoError(t, readErr)
	require.Equal(t, outputBefore, outputAfter)

	missingOutputPath := filepath.Join(dir, "missing-output.txt")
	err = RunDeobfuscateFile(reportPath, inputPath, missingOutputPath)
	require.Error(t, err)
	_, statErr := os.Stat(missingOutputPath)
	require.ErrorIs(t, statErr, os.ErrNotExist)

	reportAfter, readErr := os.ReadFile(reportPath)
	require.NoError(t, readErr)
	require.Equal(t, []byte(validDeobfuscationReport), reportAfter)
	inputInfo, statErr := os.Stat(inputPath)
	require.NoError(t, statErr)
	require.True(t, inputInfo.IsDir())
}

func TestWriteDeobfuscatedFileDoesNotPublishPartialOutput(t *testing.T) {
	for _, outputExists := range []bool{false, true} {
		t.Run(map[bool]string{false: "new-output", true: "existing-output"}[outputExists], func(t *testing.T) {
			outputPath := filepath.Join(t.TempDir(), "output.txt")
			before := []byte("sentinel output")
			if outputExists {
				require.NoError(t, os.WriteFile(outputPath, before, 0600))
			}

			err := writeDeobfuscatedFile(
				deobfuscator.Mapping{"token": "original"},
				&partialErrorReader{},
				outputPath,
				nil,
			)
			require.ErrorIs(t, err, io.ErrUnexpectedEOF)

			if outputExists {
				after, readErr := os.ReadFile(outputPath)
				require.NoError(t, readErr)
				require.Equal(t, before, after)
			} else {
				_, statErr := os.Stat(outputPath)
				require.ErrorIs(t, statErr, os.ErrNotExist)
			}
		})
	}
}

func TestRunDeobfuscateFileWritesThroughOutputSymlink(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "report.yaml")
	inputPath := filepath.Join(dir, "input.txt")
	targetPath := filepath.Join(dir, "target.txt")
	outputPath := filepath.Join(dir, "output-link.txt")
	require.NoError(t, os.WriteFile(reportPath, []byte(validDeobfuscationReport), 0600))
	require.NoError(t, os.WriteFile(inputPath, []byte("token\n"), 0600))
	require.NoError(t, os.WriteFile(targetPath, []byte("old output"), 0600))
	require.NoError(t, os.Symlink(filepath.Base(targetPath), outputPath))

	require.NoError(t, RunDeobfuscateFile(reportPath, inputPath, outputPath))

	linkInfo, err := os.Lstat(outputPath)
	require.NoError(t, err)
	require.NotZero(t, linkInfo.Mode()&os.ModeSymlink)
	restored, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	require.Equal(t, "original\n", string(restored))
}

func TestRunDeobfuscateFileRejectsSameInputAndOutput(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "report.yaml")
	inputPath := filepath.Join(dir, "response.txt")
	require.NoError(t, os.WriteFile(reportPath, []byte(validDeobfuscationReport), 0600))
	require.NoError(t, os.WriteFile(inputPath, []byte("token\n"), 0600))

	err := RunDeobfuscateFile(reportPath, inputPath, inputPath)
	require.EqualError(t, err, "input and output paths must differ")
	contents, readErr := os.ReadFile(inputPath)
	require.NoError(t, readErr)
	require.Equal(t, "token\n", string(contents))
}

func TestRunDeobfuscateFileRejectsHardLinkAlias(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "report.yaml")
	inputPath := filepath.Join(dir, "response.txt")
	outputPath := filepath.Join(dir, "response-alias.txt")
	require.NoError(t, os.WriteFile(reportPath, []byte(validDeobfuscationReport), 0600))
	require.NoError(t, os.WriteFile(inputPath, []byte("token\n"), 0600))
	if err := os.Link(inputPath, outputPath); err != nil {
		t.Skipf("hard links are unavailable: %v", err)
	}

	err := RunDeobfuscateFile(reportPath, inputPath, outputPath)
	require.EqualError(t, err, "input and output paths must differ")
	contents, readErr := os.ReadFile(inputPath)
	require.NoError(t, readErr)
	require.Equal(t, "token\n", string(contents))
}

func TestRunDeobfuscateFileRejectsReportAsOutput(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "report.yaml")
	inputPath := filepath.Join(dir, "input.txt")
	report := []byte(validDeobfuscationReport)
	require.NoError(t, os.WriteFile(reportPath, report, 0600))
	require.NoError(t, os.WriteFile(inputPath, []byte("token\n"), 0600))

	err := RunDeobfuscateFile(reportPath, inputPath, reportPath)
	require.EqualError(t, err, "report and output paths must differ")
	contents, readErr := os.ReadFile(reportPath)
	require.NoError(t, readErr)
	require.Equal(t, report, contents)
}

func TestRunDeobfuscateFileRejectsInvalidReportBeforeCreatingOutput(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "report.yaml")
	inputPath := filepath.Join(dir, "input.txt")
	outputPath := filepath.Join(dir, "restored.txt")
	require.NoError(t, os.WriteFile(reportPath, []byte("replacements: ["), 0600))
	require.NoError(t, os.WriteFile(inputPath, []byte("token\n"), 0600))

	err := RunDeobfuscateFile(reportPath, inputPath, outputPath)
	require.Error(t, err)
	_, statErr := os.Stat(outputPath)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestRunDeobfuscateFileRejectsInputReportAliases(t *testing.T) {
	for _, mode := range []string{"exact", "hardlink", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			reportPath := filepath.Join(dir, "report.yaml")
			reportData := []byte(validDeobfuscationReport)
			require.NoError(t, os.WriteFile(reportPath, reportData, 0600))

			inputPath := reportPath
			switch mode {
			case "hardlink":
				inputPath = filepath.Join(dir, "input-hardlink")
				require.NoError(t, os.Link(reportPath, inputPath))
			case "symlink":
				inputPath = filepath.Join(dir, "input-symlink")
				require.NoError(t, os.Symlink(reportPath, inputPath))
			}
			outputPath := filepath.Join(dir, "output.txt")

			err := RunDeobfuscateFile(reportPath, inputPath, outputPath)
			require.EqualError(t, err, "input and report paths must differ")
			after, readErr := os.ReadFile(reportPath)
			require.NoError(t, readErr)
			require.True(t, bytes.Equal(reportData, after))
			_, statErr := os.Stat(outputPath)
			require.ErrorIs(t, statErr, os.ErrNotExist)
		})
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

type partialErrorReader struct{ read bool }

func (r *partialErrorReader) Read(p []byte) (int, error) {
	if r.read {
		return 0, io.ErrUnexpectedEOF
	}
	r.read = true
	return copy(p, []byte("token\npartial")), io.ErrUnexpectedEOF
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
