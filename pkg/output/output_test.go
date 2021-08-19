package output

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFSWriter(t *testing.T) {
	for _, tc := range []struct {
		name         string
		writeActions []struct {
			parent    string
			name      string
			contents  []string
			expectErr string
		}
		expectedFiles []struct {
			path     string
			contents string
		}
	}{
		{
			name: "simple-operations",
			writeActions: []struct {
				parent    string
				name      string
				contents  []string
				expectErr string
			}{
				{parent: "abc/def/ghi", name: "jkw", contents: []string{"file-contents-1", "file-contents-2"}},
			},
			expectedFiles: []struct {
				path     string
				contents string
			}{
				{"abc/def/ghi/jkw", "file-contents-1\nfile-contents-2\n"},
			},
		},
		{
			name: "invalid write",
			writeActions: []struct {
				parent    string
				name      string
				contents  []string
				expectErr string
			}{
				{parent: "abc/def/ghi", name: "jkw", contents: []string{"file-contents-1", "file-contents-2"}},
				{parent: "abc/def/ghi/jkw", name: "invalidfile", contents: []string{"file-contents-1", "file-contents-2"}, expectErr: "jkw: not a directory"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tempdir, err := os.MkdirTemp(os.TempDir(), "test-*")
			require.NoError(t, err)
			defer os.RemoveAll(tempdir)
			writer, err := NewFSWriter(tempdir)
			require.NoError(t, err)
			for _, a := range tc.writeActions {
				func() {
					t.Helper()
					closeWriter, writer, err := writer.Writer(a.parent, a.name, 0700)
					if a.expectErr == "" {
						require.NoError(t, err)
						defer func() {
							require.NoError(t, closeWriter())
						}()
						for _, c := range a.contents {
							_, err = writer.WriteString(fmt.Sprintf("%s\n", c))
							require.NoError(t, err)
						}
					} else {
						require.Error(t, err)
						require.Contains(t, err.Error(), a.expectErr)
					}
				}()
			}
			for _, f := range tc.expectedFiles {
				fPath := filepath.Join(tempdir, f.path)
				contents, err := ioutil.ReadFile(fPath)
				require.NoError(t, err)
				require.Equal(t, f.contents, string(contents))
			}
		})
	}
}

func TestEnsureOutputPathNonEmptyDir(t *testing.T) {
	testDir, err := os.MkdirTemp(os.TempDir(), "test-dir-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(testDir)
	}()
	err = ioutil.WriteFile(filepath.Join(testDir, "nonempty"), []byte("nonempty"), 0644)
	require.NoError(t, err)
	err = EnsureOutputPath(testDir, false)
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
	err = EnsureOutputPath(file.Name(), false)
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
	err = EnsureOutputPath(outputDir, false)
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

	err = EnsureOutputPath(testDir, true)
	require.NoError(t, err)

	info, err := os.Stat(testDir)
	require.NoError(t, err)
	require.True(t, info.IsDir())

	_, err = os.Stat(toBeDeletedFile)
	require.Error(t, err)
	require.Truef(t, os.IsNotExist(err), "file %s exists even though it shouldn't", toBeDeletedFile)
}
