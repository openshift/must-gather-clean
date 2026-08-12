package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidFiles(t *testing.T) {
	glob, err := filepath.Glob("testfiles/valid/*")
	assert.Nil(t, err)

	for _, validFile := range glob {
		_, err := ReadConfigFromPath(validFile)
		assert.Nilf(t, err, "unexpected error while reading valid config in %s", validFile)
	}
}

func TestInvalidFiles(t *testing.T) {
	glob, err := filepath.Glob("testfiles/malformed/*")
	assert.Nil(t, err)

	for _, invalidFile := range glob {
		_, err := ReadConfigFromPath(invalidFile)
		assert.NotNilf(t, err, "expected error while reading malformed config in %s", invalidFile)
	}
}

func TestFailsOnUnsupportedExtension(t *testing.T) {
	_, err := ReadConfigFromPath("schema_test.go")
	assert.Equal(t, wrapError(UnsupportedFileTypeError{UsedExtension: ".go", SupportedExtensions: supportedExtensions}), err)
}

func TestUnsupportedFileTypeErrorMessage(t *testing.T) {
	err := UnsupportedFileTypeError{UsedExtension: ".txt", SupportedExtensions: []string{".json", ".yaml", ".yml"}}
	assert.Contains(t, err.Error(), ".txt")
	assert.Contains(t, err.Error(), ".json")
	assert.Contains(t, err.Error(), ".yaml")
	assert.Contains(t, err.Error(), ".yml")
}

func TestReadConfigFromPathNonExistentFile(t *testing.T) {
	_, err := ReadConfigFromPath("testfiles/does_not_exist.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config-read:")
}

func TestReadConfigFromPathYmlExtension(t *testing.T) {
	dir := t.TempDir()
	ymlFile := filepath.Join(dir, "config.yml")
	content := []byte(`config:
  obfuscate:
    - type: MAC
`)
	require.NoError(t, os.WriteFile(ymlFile, content, 0644))

	schema, err := ReadConfigFromPath(ymlFile)
	require.NoError(t, err)
	assert.Equal(t, ObfuscateTypeMAC, schema.Config.Obfuscate[0].Type)
}

func TestReadConfigFromPathValidJSON(t *testing.T) {
	dir := t.TempDir()
	jsonFile := filepath.Join(dir, "config.json")
	content := []byte(`{
  "config": {
    "obfuscate": [
      {"type": "IP", "replacementType": "Consistent"}
    ]
  }
}`)
	require.NoError(t, os.WriteFile(jsonFile, content, 0644))

	schema, err := ReadConfigFromPath(jsonFile)
	require.NoError(t, err)
	assert.Equal(t, ObfuscateTypeIP, schema.Config.Obfuscate[0].Type)
	assert.Equal(t, ObfuscateReplacementTypeConsistent, schema.Config.Obfuscate[0].ReplacementType)
}

func TestReadConfigFromPathMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	jsonFile := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(jsonFile, []byte(`{not valid json`), 0644))

	_, err := ReadConfigFromPath(jsonFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config-read:")
}

func TestReadConfigFromPathMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	yamlFile := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte(":\n  :\n  - [invalid"), 0644))

	_, err := ReadConfigFromPath(yamlFile)
	require.Error(t, err)
}

// SchemaJson unmarshal tests

func TestSchemaJsonRequiresConfigField(t *testing.T) {
	var s SchemaJson
	err := s.UnmarshalJSON([]byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config")
}

func TestSchemaJsonConfigEmptyIsValidAtSchemaLevel(t *testing.T) {
	var s SchemaJson
	err := s.UnmarshalJSON([]byte(`{"config": {}}`))
	require.NoError(t, err)
	assert.Empty(t, s.Config.Obfuscate)
}

func TestReadConfigFromPathRejectsEmptyConfig(t *testing.T) {
	dir := t.TempDir()
	jsonFile := filepath.Join(dir, "empty.json")
	require.NoError(t, os.WriteFile(jsonFile, []byte(`{"config": {}}`), 0644))

	_, err := ReadConfigFromPath(jsonFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config is empty")
}

func TestSchemaJsonConfigEmptyObfuscateArray(t *testing.T) {
	var s SchemaJson
	err := s.UnmarshalJSON([]byte(`{"config": {"obfuscate": []}}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "obfuscate")
}

func TestSchemaJsonConfigValidMinimal(t *testing.T) {
	var s SchemaJson
	err := s.UnmarshalJSON([]byte(`{"config": {"obfuscate": [{"type": "IP"}]}}`))
	require.NoError(t, err)
	assert.Len(t, s.Config.Obfuscate, 1)
	assert.Equal(t, ObfuscateTypeIP, s.Config.Obfuscate[0].Type)
}

func TestSchemaJsonConfigWithRandSeed(t *testing.T) {
	var s SchemaJson
	err := s.UnmarshalJSON([]byte(`{"config": {"obfuscate": [{"type": "IP"}], "randSeed": 42}}`))
	require.NoError(t, err)
	require.NotNil(t, s.Config.RandSeed)
	assert.Equal(t, 42, *s.Config.RandSeed)
}

func TestSchemaJsonConfigRandSeedOmitted(t *testing.T) {
	var s SchemaJson
	err := s.UnmarshalJSON([]byte(`{"config": {"obfuscate": [{"type": "IP"}]}}`))
	require.NoError(t, err)
	assert.Nil(t, s.Config.RandSeed)
}

// Obfuscate unmarshal tests

func TestObfuscateRequiresTypeField(t *testing.T) {
	var o Obfuscate
	err := o.UnmarshalJSON([]byte(`{"replacementType": "Static"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type")
}

func TestObfuscateDefaultReplacementType(t *testing.T) {
	var o Obfuscate
	err := o.UnmarshalJSON([]byte(`{"type": "IP"}`))
	require.NoError(t, err)
	assert.Equal(t, ObfuscateReplacementTypeStatic, o.ReplacementType)
}

func TestObfuscateDefaultTarget(t *testing.T) {
	var o Obfuscate
	err := o.UnmarshalJSON([]byte(`{"type": "IP"}`))
	require.NoError(t, err)
	assert.Equal(t, ObfuscateTargetFileContents, o.Target)
}

func TestObfuscateExplicitReplacementTypeAndTarget(t *testing.T) {
	var o Obfuscate
	err := o.UnmarshalJSON([]byte(`{"type": "IP", "replacementType": "Consistent", "target": "All"}`))
	require.NoError(t, err)
	assert.Equal(t, ObfuscateReplacementTypeConsistent, o.ReplacementType)
	assert.Equal(t, ObfuscateTargetAll, o.Target)
}

// ObfuscateType enum tests

func TestObfuscateTypeAllValidValues(t *testing.T) {
	validTypes := []struct {
		input    string
		expected ObfuscateType
	}{
		{`"IP"`, ObfuscateTypeIP},
		{`"MAC"`, ObfuscateTypeMAC},
		{`"Domain"`, ObfuscateTypeDomain},
		{`"Keywords"`, ObfuscateTypeKeywords},
		{`"Regex"`, ObfuscateTypeRegex},
		{`"AzureResources"`, ObfuscateTypeAzureResources},
		{`"Exact"`, ObfuscateTypeExact},
	}

	for _, tc := range validTypes {
		t.Run(string(tc.expected), func(t *testing.T) {
			var ot ObfuscateType
			err := json.Unmarshal([]byte(tc.input), &ot)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, ot)
		})
	}
}

func TestObfuscateTypeInvalid(t *testing.T) {
	var ot ObfuscateType
	err := json.Unmarshal([]byte(`"NotAType"`), &ot)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid value")
}

func TestObfuscateTypeInvalidJSON(t *testing.T) {
	var ot ObfuscateType
	err := json.Unmarshal([]byte(`123`), &ot)
	require.Error(t, err)
}

// ObfuscateTarget enum tests

func TestObfuscateTargetAllValidValues(t *testing.T) {
	validTargets := []struct {
		input    string
		expected ObfuscateTarget
	}{
		{`"FilePath"`, ObfuscateTargetFilePath},
		{`"FileContents"`, ObfuscateTargetFileContents},
		{`"All"`, ObfuscateTargetAll},
	}

	for _, tc := range validTargets {
		t.Run(string(tc.expected), func(t *testing.T) {
			var target ObfuscateTarget
			err := json.Unmarshal([]byte(tc.input), &target)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, target)
		})
	}
}

func TestObfuscateTargetInvalid(t *testing.T) {
	var target ObfuscateTarget
	err := json.Unmarshal([]byte(`"Invalid"`), &target)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid value")
}

func TestObfuscateTargetInvalidJSON(t *testing.T) {
	var target ObfuscateTarget
	err := json.Unmarshal([]byte(`true`), &target)
	require.Error(t, err)
}

// ObfuscateReplacementType enum tests

func TestObfuscateReplacementTypeAllValidValues(t *testing.T) {
	validTypes := []struct {
		input    string
		expected ObfuscateReplacementType
	}{
		{`"Consistent"`, ObfuscateReplacementTypeConsistent},
		{`"Static"`, ObfuscateReplacementTypeStatic},
	}

	for _, tc := range validTypes {
		t.Run(string(tc.expected), func(t *testing.T) {
			var rt ObfuscateReplacementType
			err := json.Unmarshal([]byte(tc.input), &rt)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, rt)
		})
	}
}

func TestObfuscateReplacementTypeInvalid(t *testing.T) {
	var rt ObfuscateReplacementType
	err := json.Unmarshal([]byte(`"Random"`), &rt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid value")
}

func TestObfuscateReplacementTypeInvalidJSON(t *testing.T) {
	var rt ObfuscateReplacementType
	err := json.Unmarshal([]byte(`[]`), &rt)
	require.Error(t, err)
}

// OmitType enum tests

func TestOmitTypeAllValidValues(t *testing.T) {
	validTypes := []struct {
		input    string
		expected OmitType
	}{
		{`"Kubernetes"`, OmitTypeKubernetes},
		{`"File"`, OmitTypeFile},
		{`"SymbolicLink"`, OmitTypeSymbolicLink},
	}

	for _, tc := range validTypes {
		t.Run(string(tc.expected), func(t *testing.T) {
			var ot OmitType
			err := json.Unmarshal([]byte(tc.input), &ot)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, ot)
		})
	}
}

func TestOmitTypeInvalid(t *testing.T) {
	var ot OmitType
	err := json.Unmarshal([]byte(`"BadType"`), &ot)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid value")
}

func TestOmitTypeInvalidJSON(t *testing.T) {
	var ot OmitType
	err := json.Unmarshal([]byte(`999`), &ot)
	require.Error(t, err)
}

// Omit unmarshal tests

func TestOmitRequiresTypeField(t *testing.T) {
	var o Omit
	err := o.UnmarshalJSON([]byte(`{"pattern": "*.log"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type")
}

func TestOmitInvalidJSON(t *testing.T) {
	var o Omit
	err := o.UnmarshalJSON([]byte(`not json`))
	require.Error(t, err)
}

func TestOmitFileType(t *testing.T) {
	var o Omit
	err := o.UnmarshalJSON([]byte(`{"type": "File", "pattern": "*.log"}`))
	require.NoError(t, err)
	assert.Equal(t, OmitTypeFile, o.Type)
	require.NotNil(t, o.Pattern)
	assert.Equal(t, "*.log", *o.Pattern)
}

func TestOmitKubernetesType(t *testing.T) {
	var o Omit
	err := o.UnmarshalJSON([]byte(`{
		"type": "Kubernetes",
		"kubernetesResource": {
			"kind": "Secret",
			"apiVersion": "v1",
			"namespaces": ["default", "kube-system"]
		}
	}`))
	require.NoError(t, err)
	assert.Equal(t, OmitTypeKubernetes, o.Type)
	require.NotNil(t, o.KubernetesResource)
	require.NotNil(t, o.KubernetesResource.Kind)
	assert.Equal(t, "Secret", *o.KubernetesResource.Kind)
	require.NotNil(t, o.KubernetesResource.ApiVersion)
	assert.Equal(t, "v1", *o.KubernetesResource.ApiVersion)
	assert.Equal(t, []string{"default", "kube-system"}, o.KubernetesResource.Namespaces)
}

func TestOmitSymbolicLinkType(t *testing.T) {
	var o Omit
	err := o.UnmarshalJSON([]byte(`{"type": "SymbolicLink"}`))
	require.NoError(t, err)
	assert.Equal(t, OmitTypeSymbolicLink, o.Type)
}

func TestOmitKubernetesNamespacesOnly(t *testing.T) {
	var o Omit
	err := o.UnmarshalJSON([]byte(`{
		"type": "Kubernetes",
		"kubernetesResource": {
			"namespaces": ["openshift-node"]
		}
	}`))
	require.NoError(t, err)
	assert.Nil(t, o.KubernetesResource.Kind)
	assert.Nil(t, o.KubernetesResource.ApiVersion)
	assert.Equal(t, []string{"openshift-node"}, o.KubernetesResource.Namespaces)
}

// ObfuscateExactReplacementsElem tests

func TestExactReplacementRequiresOriginal(t *testing.T) {
	var e ObfuscateExactReplacementsElem
	err := e.UnmarshalJSON([]byte(`{"replacement": "bar"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "original")
}

func TestExactReplacementRequiresReplacement(t *testing.T) {
	var e ObfuscateExactReplacementsElem
	err := e.UnmarshalJSON([]byte(`{"original": "foo"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "replacement")
}

func TestExactReplacementValid(t *testing.T) {
	var e ObfuscateExactReplacementsElem
	err := e.UnmarshalJSON([]byte(`{"original": "foo", "replacement": "bar"}`))
	require.NoError(t, err)
	assert.Equal(t, "foo", e.Original)
	assert.Equal(t, "bar", e.Replacement)
}

func TestExactReplacementInvalidJSON(t *testing.T) {
	var e ObfuscateExactReplacementsElem
	err := e.UnmarshalJSON([]byte(`broken`))
	require.Error(t, err)
}

// Full config integration tests

func TestFullConfigAllObfuscateTypes(t *testing.T) {
	input := `{
		"config": {
			"obfuscate": [
				{"type": "IP", "replacementType": "Consistent", "target": "All"},
				{"type": "MAC"},
				{"type": "Domain", "domainNames": ["example.com", "test.example.com"]},
				{"type": "Keywords", "replacement": {"secret": "redacted"}},
				{"type": "Regex", "regex": "password=.*", "target": "FileContents"},
				{"type": "AzureResources"},
				{"type": "Exact", "exactReplacements": [{"original": "foo", "replacement": "bar"}]}
			]
		}
	}`
	var s SchemaJson
	err := s.UnmarshalJSON([]byte(input))
	require.NoError(t, err)
	assert.Len(t, s.Config.Obfuscate, 7)

	ip := s.Config.Obfuscate[0]
	assert.Equal(t, ObfuscateTypeIP, ip.Type)
	assert.Equal(t, ObfuscateReplacementTypeConsistent, ip.ReplacementType)
	assert.Equal(t, ObfuscateTargetAll, ip.Target)

	mac := s.Config.Obfuscate[1]
	assert.Equal(t, ObfuscateTypeMAC, mac.Type)
	assert.Equal(t, ObfuscateReplacementTypeStatic, mac.ReplacementType)
	assert.Equal(t, ObfuscateTargetFileContents, mac.Target)

	domain := s.Config.Obfuscate[2]
	assert.Equal(t, ObfuscateTypeDomain, domain.Type)
	assert.Equal(t, []string{"example.com", "test.example.com"}, domain.DomainNames)

	keywords := s.Config.Obfuscate[3]
	assert.Equal(t, ObfuscateTypeKeywords, keywords.Type)
	assert.Equal(t, "redacted", keywords.Replacement["secret"])

	regex := s.Config.Obfuscate[4]
	assert.Equal(t, ObfuscateTypeRegex, regex.Type)
	require.NotNil(t, regex.Regex)
	assert.Equal(t, "password=.*", *regex.Regex)
	assert.Equal(t, ObfuscateTargetFileContents, regex.Target)

	azure := s.Config.Obfuscate[5]
	assert.Equal(t, ObfuscateTypeAzureResources, azure.Type)

	exact := s.Config.Obfuscate[6]
	assert.Equal(t, ObfuscateTypeExact, exact.Type)
	require.Len(t, exact.ExactReplacements, 1)
	assert.Equal(t, "foo", exact.ExactReplacements[0].Original)
	assert.Equal(t, "bar", exact.ExactReplacements[0].Replacement)
}

func TestFullConfigAllOmitTypes(t *testing.T) {
	input := `{
		"config": {
			"obfuscate": [{"type": "IP"}],
			"omit": [
				{"type": "Kubernetes", "kubernetesResource": {"kind": "Secret"}},
				{"type": "File", "pattern": "*.log"},
				{"type": "SymbolicLink"}
			]
		}
	}`
	var s SchemaJson
	err := s.UnmarshalJSON([]byte(input))
	require.NoError(t, err)
	assert.Len(t, s.Config.Omit, 3)
	assert.Equal(t, OmitTypeKubernetes, s.Config.Omit[0].Type)
	assert.Equal(t, OmitTypeFile, s.Config.Omit[1].Type)
	assert.Equal(t, OmitTypeSymbolicLink, s.Config.Omit[2].Type)
}

func TestFullConfigNoOmit(t *testing.T) {
	input := `{"config": {"obfuscate": [{"type": "IP"}]}}`
	var s SchemaJson
	err := s.UnmarshalJSON([]byte(input))
	require.NoError(t, err)
	assert.Empty(t, s.Config.Omit)
}

func TestFullConfigInvalidObfuscateType(t *testing.T) {
	input := `{"config": {"obfuscate": [{"type": "NotReal"}]}}`
	var s SchemaJson
	err := s.UnmarshalJSON([]byte(input))
	require.Error(t, err)
}

func TestFullConfigInvalidOmitType(t *testing.T) {
	input := `{"config": {"obfuscate": [{"type": "IP"}], "omit": [{"type": "Invalid"}]}}`
	var s SchemaJson
	err := s.UnmarshalJSON([]byte(input))
	require.Error(t, err)
}

func TestFullConfigObfuscateAsObjectInsteadOfArray(t *testing.T) {
	input := `{"config": {"obfuscate": {"type": "IP"}}}`
	var s SchemaJson
	err := s.UnmarshalJSON([]byte(input))
	require.Error(t, err)
}

func TestFullConfigViaYAML(t *testing.T) {
	dir := t.TempDir()
	yamlFile := filepath.Join(dir, "full.yaml")
	content := []byte(`config:
  obfuscate:
    - type: IP
      replacementType: Consistent
      target: All
    - type: Keywords
      replacement:
        hello: bye
  omit:
    - type: Kubernetes
      kubernetesResource:
        kind: Secret
        namespaces:
          - default
    - type: File
      pattern: "*.log"
  randSeed: 123
`)
	require.NoError(t, os.WriteFile(yamlFile, content, 0644))

	schema, err := ReadConfigFromPath(yamlFile)
	require.NoError(t, err)

	assert.Len(t, schema.Config.Obfuscate, 2)
	assert.Equal(t, ObfuscateTypeIP, schema.Config.Obfuscate[0].Type)
	assert.Equal(t, ObfuscateReplacementTypeConsistent, schema.Config.Obfuscate[0].ReplacementType)
	assert.Equal(t, ObfuscateTargetAll, schema.Config.Obfuscate[0].Target)

	assert.Equal(t, ObfuscateTypeKeywords, schema.Config.Obfuscate[1].Type)
	assert.Equal(t, "bye", schema.Config.Obfuscate[1].Replacement["hello"])

	assert.Len(t, schema.Config.Omit, 2)
	require.NotNil(t, schema.Config.RandSeed)
	assert.Equal(t, 123, *schema.Config.RandSeed)
}

func TestObfuscateTargetFilePath(t *testing.T) {
	var o Obfuscate
	err := o.UnmarshalJSON([]byte(`{"type": "Regex", "regex": "secret-.*", "target": "FilePath"}`))
	require.NoError(t, err)
	assert.Equal(t, ObfuscateTargetFilePath, o.Target)
}

func TestObfuscateWithMultipleExactReplacements(t *testing.T) {
	input := `{
		"type": "Exact",
		"exactReplacements": [
			{"original": "a", "replacement": "b"},
			{"original": "c", "replacement": "d"},
			{"original": "e", "replacement": "f"}
		]
	}`
	var o Obfuscate
	err := o.UnmarshalJSON([]byte(input))
	require.NoError(t, err)
	assert.Len(t, o.ExactReplacements, 3)
	assert.Equal(t, "a", o.ExactReplacements[0].Original)
	assert.Equal(t, "d", o.ExactReplacements[1].Replacement)
	assert.Equal(t, "e", o.ExactReplacements[2].Original)
}

func TestObfuscateKeywordsReplacement(t *testing.T) {
	input := `{
		"type": "Keywords",
		"replacement": {
			"key1": "val1",
			"key2": "val2"
		}
	}`
	var o Obfuscate
	err := o.UnmarshalJSON([]byte(input))
	require.NoError(t, err)
	assert.Equal(t, ObfuscateTypeKeywords, o.Type)
	assert.Len(t, o.Replacement, 2)
	assert.Equal(t, "val1", o.Replacement["key1"])
	assert.Equal(t, "val2", o.Replacement["key2"])
}

func TestObfuscateDomainNames(t *testing.T) {
	input := `{
		"type": "Domain",
		"domainNames": ["rhcloud.com", "dev.rhcloud.com", "example.org"]
	}`
	var o Obfuscate
	err := o.UnmarshalJSON([]byte(input))
	require.NoError(t, err)
	assert.Equal(t, ObfuscateTypeDomain, o.Type)
	assert.Equal(t, []string{"rhcloud.com", "dev.rhcloud.com", "example.org"}, o.DomainNames)
}

func TestObfuscateInvalidJSON(t *testing.T) {
	var o Obfuscate
	err := o.UnmarshalJSON([]byte(`not json`))
	require.Error(t, err)
}

func TestSchemaJsonInvalidJSON(t *testing.T) {
	var s SchemaJson
	err := s.UnmarshalJSON([]byte(`{`))
	require.Error(t, err)
}

func TestSchemaJsonConfigInvalidJSON(t *testing.T) {
	var c SchemaJsonConfig
	err := c.UnmarshalJSON([]byte(`not json`))
	require.Error(t, err)
}

func TestWrapErrorPreservesOriginal(t *testing.T) {
	original := UnsupportedFileTypeError{UsedExtension: ".xml", SupportedExtensions: supportedExtensions}
	wrapped := wrapError(original)
	assert.Contains(t, wrapped.Error(), "config-read:")
	assert.Contains(t, wrapped.Error(), ".xml")
}
