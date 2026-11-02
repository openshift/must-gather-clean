//go:build unix

package reporting

import (
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestReporterWritesWithRestrictiveUmaskInExistingDirectory(t *testing.T) {
	for _, tc := range []struct {
		name  string
		umask int
	}{
		{name: "owner-read-and-search-removed", umask: 0177},
		{name: "all-permissions-removed", umask: 0777},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reportDir := filepath.Join(t.TempDir(), "reports")
			require.NoError(t, os.Mkdir(reportDir, 0700))
			const existingMode = os.FileMode(0750)
			require.NoError(t, os.Chmod(reportDir, existingMode))
			reportPath := filepath.Join(reportDir, "report.yaml")

			previousUmask := syscall.Umask(tc.umask)
			t.Cleanup(func() {
				syscall.Umask(previousUmask)
			})
			require.NoError(t, NewSimpleReporter(&schema.SchemaJson{}).WriteReport(reportPath))

			require.FileExists(t, reportPath)
			reportDirInfo, err := os.Stat(reportDir)
			require.NoError(t, err)
			assert.Equal(t, existingMode, reportDirInfo.Mode().Perm())
			temporaryReports, err := filepath.Glob(filepath.Join(reportDir, reportTemporaryFilePrefix+"*"))
			require.NoError(t, err)
			require.Empty(t, temporaryReports)
		})
	}
}

func TestReporterRestrictsTemporaryFileModeBeforeEncoding(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), "report.yaml")
	require.NoError(t, os.WriteFile(reportPath, []byte("old report"), 0600))
	require.NoError(t, os.Chmod(reportPath, 0600))
	reporter := NewSimpleReporter(&schema.SchemaJson{}).(*SimpleReporter)

	previousUmask := syscall.Umask(0022)
	t.Cleanup(func() {
		syscall.Umask(previousUmask)
	})
	var temporaryMode os.FileMode
	err := reporter.writeReport(reportPath, &reportWriteHooks{
		afterTemporaryFileCreated: func(_ string, file *os.File) {
			info, statErr := file.Stat()
			require.NoError(t, statErr)
			temporaryMode = info.Mode().Perm()
			contents, readErr := os.ReadFile(reportPath)
			require.NoError(t, readErr)
			assert.Equal(t, "old report", string(contents))
		},
	})
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), temporaryMode)
}

func TestReporterWritesWithRestrictiveUmaskIntoNewNestedDirectory(t *testing.T) {
	for _, tc := range []struct {
		name  string
		umask int
	}{
		{name: "owner-read-and-search-removed", umask: 0177},
		{name: "all-permissions-removed", umask: 0777},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			reportPath := filepath.Join(root, "reports", "nested", "report.yaml")
			reporter := NewSimpleReporter(&schema.SchemaJson{})
			reporter.CollectOmitterReport([]string{"nested/omitted"})

			previousUmask := syscall.Umask(tc.umask)
			t.Cleanup(func() {
				syscall.Umask(previousUmask)
			})
			require.NoError(t, reporter.WriteReport(reportPath))

			require.NoError(t, os.Chmod(reportPath, 0600))
			contents, err := os.ReadFile(reportPath)
			require.NoError(t, err)
			report := &Report{}
			require.NoError(t, yaml.Unmarshal(contents, report))
			assert.Equal(t, []string{"nested/omitted"}, report.Omissions)

			for _, reportDir := range []string{
				filepath.Join(root, "reports"),
				filepath.Dir(reportPath),
			} {
				reportDirInfo, err := os.Stat(reportDir)
				require.NoError(t, err)
				assert.Equal(t, os.FileMode(0700), reportDirInfo.Mode().Perm())
			}
			temporaryReports, err := filepath.Glob(filepath.Join(filepath.Dir(reportPath), reportTemporaryFilePrefix+"*"))
			require.NoError(t, err)
			assert.Empty(t, temporaryReports)
		})
	}
}

func TestReporterConcurrentWritersCreateNestedDirectoryWithRestrictiveUmask(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), "reports", "nested", "report.yaml")
	previousUmask := syscall.Umask(0777)
	t.Cleanup(func() {
		syscall.Umask(previousUmask)
	})

	const writers = 64
	start := make(chan struct{})
	errs := make(chan error, writers)
	var waitGroup sync.WaitGroup
	for writer := 0; writer < writers; writer++ {
		waitGroup.Add(1)
		go func(writer int) {
			defer waitGroup.Done()
			<-start
			reporter := NewSimpleReporter(&schema.SchemaJson{})
			reporter.CollectOmitterReport([]string{string(rune('a' + writer))})
			errs <- reporter.WriteReport(reportPath)
		}(writer)
	}
	close(start)
	waitGroup.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.FileExists(t, reportPath)
	temporaryReports, err := filepath.Glob(filepath.Join(filepath.Dir(reportPath), reportTemporaryFilePrefix+"*"))
	require.NoError(t, err)
	assert.Empty(t, temporaryReports)
}
