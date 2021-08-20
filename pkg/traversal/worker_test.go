package traversal

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/omitter"
)

type testInputFile struct {
	relPath string
	dir     string
	t       *testing.T
}

func (t *testInputFile) Path() string {
	return t.relPath
}

func (t *testInputFile) Name() string {
	parts := strings.Split(t.relPath, "/")
	return parts[len(parts)-1]
}

func (t *testInputFile) Permissions() os.FileMode {
	info, err := os.Stat(t.AbsPath())
	require.NoError(t.t, err)
	return info.Mode().Perm()
}

func (t *testInputFile) Scanner() (*bufio.Scanner, func() error, error) {
	p := t.AbsPath()
	f, err := os.Open(p)
	require.NoError(t.t, err)
	return bufio.NewScanner(f), f.Close, nil
}

func (t *testInputFile) AbsPath() string {
	return filepath.Join(t.dir, t.relPath)
}

type errOmitter struct {
	contents bool
	err      error
}

func (e *errOmitter) File(_, _ string) (bool, error) {
	if !e.contents {
		return false, e.err
	}
	return false, nil
}

func (e *errOmitter) Contents(_ string) (bool, error) {
	if e.contents {
		return false, e.err
	}
	return false, nil
}

func TestWorker(t *testing.T) {
	for _, tc := range []struct {
		name        string
		output      string
		input       string
		obfuscators []obfuscator.Obfuscator
		omitter     []omitter.Omitter
		err         error
	}{
		{
			name:        "simple",
			input:       "test",
			output:      "test\n",
			obfuscators: []obfuscator.Obfuscator{noopObfuscator{}},
			omitter:     []omitter.Omitter{},
		},
		{
			name:        "error omitters",
			obfuscators: []obfuscator.Obfuscator{noopObfuscator{}},
			omitter: []omitter.Omitter{&errOmitter{
				contents: false,
				err:      errors.New("omitter error"),
			}},
			err: errors.New("omitter error"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "worker-test-*")
			require.NoError(t, err)
			defer func() {
				_ = os.RemoveAll(tempDir)
			}()

			if tc.input != "" {
				f, err := os.Create(filepath.Join(tempDir, "test.yaml"))
				require.NoError(t, err)
				_, err = f.Write([]byte(tc.input))
				require.NoError(t, err)
				require.NoError(t, f.Close())
			}

			workerQueue := make(chan workerFile, 1)
			errorCh := make(chan error, 1)
			o := testOutputter(t)
			w := newWorker(1, tc.obfuscators, tc.omitter, workerQueue, o, errorCh)
			workerQueue <- workerFile{
				f: &testInputFile{
					relPath: "test.yaml",
					dir:     tempDir,
					t:       t,
				},
			}
			close(workerQueue)
			w.run()

			if tc.output != "" {
				require.Equal(t, tc.output, o.Files["test.yaml"].Contents)
			}

			if tc.err != nil {
				timer := time.NewTimer(time.Second)
				var err error
				select {
				case err = <-errorCh:
				case <-timer.C:
				}
				require.NotNil(t, err)
				require.Equal(t, &fileProcessingError{
					path:  "test.yaml",
					cause: tc.err,
				}, err)
			}
		})
	}

}
