package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExistingEmptyDir(t *testing.T) {
	testDir, err := os.MkdirTemp(os.TempDir(), "test-dir-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(testDir)
	}()
	err = ensureOutputPath(testDir, false)
	require.NoError(t, err)
}

func TestEnsureOutputPathNonEmptyDir(t *testing.T) {
	testDir, err := os.MkdirTemp(os.TempDir(), "test-dir-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(testDir)
	}()
	err = ioutil.WriteFile(filepath.Join(testDir, "nonempty"), []byte("nonempty"), 0644)
	require.NoError(t, err)
	err = ensureOutputPath(testDir, false)
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
	err = ensureOutputPath(file.Name(), false)
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
	err = ensureOutputPath(outputDir, false)
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
	toBeDeletedFile := filepath.Join(testDir, "nonempty")
	err = ioutil.WriteFile(toBeDeletedFile, []byte("nonempty"), 0644)
	require.NoError(t, err)

	err = ensureOutputPath(testDir, true)
	require.NoError(t, err)

	info, err := os.Stat(testDir)
	require.NoError(t, err)
	require.True(t, info.IsDir())

	_, err = os.Stat(toBeDeletedFile)
	require.Error(t, err)
	require.Truef(t, os.IsNotExist(err), "file %s exists even though it shouldn't", toBeDeletedFile)
}
