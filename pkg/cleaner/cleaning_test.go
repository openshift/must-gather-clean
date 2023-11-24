package cleaner

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/kube"
	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/omitter"
	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errOmitter struct {
	pathResult bool
	pathErr    error

	k8sResult bool
	k8sErr    error
}

func (e *errOmitter) OmitPath(_ string) (bool, error) {
	return e.pathResult, e.pathErr
}

func (e *errOmitter) OmitKubeResource(_ *kube.ResourceListWithPath) (bool, error) {
	return e.k8sResult, e.k8sErr
}

type errWriter struct{}

var UnwritableErr = errors.New("unwritable")

func (errWriter) Write(_ []byte) (n int, err error) {
	return 0, UnwritableErr
}

func pString(s string) *string {
	return &s
}

func noErrorIpObfuscator(t *testing.T) obfuscator.ReportingObfuscator {
	ipObfuscator, err := obfuscator.NewIPObfuscator(schema.ObfuscateReplacementTypeStatic, obfuscator.NewSimpleTracker())
	require.NoError(t, err)
	return ipObfuscator
}

func noErrorK8sSecretOmitter(t *testing.T) omitter.KubernetesResourceOmitter {
	resourceOmitter, err := omitter.NewKubernetesResourceOmitter(pString("v1"), pString("Secret"), nil)
	require.NoError(t, err)
	return resourceOmitter
}

func TestProcessNotExistingFile(t *testing.T) {
	fileCleaner := NewFileCleaner("tmpInputDir", "tmpOutputDir", obfuscator.NoopObfuscator{}, &omitter.NoopOmitter{})
	err := fileCleaner.Process("not-existing.yaml")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestProcessNoK8sResource(t *testing.T) {
	fileCleaner := NewFileCleaner("tmpInputDir", "tmpOutputDir", obfuscator.NoopObfuscator{}, &omitter.NoopOmitter{})
	err := fileCleaner.Process("not-existing.zzzz")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestObfuscateReaderHappyPath(t *testing.T) {
	cf := ContentObfuscator{Obfuscator: noErrorIpObfuscator(t)}

	output := &strings.Builder{}
	err := cf.ObfuscateReader(strings.NewReader(`
this is one line with an ip 10.0.129.220
and another with another ip 192.168.2.11
`), output)
	require.NoError(t, err)
	assert.Equal(t, `
this is one line with an ip xxx.xxx.xxx.xxx
and another with another ip xxx.xxx.xxx.xxx
`, output.String())
}

func TestObfuscateReaderVeryLongLine(t *testing.T) {
	cf := ContentObfuscator{Obfuscator: noErrorIpObfuscator(t)}

	superLongLine := strings.Repeat("192.169.2.11 ++ ", 10000) + "\n"
	superLongLineObfuscated := strings.Repeat("xxx.xxx.xxx.xxx ++ ", 10000) + "\n"
	output := &strings.Builder{}
	err := cf.ObfuscateReader(strings.NewReader(superLongLine), output)
	require.NoError(t, err)
	assert.Equal(t, superLongLineObfuscated, output.String())
}

func TestObfuscateReaderSingleLineNoLineFeed(t *testing.T) {
	cf := ContentObfuscator{Obfuscator: noErrorIpObfuscator(t)}
	output := &strings.Builder{}
	err := cf.ObfuscateReader(strings.NewReader("192.169.2.11"), output)
	require.NoError(t, err)
	assert.Equal(t, "xxx.xxx.xxx.xxx", output.String())
}

func TestObfuscateReaderIOErrorPropagates(t *testing.T) {
	cf := ContentObfuscator{Obfuscator: noErrorIpObfuscator(t)}
	err := cf.ObfuscateReader(strings.NewReader("a line"), &errWriter{})
	require.ErrorIs(t, err, UnwritableErr)
}

func TestObfuscateFileOutputExists(t *testing.T) {
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

	existingFile := "existing.file"
	require.NoError(t, os.WriteFile(filepath.Join(tmpInputDir, existingFile), []byte("hello world"), 0666))
	require.NoError(t, os.WriteFile(filepath.Join(tmpOutputDir, existingFile), []byte("hello world"), 0666))
	fco := &FileContentObfuscator{
		ContentObfuscator: ContentObfuscator{Obfuscator: obfuscator.NoopObfuscator{}},
		inputFolder:       tmpInputDir,
		outputFolder:      tmpOutputDir,
	}

	for i := 0; i < 3; i++ {
		err = fco.ObfuscateFile(existingFile, existingFile)
		require.NoError(t, err)
		// validating if a new file is created with the ascending number pattern extensions appended
		_, err = os.Stat(filepath.Join(tmpOutputDir, existingFile) + "." + strconv.Itoa(i+1))
		require.NoError(t, err)
	}
}

func TestCleanerProcessor(t *testing.T) {
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
			output:       "test",
			obfuscators:  []obfuscator.ReportingObfuscator{obfuscator.NoopObfuscator{}},
			fileOmitters: []omitter.FileOmitter{},
			k8sOmitters:  []omitter.KubernetesResourceOmitter{},
		},
		{
			name:         "simple ip obfuscation",
			input:        "there is some cow on 192.178.1.2, what do I do?",
			output:       "there is some cow on xxx.xxx.xxx.xxx, what do I do?",
			obfuscators:  []obfuscator.ReportingObfuscator{noErrorIpObfuscator(t)},
			fileOmitters: []omitter.FileOmitter{},
			k8sOmitters:  []omitter.KubernetesResourceOmitter{},
		},
		{
			name:         "simple omission by filename",
			input:        "whatever is in there",
			obfuscators:  []obfuscator.ReportingObfuscator{noErrorIpObfuscator(t)},
			fileOmitters: []omitter.FileOmitter{newFilePatternOmitter(t, "test.yaml")},
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
					pathResult: false,
					pathErr:    errors.New("omitter error"),
				},
			},
			k8sOmitters: []omitter.KubernetesResourceOmitter{},
			err:         errors.New("omitter error"),
		},

		{
			name: "error k8s omitter",
			input: `apiVersion: v1
kind: Secret
metadata:
    namespace: kube-system
    name: 192.178.1.2
`,
			obfuscators:  []obfuscator.ReportingObfuscator{obfuscator.NoopObfuscator{}},
			fileOmitters: []omitter.FileOmitter{},
			k8sOmitters: []omitter.KubernetesResourceOmitter{&errOmitter{
				k8sErr: errors.New("omitter error"),
			}},
			err: errors.New("omitter error"),
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

			err = fileCleaner.Process(testFileName)
			if tc.err != nil {
				require.NotNil(t, err)
				require.Equal(t, tc.err, err)
			} else {
				require.Nil(t, err)
			}

			if tc.output != "" {
				bytes, err := os.ReadFile(filepath.Join(tmpOutputDir, testFileName))
				require.NoError(t, err)
				require.Equal(t, tc.output, string(bytes))
			}

			if tc.expectedOmission {
				require.Contains(t, multiOmitter.Report(), filepath.Join(tmpInputDir, testFileName))
			}
		})
	}

}

func newFilePatternOmitter(t *testing.T, pattern string) omitter.FileOmitter {
	o, err := omitter.NewFilenamePatternOmitter(pattern)
	require.NoError(t, err)
	return o
}
