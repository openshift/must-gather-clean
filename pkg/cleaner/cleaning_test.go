package cleaner

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/omitter"
	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/require"
)

type errOmitter struct {
	contents bool
	err      error
}

func (e *errOmitter) OmitPath(_ string) (bool, error) {
	if !e.contents {
		return false, e.err
	}
	return false, nil
}

func pString(s string) *string {
	return &s
}

func noErrorIpObfuscator(t *testing.T) obfuscator.ReportingObfuscator {
	ipObfuscator, err := obfuscator.NewIPObfuscator(schema.ObfuscateReplacementTypeStatic)
	require.NoError(t, err)
	return ipObfuscator
}

func noErrorK8sSecretOmitter(t *testing.T) omitter.KubernetesResourceOmitter {
	resourceOmitter, err := omitter.NewKubernetesResourceOmitter(pString("v1"), pString("Secret"), nil)
	require.NoError(t, err)
	return resourceOmitter
}

// TODO(thomas): this is more of an end2end test, we should do more unit testing of the specific methods
func TestCleaner(t *testing.T) {
	for _, tc := range []struct {
		name             string
		output           string
		input            string
		obfuscators      []obfuscator.ReportingObfuscator
		fileOmitters     []omitter.FileOmitter
		k8sOmitters      []omitter.KubernetesResourceOmitter
		expectedOmission bool
		err              error
	}{
		{
			name:         "simple",
			input:        "test",
			output:       "test\n",
			obfuscators:  []obfuscator.ReportingObfuscator{obfuscator.NoopObfuscator{}},
			fileOmitters: []omitter.FileOmitter{},
			k8sOmitters:  []omitter.KubernetesResourceOmitter{},
		},
		{
			name:         "simple ip obfuscation",
			input:        "there is some cow on 192.178.1.2, what do I do?",
			output:       "there is some cow on xxx.xxx.xxx.xxx, what do I do?\n",
			obfuscators:  []obfuscator.ReportingObfuscator{noErrorIpObfuscator(t)},
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
			output: `apiVersion: v1
kind: Secret
metadata:
    namespace: kube-system
    name: xxx.xxx.xxx.xxx
`,
			obfuscators:  []obfuscator.ReportingObfuscator{noErrorIpObfuscator(t)},
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
			output: `apiVersion: v1
kind: Secret
metadata:
    namespace: kube-system
    name: just a name
    xxx.xxx.xxx.xxx: as a key? unheard of in the land of dns names
`,
			obfuscators:  []obfuscator.ReportingObfuscator{noErrorIpObfuscator(t)},
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
			obfuscators:      []obfuscator.ReportingObfuscator{noErrorIpObfuscator(t)},
			fileOmitters:     []omitter.FileOmitter{},
			k8sOmitters:      []omitter.KubernetesResourceOmitter{noErrorK8sSecretOmitter(t)},
			output:           "",
			expectedOmission: true,
		},
		{
			name:        "error omitters",
			obfuscators: []obfuscator.ReportingObfuscator{obfuscator.NoopObfuscator{}},
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
			tmpInputDir, err := os.MkdirTemp("", "Worker-test-*")
			require.NoError(t, err)
			defer func() {
				_ = os.RemoveAll(tmpInputDir)
			}()

			tmpOutputDir, err := os.MkdirTemp("", "Worker-test-*")
			require.NoError(t, err)
			defer func() {
				_ = os.RemoveAll(tmpOutputDir)
			}()

			const testFileName = "test.yaml"
			if tc.input != "" {
				f, err := os.Create(filepath.Join(tmpInputDir, testFileName))
				require.NoError(t, err)
				_, err = f.Write([]byte(tc.input))
				require.NoError(t, err)
				require.NoError(t, f.Close())
			}

			reportingObfuscator := obfuscator.NewMultiObfuscator(tc.obfuscators)
			multiOmitter := omitter.NewMultiReportingOmitter(tc.fileOmitters, tc.k8sOmitters)
			fileCleaner := NewFileCleaner(tmpInputDir, tmpOutputDir, reportingObfuscator, multiOmitter)

			err = fileCleaner.ProcessFile(testFileName)
			if tc.err != nil {
				require.NotNil(t, err)
				require.Equal(t, tc.err, err)
			} else {
				require.Nil(t, err)
			}

			if tc.output != "" {
				bytes, err := ioutil.ReadFile(filepath.Join(tmpOutputDir, testFileName))
				require.NoError(t, err)
				require.Equal(t, tc.output, string(bytes))
			}

			if tc.expectedOmission {
				require.Contains(t, multiOmitter.Report(), filepath.Join(tmpInputDir, testFileName))
			}
		})
	}

}
