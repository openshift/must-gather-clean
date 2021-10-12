package fsutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExistingEmptyDir(t *testing.T) {
	testDir, err := os.MkdirTemp(os.TempDir(), "test-dir-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(testDir)
	}()
	stat, err := os.Stat(testDir)
	require.NoError(t, err)

	err = ensureOutputPath(testDir, false, stat)
	require.NoError(t, err)
}

func TestEnsureOutputPathNonEmptyDir(t *testing.T) {
	testDir, err := os.MkdirTemp(os.TempDir(), "test-dir-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(testDir)
	}()
	stat, err := os.Stat(testDir)
	require.NoError(t, err)

	err = ioutil.WriteFile(filepath.Join(testDir, "nonempty"), []byte("nonempty"), 0644)
	require.NoError(t, err)

	err = ensureOutputPath(testDir, false, stat)
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
	stat, err := os.Stat(file.Name())
	require.NoError(t, err)

	err = ensureOutputPath(file.Name(), false, stat)
	require.Error(t, err)
	require.Equal(t, fmt.Errorf("output destination must be a directory: '%s'", file.Name()), err)
}

func TestEnsureOutputPathCreateIfRequired(t *testing.T) {
	testDir, err := os.MkdirTemp("", "test-dir-*")
	require.NoError(t, err)
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(testDir)

	stat, err := os.Stat(testDir)
	require.NoError(t, err)

	outputDir := filepath.Join(testDir, "nonexistent")
	err = ensureOutputPath(outputDir, false, stat)
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
	stat, err := os.Stat(testDir)
	require.NoError(t, err)

	toBeDeletedFile := filepath.Join(testDir, "nonempty")
	err = ioutil.WriteFile(toBeDeletedFile, []byte("nonempty"), 0664)
	require.NoError(t, err)

	err = ensureOutputPath(testDir, true, stat)
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
	info, err := os.Stat(testDir)
	require.NoError(t, err)

	// this creates a parallel folder structure with testDir permissions
	inputFolder := filepath.Join("b", "a", "a")
	require.NoError(t, os.MkdirAll(inputFolder, info.Mode()))

	outputFolder := filepath.Join(testDir, "a", "a", "a")
	require.NoError(t, mkdirAllWithChown(outputFolder, info))

	info, err = os.Stat(inputFolder)
	require.NoError(t, err)
	actualInfo, err := os.Stat(outputFolder)
	require.NoError(t, err)
	assert.Equal(t, info.Mode(), actualInfo.Mode())
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
