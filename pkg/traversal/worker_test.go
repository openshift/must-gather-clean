package traversal

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openshift/must-gather-clean/pkg/schema"
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

func (e *errOmitter) Omit(_, _ string) (bool, error) {
	if !e.contents {
		return false, e.err
	}
	return false, nil
}

func pString(s string) *string {
	return &s
}

func noErrorIpObfuscator(t *testing.T) obfuscator.Obfuscator {
	ipObfuscator, err := obfuscator.NewIPObfuscator(schema.ObfuscateReplacementTypeStatic)
	require.NoError(t, err)
	return ipObfuscator
}

func noErrorK8sSecretOmitter(t *testing.T) omitter.KubernetesResourceOmitter {
	resourceOmitter, err := omitter.NewKubernetesResourceOmitter(pString("v1"), pString("Secret"), nil)
	require.NoError(t, err)
	return resourceOmitter
}

func TestWorker(t *testing.T) {
	for _, tc := range []struct {
		name             string
		output           string
		input            string
		obfuscators      []obfuscator.Obfuscator
		fileOmitters     []omitter.FileOmitter
		k8sOmitters      []omitter.KubernetesResourceOmitter
		expectedOmission bool
		err              error
	}{
		{
			name:         "simple",
			input:        "test",
			output:       "test\n",
			obfuscators:  []obfuscator.Obfuscator{noopObfuscator{}},
			fileOmitters: []omitter.FileOmitter{},
			k8sOmitters:  []omitter.KubernetesResourceOmitter{},
		},
		{
			name:         "simple ip obfuscation",
			input:        "there is some cow on 192.178.1.2, what do I do?",
			output:       "there is some cow on xxx.xxx.xxx.xxx, what do I do?\n",
			obfuscators:  []obfuscator.Obfuscator{noErrorIpObfuscator(t)},
			fileOmitters: []omitter.FileOmitter{},
			k8sOmitters:  []omitter.KubernetesResourceOmitter{},
		},
		{
			name: "ip obfuscated in yaml when its in a k8s resource",
			input: `apiVersion: v1
kind: Secret
metadata:
    namespace: kube-system
    name: 192.178.1.2
`,
			output:       "apiVersion: v1\nkind: Secret\nmetadata:\n    namespace: kube-system\n    name: xxx.xxx.xxx.xxx\n",
			obfuscators:  []obfuscator.Obfuscator{noErrorIpObfuscator(t)},
			fileOmitters: []omitter.FileOmitter{},
			k8sOmitters:  []omitter.KubernetesResourceOmitter{},
		},
		{
			name: "ip obfuscated in yaml when its in a k8s resource as a key",
			input: `apiVersion: v1
kind: Secret
metadata:
    namespace: kube-system
    name: just a name
    192.178.1.2: as a key? unheard of in the land of dns names
`,
			output:       "apiVersion: v1\nkind: Secret\nmetadata:\n    namespace: kube-system\n    name: just a name\n    xxx.xxx.xxx.xxx: as a key? unheard of in the land of dns names\n",
			obfuscators:  []obfuscator.Obfuscator{noErrorIpObfuscator(t)},
			fileOmitters: []omitter.FileOmitter{},
			k8sOmitters:  []omitter.KubernetesResourceOmitter{},
		},
		{
			name: "ip not obfuscated because secret k8s resource is omitted",
			input: `apiVersion: v1
kind: Secret
metadata:
    namespace: kube-system
    name: 192.178.1.2
`,
			obfuscators:      []obfuscator.Obfuscator{noErrorIpObfuscator(t)},
			fileOmitters:     []omitter.FileOmitter{},
			k8sOmitters:      []omitter.KubernetesResourceOmitter{noErrorK8sSecretOmitter(t)},
			output:           "",
			expectedOmission: true,
		},
		{
			name:        "error omitters",
			obfuscators: []obfuscator.Obfuscator{noopObfuscator{}},
			fileOmitters: []omitter.FileOmitter{
				&errOmitter{
					contents: false,
					err:      errors.New("omitter error"),
				},
			},
			k8sOmitters: []omitter.KubernetesResourceOmitter{},
			err:         errors.New("omitter error"),
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
			w := newWorker(1, tc.obfuscators, tc.fileOmitters, tc.k8sOmitters, workerQueue, o, errorCh)
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

			if tc.expectedOmission {
				require.Contains(t, w.omittedFiles, "test.yaml")
			}
		})
	}

}
