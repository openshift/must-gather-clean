package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeobfuscateCommandUsesStdinAndStdout(t *testing.T) {
	oldPipeModeEnabled := PipeModeEnabled
	PipeModeEnabled = true
	t.Cleanup(func() { PipeModeEnabled = oldPipeModeEnabled })
	initFlags()
	reportPath := filepath.Join(t.TempDir(), "report.yaml")
	report := []byte("config:\n  obfuscate:\n    - type: IP\n      replacementType: Consistent\n      replacement: {original: token}\nreplacements:\n  - - canonical: original\n      replacedWith: token\n      occurrences: [{original: original}]\n")
	require.NoError(t, os.WriteFile(reportPath, report, 0600))

	oldStdin, oldStdout := os.Stdin, os.Stdout
	stdinReader, stdinWriter, err := os.Pipe()
	require.NoError(t, err)
	stdoutReader, stdoutWriter, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Stdin, os.Stdout = oldStdin, oldStdout
		_ = stdinReader.Close()
		_ = stdinWriter.Close()
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	})
	os.Stdin, os.Stdout = stdinReader, stdoutWriter
	rootCmd.SetArgs([]string{"deobfuscate", "--report", reportPath})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	outputDone := make(chan []byte, 1)
	go func() {
		output, _ := io.ReadAll(stdoutReader)
		outputDone <- output
	}()

	executionDone := make(chan error, 1)
	go func() { executionDone <- rootCmd.Execute() }()

	_, err = io.WriteString(stdinWriter, "node token\n")
	require.NoError(t, err)
	require.NoError(t, stdinWriter.Close())
	require.NoError(t, <-executionDone)
	require.NoError(t, stdoutWriter.Close())
	require.Equal(t, "node original\n", string(<-outputDone))
}
