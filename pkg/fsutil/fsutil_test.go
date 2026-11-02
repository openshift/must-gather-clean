package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resolvedTempDir(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	resolved, err := filepath.EvalSymlinks(directory)
	require.NoError(t, err)
	return resolved
}

func TestExistingEmptyDir(t *testing.T) {
	testDir, err := os.MkdirTemp(os.TempDir(), "test-dir-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(testDir)
	}()

	err = ensureOutputPath(testDir, false, testDir)
	require.NoError(t, err)
}

func TestEnsureOutputPathNonEmptyDir(t *testing.T) {
	testDir, err := os.MkdirTemp(os.TempDir(), "test-dir-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(testDir)
	}()

	err = os.WriteFile(filepath.Join(testDir, "nonempty"), []byte("nonempty"), 0644)
	require.NoError(t, err)

	err = ensureOutputPath(testDir, false, testDir)
	require.Error(t, err)
	require.Equal(t, fmt.Errorf("output directory %s is not empty", testDir), err)
}

func TestEnsureOutputPathInvalidLocation(t *testing.T) {
	file, err := os.CreateTemp("", "temp-file")
	require.NoError(t, err)
	_, err = file.Write([]byte("test-contents"))
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)

	err = ensureOutputPath(file.Name(), false, file.Name())
	require.Error(t, err)
	require.Equal(t, fmt.Errorf("output destination must be a directory: '%s'", file.Name()), err)
}

func TestEnsureOutputPathCreateIfRequired(t *testing.T) {
	testDir, err := os.MkdirTemp("", "test-dir-*")
	require.NoError(t, err)
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(testDir)

	outputDir := filepath.Join(testDir, "nonexistent")
	err = ensureOutputPath(outputDir, false, testDir)
	require.NoError(t, err)
	info, err := os.Stat(outputDir)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestEnsureOutputPathDeletesIfRequired(t *testing.T) {
	testDir, err := os.MkdirTemp(os.TempDir(), "test-dir-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(testDir)
	}()

	secondTestDir, err := os.MkdirTemp(os.TempDir(), "test-dir-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(secondTestDir)
	}()

	toBeDeletedFile := filepath.Join(testDir, "nonempty")
	err = os.WriteFile(toBeDeletedFile, []byte("nonempty"), 0664)
	require.NoError(t, err)

	err = ensureOutputPath(testDir, true, secondTestDir)
	require.NoError(t, err)

	info, err := os.Stat(testDir)
	require.NoError(t, err)
	require.True(t, info.IsDir())

	_, err = os.Stat(toBeDeletedFile)
	require.Error(t, err)
	require.Truef(t, os.IsNotExist(err), "file %s exists even though it shouldn't: %s", toBeDeletedFile, err)
}

func TestMkdirRecursively(t *testing.T) {
	testDir, err := os.MkdirTemp(os.TempDir(), "test-dir-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(testDir)
	}()
	initialInfo, err := os.Stat(testDir)
	require.NoError(t, err)

	// this creates a parallel folder structure with testDir permissions
	inputFolder := filepath.Join(testDir, "b", "a", "a")
	require.NoError(t, os.MkdirAll(inputFolder, initialInfo.Mode()))

	outputFolder := filepath.Join(testDir, "a", "a", "a")
	require.NoError(t, MkdirAllWithChown(outputFolder, inputFolder))

	expectedInfo, err := os.Stat(inputFolder)
	require.NoError(t, err)
	actualInfo, err := os.Stat(outputFolder)
	require.NoError(t, err)
	assert.Equal(t, expectedInfo.Mode(), actualInfo.Mode())
}

func TestSymlinkDetection(t *testing.T) {
	tmpInputDir, err := os.MkdirTemp(os.TempDir(), "test-dir-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tmpInputDir)
	}()

	textFile := filepath.Join(tmpInputDir, "link.txt")
	err = os.WriteFile(textFile, []byte("text"), 0644)
	require.NoError(t, err)

	linkPath := filepath.Join(tmpInputDir, "link")
	err = os.Symlink(textFile, linkPath)
	require.NoError(t, err)

	info, err := os.Lstat(linkPath)
	require.NoError(t, err)
	assert.Truef(t, IsSymbolicLink(info), "%s should be a symbolic link", info.Name())

	info, err = os.Lstat(textFile)
	require.NoError(t, err)
	assert.Falsef(t, IsSymbolicLink(info), "%s should not be a symbolic link", info.Name())
}

func TestResolveSymlinkFollowsChainsAndDanglingTargets(t *testing.T) {
	dir := resolvedTempDir(t)
	target := filepath.Join(dir, "target.yaml")
	first := filepath.Join(dir, "first")
	second := filepath.Join(dir, "second")
	if err := os.Symlink(filepath.Base(target), first); err != nil {
		t.Skipf("cannot create symbolic link: %v", err)
	}
	if err := os.Symlink(filepath.Base(first), second); err != nil {
		t.Skipf("cannot create symbolic link: %v", err)
	}

	resolved, err := ResolveSymlink(second)
	require.NoError(t, err)
	assert.Equal(t, target, resolved)
}

func TestResolveSymlinkRejectsLoops(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first")
	second := filepath.Join(dir, "second")
	if err := os.Symlink(filepath.Base(second), first); err != nil {
		t.Skipf("cannot create symbolic link: %v", err)
	}
	if err := os.Symlink(filepath.Base(first), second); err != nil {
		t.Skipf("cannot create symbolic link: %v", err)
	}

	_, err := ResolveSymlink(first)
	require.ErrorContains(t, err, "too many levels of symbolic links")
}

func TestResolveSymlinkUsesRealParentForRelativeTarget(t *testing.T) {
	root := resolvedTempDir(t)
	realParent := filepath.Join(root, "real", "deep")
	require.NoError(t, os.MkdirAll(realParent, 0700))
	linkedParent := filepath.Join(root, "linked")
	if err := os.Symlink(realParent, linkedParent); err != nil {
		t.Skipf("cannot create symbolic link: %v", err)
	}
	reportLink := filepath.Join(linkedParent, "report.yaml")
	if err := os.Symlink("../target.yaml", reportLink); err != nil {
		t.Skipf("cannot create symbolic link: %v", err)
	}

	resolved, err := ResolveSymlink(reportLink)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "real", "target.yaml"), resolved)
}

func TestResolveSymlinkHandlesAbsoluteTargetThroughSymlink(t *testing.T) {
	root := resolvedTempDir(t)
	realParent := filepath.Join(root, "real", "deep")
	require.NoError(t, os.MkdirAll(realParent, 0700))
	linkedComponent := filepath.Join(root, "linked-component")
	if err := os.Symlink(realParent, linkedComponent); err != nil {
		t.Skipf("cannot create symbolic link: %v", err)
	}
	reportLink := filepath.Join(root, "report.yaml")
	target := linkedComponent + string(os.PathSeparator) + ".." +
		string(os.PathSeparator) + "target.yaml"
	if err := os.Symlink(target, reportLink); err != nil {
		t.Skipf("cannot create symbolic link: %v", err)
	}

	resolved, err := ResolveSymlink(reportLink)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "real", "target.yaml"), resolved)
}

func TestResolveSymlinkRejectsMissingComponentBeforeParentTraversal(t *testing.T) {
	root := t.TempDir()
	reportPath := filepath.Join(root, "report.yaml")
	require.NoError(t, os.WriteFile(reportPath, []byte("keep"), 0600))
	link := filepath.Join(root, "report-link.yaml")
	if err := os.Symlink("missing"+string(os.PathSeparator)+".."+string(os.PathSeparator)+"report.yaml", link); err != nil {
		t.Skipf("cannot create symbolic link: %v", err)
	}

	_, err := ResolveSymlink(link)
	require.ErrorContains(t, err, "after a missing component")
}

func TestResolveSymlinkRejectsDirectoryShapedMissingTargets(t *testing.T) {
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
			path := tc.setup(t, t.TempDir())
			_, err := ResolveSymlink(path)
			require.ErrorContains(t, err, "directory path")
		})
	}
}

func TestResolveSymlinkRejectsDirectoryShapedExistingTargets(t *testing.T) {
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
			require.NoError(t, os.WriteFile(filepath.Join(root, "report.yaml"), []byte("keep"), 0600))
			_, err := ResolveSymlink(tc.path(root))
			require.Error(t, err)
		})
	}
}

func TestResolveSymlinkHonorsKernelSymlinkLimit(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	require.NoError(t, os.WriteFile(target, []byte("target"), 0600))
	previous := target
	for i := 0; i < 41; i++ {
		link := filepath.Join(root, "link-"+strconv.Itoa(i))
		if err := os.Symlink(filepath.Base(previous), link); err != nil {
			t.Skipf("cannot create symbolic link: %v", err)
		}
		previous = link
	}
	if _, err := os.Stat(previous); err == nil {
		t.Skip("filesystem accepts more than 40 symlink components")
	}

	_, err := ResolveSymlink(previous)
	require.ErrorContains(t, err, "too many levels of symbolic links")
}
