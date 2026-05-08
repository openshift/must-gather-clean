package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/kube"
	"github.com/openshift/must-gather-clean/pkg/obfuscator"
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

// newSeedableMultiObfuscator builds a MultiObfuscator containing a single
// AzureResourceObfuscator so we can observe what seedObfuscatorsFromInputDir seeds.
func newSeedableMultiObfuscator(t *testing.T) *obfuscator.MultiObfuscator {
	t.Helper()
	seed := 42
	tracker := obfuscator.NewSimpleTracker()
	azureObf, err := obfuscator.NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, tracker, &seed)
	require.NoError(t, err)
	return obfuscator.NewMultiObfuscator([]obfuscator.ReportingObfuscator{azureObf})
}

// writeFiles creates empty files under dir/subdir for each name.
func writeFiles(t *testing.T, dir, subdir string, names []string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, subdir), 0755))
	for _, name := range names {
		require.NoError(t, os.WriteFile(filepath.Join(dir, subdir, name), []byte{}, 0644))
	}
}

// TestSeedObfuscatorsFromInputDir_UsesPreferredDirs verifies that when a preferred
// seed directory ("service" or "cluster") contains valid cluster-named files, its
// cluster prefix is seeded and the fallback directory is not used.
func TestSeedObfuscatorsFromInputDir_UsesPreferredDirs(t *testing.T) {
	dir := t.TempDir()

	// Preferred "service" dir: files yield prefix "dev-qm7v3npl-svc".
	writeFiles(t, dir, "service", []string{
		"dev-qm7v3npl-svc-default-pod.jsonl",
		"dev-qm7v3npl-svc-kube-system-worker.jsonl",
	})
	// Fallback dir: different cluster ID — must NOT be seeded.
	writeFiles(t, dir, "other", []string{
		"stg-xyz9def2-svc-default-pod.jsonl",
		"stg-xyz9def2-svc-kube-system-worker.jsonl",
	})

	mo := newSeedableMultiObfuscator(t)
	seedObfuscatorsFromInputDir(dir, mo)

	// "dev-qm7v3npl-svc" was seeded from the preferred dir and must be replaced.
	out := mo.Contents("log line with dev-qm7v3npl-svc in it")
	assert.NotContains(t, out, "dev-qm7v3npl-svc", "preferred-dir prefix should be obfuscated")

	// The fallback dir's cluster token must NOT have been seeded.
	out2 := mo.Contents("log line with stg-xyz9def2-svc in it")
	assert.Contains(t, out2, "xyz9def2", "fallback dir should not be used when preferred dir yields a prefix")
}

// TestSeedObfuscatorsFromInputDir_FallsBackToOtherDirs verifies that when the
// preferred seed directories yield no valid cluster prefix, the seeder falls back
// to all other subdirectories in the input path.
func TestSeedObfuscatorsFromInputDir_FallsBackToOtherDirs(t *testing.T) {
	dir := t.TempDir()

	// Preferred "service" dir exists but contains no cluster-named files.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "service"), 0755))

	// Fallback dir has valid cluster-named files yielding prefix "stg-xyz9def2-svc".
	writeFiles(t, dir, "fallback", []string{
		"stg-xyz9def2-svc-default-pod.jsonl",
		"stg-xyz9def2-svc-kube-system-worker.jsonl",
	})

	mo := newSeedableMultiObfuscator(t)
	seedObfuscatorsFromInputDir(dir, mo)

	// "stg-xyz9def2-svc" was seeded from the fallback dir and must be replaced.
	out := mo.Contents("log line with stg-xyz9def2-svc in it")
	assert.NotContains(t, out, "stg-xyz9def2-svc", "fallback-dir prefix should be obfuscated when preferred dirs yield nothing")
	assert.NotEqual(t, "log line with stg-xyz9def2-svc in it", out, "output must differ from input — replacement must have occurred")
}

// TestSeedObfuscatorsFromInputDir_SeedsBothServiceAndMgmt verifies that when both
// a "service" and "mgmt" preferred directory contain cluster-named files, BOTH
// prefixes are seeded independently — the mgmt cluster must not be dropped just
// because the service cluster was found first.
func TestSeedObfuscatorsFromInputDir_SeedsBothServiceAndMgmt(t *testing.T) {
	dir := t.TempDir()

	// "service" dir: staging service cluster (tst-northeu-svc-1-*)
	writeFiles(t, dir, "service", []string{
		"tst-northeu-svc-1-default-pod.jsonl",
		"tst-northeu-svc-1-kube-system-worker.jsonl",
	})
	// "mgmt" dir: management cluster with entirely different naming scheme
	writeFiles(t, dir, "mgmt", []string{
		"hcp-underlay-cd-mgmt-1-default-pod.jsonl",
		"hcp-underlay-cd-mgmt-1-kube-system-worker.jsonl",
	})

	mo := newSeedableMultiObfuscator(t)
	seedObfuscatorsFromInputDir(dir, mo)

	// Service cluster prefix must be obfuscated.
	out1 := mo.Contents("log line with tst-northeu-svc-1 in it")
	assert.NotContains(t, out1, "tst-northeu-svc-1", "service cluster prefix must be obfuscated")

	// Management cluster prefix must ALSO be obfuscated.
	out2 := mo.Contents("log line with hcp-underlay-cd-mgmt-1 in it")
	assert.NotContains(t, out2, "hcp-underlay-cd-mgmt-1", "management cluster prefix must be obfuscated")
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
