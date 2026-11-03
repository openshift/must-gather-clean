package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/kube"
	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunFailsOnNegativeAndZeroWorkers(t *testing.T) {
	err := Run("", "", "", false, "", 0)
	assert.Equal(t, fmt.Errorf("invalid number of workers specified %d", 0), err)
	err = Run("", "", "", false, "", -2)
	assert.Equal(t, fmt.Errorf("invalid number of workers specified %d", -2), err)
}

func TestRunFailsOnNotExistingInputPath(t *testing.T) {
	err := Run("", "", "", false, "", 1)
	assert.Equal(t, "input folder does not exist: stat : no such file or directory", err.Error())
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

	mfo, _, err := createObfuscatorsFromConfig(config)
	require.NoError(t, err)
	assert.Equal(t, "something else", mfo.Contents("something"))
}

func TestCreateObfuscatorsSharesTokensAcrossConsistentEntries(t *testing.T) {
	config := &schema.SchemaJson{Config: schema.SchemaJsonConfig{
		Obfuscate: []schema.Obfuscate{
			{
				Type:            schema.ObfuscateTypeIP,
				ReplacementType: schema.ObfuscateReplacementTypeConsistent,
				Target:          schema.ObfuscateTargetFilePath,
			},
			{
				Type:            schema.ObfuscateTypeIP,
				ReplacementType: schema.ObfuscateReplacementTypeConsistent,
				Target:          schema.ObfuscateTargetFileContents,
			},
		},
	}}

	mfo, _, err := createObfuscatorsFromConfig(config)
	require.NoError(t, err)
	assert.Equal(t, "x-ipv4-0000000001-x", mfo.Path("192.0.2.1"))
	assert.Equal(t, "x-ipv4-0000000002-x", mfo.Contents("198.51.100.7"))

	reports := mfo.ReportPerObfuscator()
	require.Len(t, reports, 2)
	assert.Equal(t, "x-ipv4-0000000001-x", reports[0].Replacements[0].ReplacedWith)
	assert.Equal(t, "x-ipv4-0000000002-x", reports[1].Replacements[0].ReplacedWith)
}

func TestCreateObfuscatorsRejectsTokenSharedByConfiguredValues(t *testing.T) {
	config := &schema.SchemaJson{Config: schema.SchemaJsonConfig{
		Obfuscate: []schema.Obfuscate{{
			Type:            schema.ObfuscateTypeIP,
			ReplacementType: schema.ObfuscateReplacementTypeConsistent,
			Target:          schema.ObfuscateTargetFileContents,
			Replacement: map[string]string{
				"192.0.2.1":    "x-ipv4-0000000001-x",
				"198.51.100.7": "x-ipv4-0000000001-x",
			},
		}},
	}}

	_, _, err := createObfuscatorsFromConfig(config)
	require.ErrorContains(t, err, "x-ipv4-0000000001-x")
}

func TestCreateObfuscatorsAllowsTokenSharedByIndependentGroups(t *testing.T) {
	config := &schema.SchemaJson{Config: schema.SchemaJsonConfig{
		Obfuscate: []schema.Obfuscate{
			{
				Type:            schema.ObfuscateTypeIP,
				ReplacementType: schema.ObfuscateReplacementTypeConsistent,
				Target:          schema.ObfuscateTargetFilePath,
				Replacement:     map[string]string{"192.0.2.1": "x-ipv4-0000000001-x"},
			},
			{
				Type:            schema.ObfuscateTypeIP,
				ReplacementType: schema.ObfuscateReplacementTypeConsistent,
				Target:          schema.ObfuscateTargetFileContents,
				Replacement:     map[string]string{"198.51.100.7": "x-ipv4-0000000001-x"},
			},
		},
	}}

	mfo, _, err := createObfuscatorsFromConfig(config)
	require.NoError(t, err)
	assert.Equal(t, "x-ipv4-0000000001-x", mfo.Path("192.0.2.1"))
	assert.Equal(t, "x-ipv4-0000000001-x", mfo.Contents("198.51.100.7"))
}

func TestRunReplaysSharedTokenAcrossIndependentGroups(t *testing.T) {
	tempDir := t.TempDir()
	inputDir := filepath.Join(tempDir, "input")
	outputDir := filepath.Join(tempDir, "output")
	reportDir := filepath.Join(tempDir, "report")
	require.NoError(t, os.Mkdir(inputDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(inputDir, "192.0.2.1.txt"),
		[]byte("second=198.51.100.7 new=203.0.113.9\n"),
		0644,
	))

	configPath := filepath.Join(tempDir, "report.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
config:
  obfuscate:
    - type: IP
      replacementType: Consistent
      target: FilePath
      replacement:
        192.0.2.1: x-ipv4-0000000001-x
    - type: IP
      replacementType: Consistent
      target: FileContents
      replacement:
        198.51.100.7: x-ipv4-0000000001-x
`), 0644))

	require.NoError(t, Run(configPath, inputDir, outputDir, true, reportDir, 1))
	obfuscated, err := os.ReadFile(filepath.Join(outputDir, "x-ipv4-0000000001-x.txt"))
	require.NoError(t, err)
	require.Equal(t, "second=x-ipv4-0000000001-x new=x-ipv4-0000000002-x\n", string(obfuscated))
}

func TestCreateObfuscatorsAllowsEmptyConfiguredTokens(t *testing.T) {
	config := &schema.SchemaJson{Config: schema.SchemaJsonConfig{
		Obfuscate: []schema.Obfuscate{{
			Type:            schema.ObfuscateTypeIP,
			ReplacementType: schema.ObfuscateReplacementTypeConsistent,
			Replacement: map[string]string{
				"192.0.2.1":    "",
				"198.51.100.7": "",
			},
		}},
	}}

	mfo, _, err := createObfuscatorsFromConfig(config)
	require.NoError(t, err)
	reports := mfo.ReportPerObfuscator()
	require.Len(t, reports, 1)
	require.Len(t, reports[0].Replacements, 2)
	for _, replacement := range reports[0].Replacements {
		assert.Empty(t, replacement.ReplacedWith)
	}
}

func TestCreateObfuscatorsKeepsKeywordKeysLiteral(t *testing.T) {
	config := &schema.SchemaJson{Config: schema.SchemaJsonConfig{
		Obfuscate: []schema.Obfuscate{{
			Type:        schema.ObfuscateTypeKeywords,
			Target:      schema.ObfuscateTargetFileContents,
			Replacement: map[string]string{"192-0-2-1": "redacted"},
		}},
	}}

	obfuscator, _, err := createObfuscatorsFromConfig(config)
	require.NoError(t, err)
	assert.Equal(t, "redacted 192.0.2.1", obfuscator.Contents("192-0-2-1 192.0.2.1"))
}

func TestCreateObfuscatorsRejectsConflictingAliasTokens(t *testing.T) {
	config := &schema.SchemaJson{Config: schema.SchemaJsonConfig{
		Obfuscate: []schema.Obfuscate{{
			Type:            schema.ObfuscateTypeIP,
			ReplacementType: schema.ObfuscateReplacementTypeConsistent,
			Replacement: map[string]string{
				"192.0.2.1": "token-a",
				"192-0-2-1": "token-b",
			},
		}},
	}}

	_, _, err := createObfuscatorsFromConfig(config)
	require.ErrorContains(t, err, "conflicting existing replacements")
}

func TestCreateObfuscatorsRejectsInvalidIPReplacementKey(t *testing.T) {
	config := &schema.SchemaJson{Config: schema.SchemaJsonConfig{
		Obfuscate: []schema.Obfuscate{{
			Type:            schema.ObfuscateTypeIP,
			ReplacementType: schema.ObfuscateReplacementTypeConsistent,
			Replacement:     map[string]string{"not-an-ip": "token"},
		}},
	}}

	_, _, err := createObfuscatorsFromConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not-an-ip")
}

func TestRunCanonicalizesPlausibleIPReplacementKeyAndRoundTrips(t *testing.T) {
	tempDir := t.TempDir()
	inputDir := filepath.Join(tempDir, "input")
	outputDir := filepath.Join(tempDir, "output")
	reportDir := filepath.Join(tempDir, "report")
	require.NoError(t, os.Mkdir(inputDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "value.txt"), []byte("value=192.0.2.1\n"), 0644))

	configPath := filepath.Join(tempDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
config:
  obfuscate:
    - type: IP
      replacementType: Consistent
      target: All
      replacement:
        192-0-2-1: x-ipv4-0000000001-x
`), 0644))

	require.NoError(t, Run(configPath, inputDir, outputDir, true, reportDir, 1))
	obfuscated, err := os.ReadFile(filepath.Join(outputDir, "value.txt"))
	require.NoError(t, err)
	require.Equal(t, "value=x-ipv4-0000000001-x\n", string(obfuscated))

	var restored bytes.Buffer
	require.NoError(t, RunDeobfuscate(
		filepath.Join(reportDir, reportFileName),
		bytes.NewReader(obfuscated),
		&restored,
	))
	assert.Equal(t, "value=192.0.2.1\n", restored.String())
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

	om, err := createOmittersFromConfig(config, "")
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

	bytes, err := os.ReadFile(outputFile.Name())
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

	bytes, err := os.ReadFile(outputFile.Name())
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
