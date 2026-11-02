package reporting

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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

func TestReporterSerializesEntriesInDocumentedOrder(t *testing.T) {
	config := &schema.SchemaJson{Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{{
		Type: schema.ObfuscateTypeDomain,
	}}}}
	reporter := NewSimpleReporter(config)
	reporter.CollectOmitterReport([]string{"z-path", "a-path"})
	reporter.CollectObfuscatorReport([]obfuscator.ReplacementReport{{
		Replacements: []obfuscator.Replacement{
			{Canonical: "z.example", ReplacedWith: "z-replacement", Counter: map[string]uint{"z-original": 1, "a-original": 2}},
			{Canonical: "a.example", ReplacedWith: "a-replacement", Counter: map[string]uint{"z-original": 1, "a-original": 1}},
		},
	}})

	reportPath := filepath.Join(t.TempDir(), "report.yaml")
	require.NoError(t, reporter.WriteReport(reportPath))
	contents, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	serialized := &Report{}
	require.NoError(t, yaml.Unmarshal(contents, serialized))

	assert.Equal(t, []string{"a-path", "z-path"}, serialized.Omissions)
	assert.Equal(t, [][]Replacement{{
		{Canonical: "a.example", ReplacedWith: "a-replacement", Occurrences: []Occurrence{{Original: "a-original", Count: 1}, {Original: "z-original", Count: 1}}},
		{Canonical: "z.example", ReplacedWith: "z-replacement", Occurrences: []Occurrence{{Original: "a-original", Count: 2}, {Original: "z-original", Count: 1}}},
	}}, serialized.Replacements)
}

func TestReporterProducesStableBytesForEquivalentReports(t *testing.T) {
	const replacementKeys = "abcdefgh"
	buildReporter := func(reverse bool) Reporter {
		makeReplacements := func(prefix string) schema.ObfuscateReplacement {
			replacements := schema.ObfuscateReplacement{}
			for i := range replacementKeys {
				keyIndex := i
				if reverse {
					keyIndex = len(replacementKeys) - 1 - i
				}
				key := string(replacementKeys[keyIndex])
				replacements[key] = prefix + key
			}
			return replacements
		}

		reporter := NewSimpleReporter(&schema.SchemaJson{Config: schema.SchemaJsonConfig{
			Obfuscate: []schema.Obfuscate{
				{Type: schema.ObfuscateTypeKeywords, Replacement: makeReplacements("first-")},
				{Type: schema.ObfuscateTypeKeywords, Replacement: makeReplacements("second-")},
			},
		}})
		reporter.CollectOmitterReport([]string{"z-path", "a-path"})
		reports := []obfuscator.ReplacementReport{
			{Replacements: []obfuscator.Replacement{
				{Canonical: "z.example", ReplacedWith: "z-first", Counter: map[string]uint{"z-original": 1, "a-original": 2}},
				{Canonical: "a.example", ReplacedWith: "a-first", Counter: map[string]uint{"z-original": 1, "a-original": 1}},
			}},
			{Replacements: []obfuscator.Replacement{
				{Canonical: "y.example", ReplacedWith: "y-second", Counter: map[string]uint{"y-original": 1, "b-original": 2}},
				{Canonical: "b.example", ReplacedWith: "b-second", Counter: map[string]uint{"y-original": 1, "b-original": 1}},
			}},
		}
		if reverse {
			for i := range reports {
				reports[i].Replacements[0], reports[i].Replacements[1] = reports[i].Replacements[1], reports[i].Replacements[0]
			}
		}
		reporter.CollectObfuscatorReport(reports)
		return reporter
	}

	firstPath := filepath.Join(t.TempDir(), "first.yaml")
	secondPath := filepath.Join(t.TempDir(), "second.yaml")
	require.NoError(t, buildReporter(false).WriteReport(firstPath))
	require.NoError(t, buildReporter(true).WriteReport(secondPath))

	first, err := os.ReadFile(firstPath)
	require.NoError(t, err)
	second, err := os.ReadFile(secondPath)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(first, second), "equivalent reports have different serialized bytes")
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

func TestReporterLeavesExistingReportOnCollectionError(t *testing.T) {
	config := &schema.SchemaJson{Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{
		{Type: schema.ObfuscateTypeIP},
		{Type: schema.ObfuscateTypeMAC},
	}}}
	reporter := NewSimpleReporter(config)
	reporter.CollectObfuscatorReport([]obfuscator.ReplacementReport{{}})

	reportPath := filepath.Join(t.TempDir(), "report.yaml")
	require.NoError(t, os.WriteFile(reportPath, []byte("previous report\n"), 0640))
	require.NoError(t, os.Chmod(reportPath, 0640))

	err := reporter.WriteReport(reportPath)
	require.ErrorContains(t, err, "1 replacement groups for 2 configured obfuscators")
	contents, readErr := os.ReadFile(reportPath)
	require.NoError(t, readErr)
	assert.Equal(t, "previous report\n", string(contents))
	info, statErr := os.Stat(reportPath)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0640), info.Mode().Perm())
}

func TestReporterRejectsMissingConfiguration(t *testing.T) {
	reporter := NewSimpleReporter(nil)
	reporter.CollectObfuscatorReport(nil)

	err := reporter.WriteReport(filepath.Join(t.TempDir(), "report.yaml"))
	require.ErrorContains(t, err, "without a configuration")
}

func TestReporterUsesUpstreamFileAndDirectoryPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix directory and file permission bits")
	}
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
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	const modeMask = os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky
	reportPath := filepath.Join(t.TempDir(), "report.yaml")
	require.NoError(t, os.WriteFile(reportPath, []byte("old report"), 0600))
	existingMode := os.FileMode(0640) | os.ModeSetgid
	require.NoError(t, os.Chmod(reportPath, existingMode))

	reporter := NewSimpleReporter(&schema.SchemaJson{})
	require.NoError(t, reporter.WriteReport(reportPath))

	info, err := os.Stat(reportPath)
	require.NoError(t, err)
	assert.Equal(t, existingMode, info.Mode()&modeMask)
}

func TestReporterCleansTemporaryFileAfterEncodingError(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), "report.yaml")
	require.NoError(t, os.WriteFile(reportPath, []byte("old report"), 0640))
	require.NoError(t, os.Chmod(reportPath, 0640))
	reporter := NewSimpleReporter(&schema.SchemaJson{}).(*SimpleReporter)

	err := reporter.writeReport(reportPath, &reportWriteHooks{
		afterTemporaryFileCreated: func(_ string, file *os.File) {
			require.NoError(t, file.Close())
		},
	})
	require.ErrorContains(t, err, "failed to write report")

	contents, readErr := os.ReadFile(reportPath)
	require.NoError(t, readErr)
	assert.Equal(t, "old report", string(contents))
	info, statErr := os.Stat(reportPath)
	require.NoError(t, statErr)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0640), info.Mode().Perm())
	}
	temporaryReports, globErr := filepath.Glob(filepath.Join(filepath.Dir(reportPath), reportTemporaryFilePrefix+"*"))
	require.NoError(t, globErr)
	assert.Empty(t, temporaryReports)
}

func TestReporterWritesThroughReportSymlink(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target-report.yaml")
	reportPath := filepath.Join(dir, "report.yaml")
	require.NoError(t, os.WriteFile(targetPath, []byte("old report"), 0600))
	if err := os.Symlink(filepath.Base(targetPath), reportPath); err != nil {
		t.Skipf("cannot create symbolic link: %v", err)
	}

	reporter := NewSimpleReporter(&schema.SchemaJson{})
	require.NoError(t, reporter.WriteReport(reportPath))

	linkInfo, err := os.Lstat(reportPath)
	require.NoError(t, err)
	assert.NotZero(t, linkInfo.Mode()&os.ModeSymlink)
	contents, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	assert.NotContains(t, string(contents), "old report")
}

func TestReporterWritesThroughDanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target-report.yaml")
	reportPath := filepath.Join(dir, "report.yaml")
	if err := os.Symlink(filepath.Base(targetPath), reportPath); err != nil {
		t.Skipf("cannot create symbolic link: %v", err)
	}

	reporter := NewSimpleReporter(&schema.SchemaJson{})
	require.NoError(t, reporter.WriteReport(reportPath))

	linkInfo, err := os.Lstat(reportPath)
	require.NoError(t, err)
	assert.NotZero(t, linkInfo.Mode()&os.ModeSymlink)
	contents, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	report := &Report{}
	require.NoError(t, yaml.Unmarshal(contents, report))
	assert.Empty(t, report.Replacements)
}

func TestTemporaryReportFileIsRegularFileInPinnedDirectory(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	defer func() {
		_ = root.Close()
	}()

	name, file, err := createTemporaryReportFile(root)
	require.NoError(t, err)
	defer func() {
		_ = file.Close()
		_ = root.Remove(name)
	}()

	assert.Equal(t, filepath.Base(name), name)
	info, err := root.Stat(name)
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular())
}

func TestReporterConcurrentWritersPublishCompleteReports(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "report.yaml")
	const writers = 16

	var waitGroup sync.WaitGroup
	errors := make(chan error, writers)
	for writer := 0; writer < writers; writer++ {
		waitGroup.Add(1)
		go func(writer int) {
			defer waitGroup.Done()
			reporter := NewSimpleReporter(&schema.SchemaJson{})
			reporter.CollectOmitterReport([]string{string(rune('a' + writer))})
			errors <- reporter.WriteReport(reportPath)
		}(writer)
	}
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}

	contents, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	report := &Report{}
	require.NoError(t, yaml.Unmarshal(contents, report))
	require.Len(t, report.Omissions, 1)
	temporaryReports, err := filepath.Glob(filepath.Join(dir, reportTemporaryFilePrefix+"*"))
	require.NoError(t, err)
	assert.Empty(t, temporaryReports)
}

func TestReporterWritesThroughRelativeSymlinkWithSymlinkedParent(t *testing.T) {
	root := t.TempDir()
	realParent := filepath.Join(root, "real", "deep")
	require.NoError(t, os.MkdirAll(realParent, 0700))
	linkedParent := filepath.Join(root, "linked")
	if err := os.Symlink(realParent, linkedParent); err != nil {
		t.Skipf("cannot create symbolic link: %v", err)
	}

	targetPath := filepath.Join(root, "real", "target-report.yaml")
	require.NoError(t, os.WriteFile(targetPath, []byte("old report"), 0600))
	reportPath := filepath.Join(linkedParent, "report.yaml")
	if err := os.Symlink("../target-report.yaml", reportPath); err != nil {
		t.Skipf("cannot create symbolic link: %v", err)
	}

	reporter := NewSimpleReporter(&schema.SchemaJson{})
	require.NoError(t, reporter.WriteReport(reportPath))

	contents, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	report := &Report{}
	require.NoError(t, yaml.Unmarshal(contents, report))
	assert.Empty(t, report.Replacements)
	assert.NoFileExists(t, filepath.Join(root, "target-report.yaml"))
}

func TestReporterDoesNotFollowMissingComponentBeforeParentTraversal(t *testing.T) {
	root := t.TempDir()
	sentinel := filepath.Join(root, "report.yaml")
	require.NoError(t, os.WriteFile(sentinel, []byte("keep"), 0600))
	reportLink := filepath.Join(root, "report-link.yaml")
	if err := os.Symlink("missing"+string(os.PathSeparator)+".."+string(os.PathSeparator)+"report.yaml", reportLink); err != nil {
		t.Skipf("cannot create symbolic link: %v", err)
	}

	err := NewSimpleReporter(&schema.SchemaJson{}).WriteReport(reportLink)
	require.ErrorContains(t, err, "after a missing component")
	contents, readErr := os.ReadFile(sentinel)
	require.NoError(t, readErr)
	assert.Equal(t, "keep", string(contents))
}

func TestReporterRejectsDirectoryShapedMissingTargets(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, root string) string
	}{
		{
			name: "trailing-separator",
			setup: func(_ *testing.T, root string) string {
				return filepath.Join(root, "missing-report") + string(os.PathSeparator)
			},
		},
		{
			name: "trailing-dot",
			setup: func(_ *testing.T, root string) string {
				return filepath.Join(root, "missing-report") + string(os.PathSeparator) + "."
			},
		},
		{
			name: "dangling-link-trailing-separator",
			setup: func(t *testing.T, root string) string {
				link := filepath.Join(root, "report-link")
				if err := os.Symlink("missing-target", link); err != nil {
					t.Skipf("cannot create symbolic link: %v", err)
				}
				return link + string(os.PathSeparator)
			},
		},
		{
			name: "dangling-target-trailing-separator",
			setup: func(t *testing.T, root string) string {
				link := filepath.Join(root, "report-link")
				if err := os.Symlink("missing-target"+string(os.PathSeparator), link); err != nil {
					t.Skipf("cannot create symbolic link: %v", err)
				}
				return link
			},
		},
		{
			name: "dangling-target-dot",
			setup: func(t *testing.T, root string) string {
				link := filepath.Join(root, "report-link")
				if err := os.Symlink("missing-target"+string(os.PathSeparator)+".", link); err != nil {
					t.Skipf("cannot create symbolic link: %v", err)
				}
				return link
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := tc.setup(t, root)
			before := directoryEntryNames(t, root)
			err := NewSimpleReporter(&schema.SchemaJson{}).WriteReport(path)
			require.ErrorContains(t, err, "directory path")
			assert.Equal(t, before, directoryEntryNames(t, root))
			temporaryReports, globErr := filepath.Glob(filepath.Join(root, reportTemporaryFilePrefix+"*"))
			require.NoError(t, globErr)
			assert.Empty(t, temporaryReports)
		})
	}
}

func TestReporterRejectsDirectoryShapedExistingTargets(t *testing.T) {
	for _, tc := range []struct {
		name string
		path func(root string) string
	}{
		{
			name: "trailing-separator",
			path: func(root string) string {
				return filepath.Join(root, "report.yaml") + string(os.PathSeparator)
			},
		},
		{
			name: "trailing-dot",
			path: func(root string) string {
				return filepath.Join(root, "report.yaml") + string(os.PathSeparator) + "."
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			reportPath := filepath.Join(root, "report.yaml")
			require.NoError(t, os.WriteFile(reportPath, []byte("keep"), 0600))

			err := NewSimpleReporter(&schema.SchemaJson{}).WriteReport(tc.path(root))
			require.Error(t, err)
			contents, readErr := os.ReadFile(reportPath)
			require.NoError(t, readErr)
			assert.Equal(t, "keep", string(contents))
			temporaryReports, globErr := filepath.Glob(filepath.Join(root, reportTemporaryFilePrefix+"*"))
			require.NoError(t, globErr)
			assert.Empty(t, temporaryReports)
		})
	}
}

func directoryEntryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestReporterPublishesIntoPinnedDirectoryAfterParentSwap(t *testing.T) {
	root := t.TempDir()
	reportDir := filepath.Join(root, "reports")
	require.NoError(t, os.Mkdir(reportDir, 0700))
	movedDir := filepath.Join(root, "reports-original")
	reportPath := filepath.Join(reportDir, "report.yaml")
	victimPath := filepath.Join(reportDir, "unrelated-report.yaml")
	reporter := NewSimpleReporter(&schema.SchemaJson{}).(*SimpleReporter)

	err := reporter.writeReport(reportPath, &reportWriteHooks{
		afterOpenRoot: func() {
			require.NoError(t, os.Rename(reportDir, movedDir))
			require.NoError(t, os.Mkdir(reportDir, 0700))
			require.NoError(t, os.WriteFile(victimPath, []byte("unrelated report\n"), 0600))
		},
	})
	require.NoError(t, err)

	contents, err := os.ReadFile(victimPath)
	require.NoError(t, err)
	assert.Equal(t, "unrelated report\n", string(contents))
	published := &Report{}
	contents, err = os.ReadFile(filepath.Join(movedDir, "report.yaml"))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(contents, published))
	temporaryReports, err := filepath.Glob(filepath.Join(movedDir, reportTemporaryFilePrefix+"*"))
	require.NoError(t, err)
	assert.Empty(t, temporaryReports)
}

func TestReporterAcceptsLongValidBasename(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), strings.Repeat("r", 240))
	require.NoError(t, NewSimpleReporter(&schema.SchemaJson{}).WriteReport(reportPath))
	require.FileExists(t, reportPath)
}
