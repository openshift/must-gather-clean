package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/kube"
	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/assert"
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

func TestRunFailsOnNegativeAndZeroWorkers(t *testing.T) {
	err := Run("", "", "", false, "", 0)
	assert.Equal(t, err, fmt.Errorf("invalid number of workers specified %d", 0))
	err = Run("", "", "", false, "", -2)
	assert.Equal(t, err, fmt.Errorf("invalid number of workers specified %d", -2))
}

func TestFailConfigReading(t *testing.T) {
	testDir, err := os.MkdirTemp(os.TempDir(), "test-dir-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(testDir)
	}()

	err = Run("some.yaml", "", testDir, false, "", 1)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestCreateObfuscatorFromFullConfig(t *testing.T) {
	sampleRegex := "^would-match$"
	config := &schema.SchemaJson{Config: schema.SchemaJsonConfig{
		Obfuscate: []schema.Obfuscate{
			{
				Type: schema.ObfuscateTypeKeywords,
				Replacement: map[string]string{
					"something": "something else",
				},
				Target: schema.ObfuscateTargetFileContents,
			},
			{
				Type:            schema.ObfuscateTypeMAC,
				ReplacementType: schema.ObfuscateReplacementTypeStatic,
				Target:          schema.ObfuscateTargetFileContents,
			},
			{
				Type:   schema.ObfuscateTypeRegex,
				Regex:  &sampleRegex,
				Target: schema.ObfuscateTargetFileContents,
			},
			{
				Type:            schema.ObfuscateTypeDomain,
				DomainNames:     []string{"something.com"},
				Target:          schema.ObfuscateTargetFileContents,
				ReplacementType: schema.ObfuscateReplacementTypeStatic,
			},
			{
				Type:            schema.ObfuscateTypeIP,
				ReplacementType: schema.ObfuscateReplacementTypeStatic,
				Target:          schema.ObfuscateTargetFileContents,
			},
		},
		Omit: nil,
	}}

	mfo, err := createObfuscatorsFromConfig(config)
	require.NoError(t, err)
	assert.Equal(t, "something else", mfo.Contents("something"))
}

func TestCreateOmitter(t *testing.T) {
	sampleApiVersion := "v1"
	sampleKind := "Resource"
	sampleRegex := "would-match"

	config := &schema.SchemaJson{Config: schema.SchemaJsonConfig{
		Omit: []schema.Omit{
			{
				Type: schema.OmitTypeKubernetes,
				KubernetesResource: &schema.OmitKubernetesResource{
					ApiVersion: &sampleApiVersion,
					Kind:       &sampleKind,
					Namespaces: []string{"kube-system"},
				}},
			{
				Type:    schema.OmitTypeFile,
				Pattern: &sampleRegex,
			},
		},
	}}

	om, err := createOmittersFromConfig(config)
	require.NoError(t, err)

	match, err := om.OmitPath("would-match")
	require.NoError(t, err)
	assert.Truef(t, match, "'would-match' should match the path omission config")
	match, err = om.OmitPath("would-not-match")
	require.NoError(t, err)
	assert.Falsef(t, match, "'would-not-match' should match the path omission config")

	match, err = om.OmitKubeResource(&kube.ResourceListWithPath{
		ResourceList: kube.ResourceList{
			Items: []kube.Resource{
				{ApiVersion: sampleApiVersion, Kind: sampleKind, Metadata: kube.Metadata{Namespace: "kube-system"}},
			},
		},
		Path: "some-path",
	})
	require.NoError(t, err)
	assert.Truef(t, match, "k8s resource with the exact same input should match")
}

func TestRunPipeNoConfig(t *testing.T) {
	file, err := os.CreateTemp("", "temp-file")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(file.Name())
	}()

	_, err = file.WriteString("some IP 192.167.122.2 that needs to be obfuscated\nand some mac eb:a1:2a:b2:09:bf\n")
	require.NoError(t, err)

	require.NoError(t, file.Close())
	inputFile, err := os.Open(file.Name())
	require.NoError(t, err)
	defer func() {
		_ = inputFile.Close()
	}()

	outputFile, err := os.CreateTemp("", "temp-file")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(outputFile.Name())
	}()

	err = RunPipe("", inputFile, outputFile)
	require.NoError(t, err)
	require.NoError(t, outputFile.Close())

	bytes, err := ioutil.ReadFile(outputFile.Name())
	require.NoError(t, err)

	assert.Equal(t, "some IP x-ipv4-0000000001-x that needs to be obfuscated\nand some mac x-mac-0000000001-x\n", string(bytes))
}

func TestRunPipeConfigMacOnly(t *testing.T) {
	cfgFile, err := os.CreateTemp("", "temp-file-*.yaml")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(cfgFile.Name())
	}()

	_, err = cfgFile.WriteString(`
config:
  obfuscate:
    - type: MAC
      replacementType: Consistent
      target: All
`)
	require.NoError(t, err)
	require.NoError(t, cfgFile.Close())

	file, err := os.CreateTemp("", "temp-file")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(file.Name())
	}()

	_, err = file.WriteString("some IP 192.167.122.2 that should not to be obfuscated\nand some mac eb:a1:2a:b2:09:bf\n")
	require.NoError(t, err)

	require.NoError(t, file.Close())
	inputFile, err := os.Open(file.Name())
	require.NoError(t, err)
	defer func() {
		_ = inputFile.Close()
	}()

	outputFile, err := os.CreateTemp("", "temp-file")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(outputFile.Name())
	}()

	err = RunPipe(cfgFile.Name(), inputFile, outputFile)
	require.NoError(t, err)
	require.NoError(t, outputFile.Close())

	bytes, err := ioutil.ReadFile(outputFile.Name())
	require.NoError(t, err)

	assert.Equal(t, "some IP 192.167.122.2 that should not to be obfuscated\nand some mac x-mac-0000000001-x\n", string(bytes))
}

func TestWaterMarkerNotCreatedOnFail(t *testing.T) {
	testDir, err := os.MkdirTemp(os.TempDir(), "test-dir-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(testDir)
	}()

	err = Run("some.yaml", "", testDir, false, "", 1)
	assert.ErrorIs(t, err, os.ErrNotExist)
	require.NoFileExists(t, filepath.Join(testDir, "watermark.txt"))
}
