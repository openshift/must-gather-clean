//go:build linux

package reporting

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReporterAcceptsTargetNearPATHMAX(t *testing.T) {
	const targetLength = 4085
	root := t.TempDir()
	reportName := "report.yaml"
	remaining := targetLength - len(root) - len(string(os.PathSeparator)+reportName)
	require.Positive(t, remaining)

	reportDir := root
	for remaining > 0 {
		componentLength := min(200, remaining-1)
		reportDir = filepath.Join(reportDir, strings.Repeat("d", componentLength))
		require.NoError(t, os.Mkdir(reportDir, 0700))
		remaining -= componentLength + 1
	}
	reportPath := filepath.Join(reportDir, reportName)
	require.Len(t, reportPath, targetLength)

	require.NoError(t, NewSimpleReporter(&schema.SchemaJson{}).WriteReport(reportPath))
	require.FileExists(t, reportPath)
}

func TestReporterRejectsUnreadableOutputDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses Unix permission checks")
	}
	root := t.TempDir()
	reportDir := filepath.Join(root, "reports")
	require.NoError(t, os.Mkdir(reportDir, 0300))
	t.Cleanup(func() {
		_ = os.Chmod(reportDir, 0700)
	})
	reportPath := filepath.Join(reportDir, "report.yaml")

	err := NewSimpleReporter(&schema.SchemaJson{}).WriteReport(reportPath)
	require.ErrorContains(t, err, "failed to open reporting output folder")

	require.NoError(t, os.Chmod(reportDir, 0700))
	assert.NoFileExists(t, reportPath)
	temporaryReports, globErr := filepath.Glob(filepath.Join(reportDir, reportTemporaryFilePrefix+"*"))
	require.NoError(t, globErr)
	assert.Empty(t, temporaryReports)
}

func TestReporterRejectsNonRegularTarget(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), "report.yaml")
	require.NoError(t, syscall.Mkfifo(reportPath, 0600))

	err := NewSimpleReporter(&schema.SchemaJson{}).WriteReport(reportPath)
	require.ErrorContains(t, err, "not a regular file")
	info, statErr := os.Lstat(reportPath)
	require.NoError(t, statErr)
	require.NotZero(t, info.Mode()&os.ModeNamedPipe)
}
