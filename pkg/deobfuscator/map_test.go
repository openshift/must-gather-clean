package deobfuscator

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/reporting"
	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMappingReplacesUnambiguousTokens(t *testing.T) {
	mapping := mappingWithConsistentConfig([][]reporting.Replacement{{
		{Canonical: "10.0.0.1", ReplacedWith: "x-ip-1-x"},
		{Canonical: "cluster.example", ReplacedWith: "x-domain-1-x"},
	}})

	require.Equal(t, "node 10.0.0.1 cluster.example unknown", mapping.Replace("node x-ip-1-x x-domain-1-x unknown"))
}

func TestMappingPrefersTheLongestToken(t *testing.T) {
	mapping := mappingWithConsistentConfig([][]reporting.Replacement{{
		{Canonical: "short", ReplacedWith: "token"},
		{Canonical: "long", ReplacedWith: "token-long"},
	}})

	require.Equal(t, "long short", mapping.Replace("token-long token"))
}

func TestMappingLeavesAmbiguousTokensUnchanged(t *testing.T) {
	mapping := mappingWithConsistentConfig([][]reporting.Replacement{{
		{Canonical: "10.0.0.1", ReplacedWith: "x-static-ip"},
		{Canonical: "10.0.0.2", ReplacedWith: "x-static-ip"},
	}})

	require.Equal(t, "x-static-ip", mapping.Replace("x-static-ip"))
}

func TestMappingDoesNotPartiallyRestoreAnAmbiguousOverlappingToken(t *testing.T) {
	mapping := mappingWithConsistentConfig([][]reporting.Replacement{{
		{Canonical: "10.0.0.1", ReplacedWith: "token"},
		{Canonical: "10.0.0.2", ReplacedWith: "token-long"},
		{Canonical: "10.0.0.3", ReplacedWith: "token-long"},
	}})

	require.Empty(t, mapping)
	require.Equal(t, "token-long", mapping.Replace("token-long"))
}

func TestMappingLeavesUnsupportedTargetUnchanged(t *testing.T) {
	report := reporting.Report{
		Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{{
			Type:            schema.ObfuscateTypeIP,
			ReplacementType: schema.ObfuscateReplacementTypeConsistent,
			Target:          schema.ObfuscateTarget("InvalidTarget"),
		}}},
		Replacements: [][]reporting.Replacement{
			{{Canonical: "10.0.0.1", ReplacedWith: "token"}},
		},
	}

	mapping := newMapping(report)
	require.Empty(t, mapping)
	require.Equal(t, "token", mapping.Replace("token"))
}

func TestLoadReportLeavesUnsupportedTargetTokenUnchanged(t *testing.T) {
	report := []byte("config:\n  obfuscate:\n    - type: IP\n      replacementType: Consistent\n      target: InvalidTarget\nreplacements:\n  - - canonical: 10.0.0.1\n      replacedWith: token\n")
	path := filepath.Join(t.TempDir(), "report.yaml")
	require.NoError(t, writeFile(path, report))

	mapping, err := LoadReport(path)
	require.NoError(t, err)
	require.Empty(t, mapping)
	require.Equal(t, "token", mapping.Replace("token"))
}

func TestMappingSkipsEmptyValues(t *testing.T) {
	mapping := mappingWithConsistentConfig([][]reporting.Replacement{{
		{Canonical: "", ReplacedWith: "token"},
		{Canonical: "original", ReplacedWith: ""},
	}})

	require.Empty(t, mapping)
	require.Equal(t, "token", mapping.Replace("token"))
}

func TestMappingSkipsStaticReplacementEvenWhenItIsUnambiguous(t *testing.T) {
	report := reporting.Report{
		Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{{
			Type:            schema.ObfuscateTypeIP,
			ReplacementType: schema.ObfuscateReplacementTypeStatic,
		}}},
		Replacements: [][]reporting.Replacement{{{
			Canonical:    "10.0.0.1",
			ReplacedWith: "xxx.xxx.xxx.xxx",
		}}},
	}

	mapping := newMapping(report)
	require.Empty(t, mapping)
	require.Equal(t, "xxx.xxx.xxx.xxx", mapping.Replace("xxx.xxx.xxx.xxx"))
}

func TestMappingLeavesAzureResourceTokensUnchanged(t *testing.T) {
	report := reporting.Report{
		Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{{
			Type:            schema.ObfuscateTypeAzureResources,
			ReplacementType: schema.ObfuscateReplacementTypeConsistent,
		}}},
		Replacements: [][]reporting.Replacement{{{
			Canonical:    "cluster.example",
			ReplacedWith: "resource-brave-fox",
		}}},
	}

	mapping := newMapping(report)
	require.Empty(t, mapping)
	require.Equal(t, "resource-brave-fox", mapping.Replace("resource-brave-fox"))
}

func TestMappingSkipsChainedReplacements(t *testing.T) {
	report := reporting.Report{
		Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{
			{Type: schema.ObfuscateTypeIP, ReplacementType: schema.ObfuscateReplacementTypeConsistent},
			{Type: schema.ObfuscateTypeMAC, ReplacementType: schema.ObfuscateReplacementTypeConsistent},
		}},
		Replacements: [][]reporting.Replacement{
			{{Canonical: "original", ReplacedWith: "token-one"}},
			{{Canonical: "token-one", ReplacedWith: "token-two"}},
		},
	}

	mapping := newMapping(report)
	require.Empty(t, mapping)
	require.Equal(t, "token-two token-one", mapping.Replace("token-two token-one"))
}

func TestMappingSkipsTokensConsumedByExactConfiguration(t *testing.T) {
	for _, original := range []string{"token", "prefix-token-suffix"} {
		t.Run(original, func(t *testing.T) {
			report := reporting.Report{
				Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{
					{Type: schema.ObfuscateTypeIP, ReplacementType: schema.ObfuscateReplacementTypeConsistent},
					{Type: schema.ObfuscateTypeExact, ExactReplacements: []schema.ObfuscateExactReplacementsElem{{
						Original: original, Replacement: "literal-output",
					}}},
				}},
				Replacements: [][]reporting.Replacement{
					{{Canonical: "10.0.0.1", ReplacedWith: "token"}},
					{},
				},
			}

			mapping := newMapping(report)
			require.Empty(t, mapping)
			require.Equal(t, "token", mapping.Replace("token"))
		})
	}
}

func TestMappingSkipsAChainThroughAnUnsupportedGroup(t *testing.T) {
	report := reporting.Report{
		Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{
			{Type: schema.ObfuscateTypeIP, ReplacementType: schema.ObfuscateReplacementTypeConsistent},
			{Type: schema.ObfuscateTypeKeywords},
		}},
		Replacements: [][]reporting.Replacement{
			{{Canonical: "original", ReplacedWith: "token-one"}},
			{{Canonical: "token-one", ReplacedWith: "token-two"}},
		},
	}

	mapping := newMapping(report)
	require.Empty(t, mapping)
	require.Equal(t, "token-one token-two", mapping.Replace("token-one token-two"))
}

func TestMappingSkipsAChainThroughAnUnsupportedAzureGroup(t *testing.T) {
	report := reporting.Report{
		Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{
			{Type: schema.ObfuscateTypeAzureResources, ReplacementType: schema.ObfuscateReplacementTypeConsistent},
			{Type: schema.ObfuscateTypeMAC, ReplacementType: schema.ObfuscateReplacementTypeConsistent},
		}},
		Replacements: [][]reporting.Replacement{
			{{Canonical: "original", ReplacedWith: "resource-brave-fox"}},
			{{Canonical: "prefix-resource-brave-fox-suffix", ReplacedWith: "x-mac-0000000001-x"}},
		},
	}

	mapping := newMapping(report)
	require.Empty(t, mapping)
	require.Equal(t, "resource-brave-fox x-mac-0000000001-x", mapping.Replace("resource-brave-fox x-mac-0000000001-x"))
}

func TestMappingSkipsTokensEmbeddedInUnsupportedReplacementOutputs(t *testing.T) {
	report := reporting.Report{
		Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{
			{Type: schema.ObfuscateTypeIP, ReplacementType: schema.ObfuscateReplacementTypeConsistent},
			{Type: schema.ObfuscateTypeKeywords},
		}},
		Replacements: [][]reporting.Replacement{
			{{Canonical: "192.0.2.1", ReplacedWith: "x-ip-0000000001-x"}},
			{{Canonical: "keyword", ReplacedWith: "literal-x-ip-0000000001-x-suffix"}},
		},
	}

	mapping := newMapping(report)
	require.Equal(t, "literal-x-ip-0000000001-x-suffix", mapping.Replace("literal-x-ip-0000000001-x-suffix"))
}

func TestMappingSkipsTokensEmbeddedInExactReplacementOutputs(t *testing.T) {
	report := []byte("config:\n  obfuscate:\n    - type: Exact\n      exactReplacements:\n        - original: keyword\n          replacement: literal-x-ip-0000000001-x-suffix\n    - type: IP\n      replacementType: Consistent\n      replacement: {192.0.2.1: x-ip-0000000001-x}\nreplacements:\n  - []\n  - - canonical: 192.0.2.1\n      replacedWith: x-ip-0000000001-x\n      occurrences: [{original: 192.0.2.1}]\n")
	path := filepath.Join(t.TempDir(), "report.yaml")
	require.NoError(t, writeFile(path, report))

	mapping, err := LoadReport(path)
	require.NoError(t, err)
	require.Equal(t, "literal-x-ip-0000000001-x-suffix", mapping.Replace("literal-x-ip-0000000001-x-suffix"))
}

func TestMappingSkipsChainedReplacementWhoseCanonicalContainsEarlierToken(t *testing.T) {
	report := reporting.Report{
		Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{
			{Type: schema.ObfuscateTypeDomain, ReplacementType: schema.ObfuscateReplacementTypeConsistent, Target: schema.ObfuscateTargetAll},
			{Type: schema.ObfuscateTypeMAC, ReplacementType: schema.ObfuscateReplacementTypeConsistent, Target: schema.ObfuscateTargetAll},
		}},
		Replacements: [][]reporting.Replacement{
			{{Canonical: "rhcloud.com", ReplacedWith: "domain0000000001"}},
			{{Canonical: "cluster.domain0000000001", ReplacedWith: "x-mac-0000000001-x"}},
		},
	}

	mapping := newMapping(report)
	require.Equal(t, "x-mac-0000000001-x", mapping.Replace("x-mac-0000000001-x"))
	require.Equal(t, "domain0000000001", mapping.Replace("domain0000000001"))
	require.Equal(t, "cluster.domain0000000001", mapping.Replace("cluster.domain0000000001"))

	report.Config.Obfuscate[0].Target = schema.ObfuscateTargetFilePath
	report.Config.Obfuscate[1].Target = schema.ObfuscateTargetFileContents
	mapping = newMapping(report)
	require.Equal(t, "cluster.domain0000000001", mapping.Replace("x-mac-0000000001-x"))
	require.Equal(t, "rhcloud.com", mapping.Replace("domain0000000001"))
}

func TestMappingDoesNotPartiallyRestoreOverlappingTokensInSubstringChain(t *testing.T) {
	mapping := mappingWithConsistentConfig([][]reporting.Replacement{
		{{Canonical: "domain.example", ReplacedWith: "domain"}},
		{{Canonical: "cluster.example", ReplacedWith: "domain-token"}},
		{{Canonical: "cluster.domain-token", ReplacedWith: "x-mac-0000000001-x"}},
	})

	require.Empty(t, mapping)
	require.Equal(t, "domain domain-token cluster.domain-token x-mac-0000000001-x", mapping.Replace("domain domain-token cluster.domain-token x-mac-0000000001-x"))
}

func TestLoadReportPreservesSelfChainContainingShorterToken(t *testing.T) {
	report := reporting.Report{
		Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{{
			Type: schema.ObfuscateTypeIP, ReplacementType: schema.ObfuscateReplacementTypeConsistent,
			Replacement: schema.ObfuscateReplacement{"192.0.2.1": "token", "token-long": "token-long"},
		}}},
		Replacements: [][]reporting.Replacement{{
			{Canonical: "192.0.2.1", ReplacedWith: "token", Occurrences: []reporting.Occurrence{{Original: "192.0.2.1"}}},
			{Canonical: "token-long", ReplacedWith: "token-long", Occurrences: []reporting.Occurrence{{Original: "token-long"}}},
		}},
	}
	mapping, err := LoadReport(writeReport(t, report))
	require.NoError(t, err)
	require.Equal(t, "token-long token", mapping.Replace("token-long token"))
}

func TestLoadReportPreservesChainsFromExactOutputs(t *testing.T) {
	for _, original := range []string{"192.0.2.1", "prefix-192.0.2.1-suffix"} {
		for _, target := range []schema.ObfuscateTarget{schema.ObfuscateTargetAll, schema.ObfuscateTargetFileContents, schema.ObfuscateTargetFilePath} {
			t.Run(original+string(target), func(t *testing.T) {
				report := reporting.Report{
					Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{
						{Type: schema.ObfuscateTypeExact, Target: target, ExactReplacements: []schema.ObfuscateExactReplacementsElem{{Original: "keyword", Replacement: "192.0.2.1"}}},
						{Type: schema.ObfuscateTypeIP, Target: target, ReplacementType: schema.ObfuscateReplacementTypeConsistent, Replacement: schema.ObfuscateReplacement{original: "token"}},
					}},
					Replacements: [][]reporting.Replacement{{}, {{Canonical: original, ReplacedWith: "token", Occurrences: []reporting.Occurrence{{Original: original}}}}},
				}
				mapping, err := LoadReport(writeReport(t, report))
				require.NoError(t, err)
				require.Equal(t, "token", mapping.Replace("token"))
			})
		}
	}
}

func TestLoadReportDoesNotChainExactOutputAcrossTargets(t *testing.T) {
	report := reporting.Report{
		Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{
			{Type: schema.ObfuscateTypeExact, Target: schema.ObfuscateTargetFilePath, ExactReplacements: []schema.ObfuscateExactReplacementsElem{{Original: "keyword", Replacement: "192.0.2.1"}}},
			{Type: schema.ObfuscateTypeIP, Target: schema.ObfuscateTargetFileContents, ReplacementType: schema.ObfuscateReplacementTypeConsistent, Replacement: schema.ObfuscateReplacement{"192.0.2.1": "token"}},
		}},
		Replacements: [][]reporting.Replacement{{}, {{Canonical: "192.0.2.1", ReplacedWith: "token", Occurrences: []reporting.Occurrence{{Original: "192.0.2.1"}}}}},
	}
	mapping, err := LoadReport(writeReport(t, report))
	require.NoError(t, err)
	require.Equal(t, "192.0.2.1", mapping.Replace("token"))
}

func TestLoadReportPreservesChainsAcrossCanonicalization(t *testing.T) {
	for _, firstType := range []schema.ObfuscateType{schema.ObfuscateTypeExact, schema.ObfuscateTypeKeywords} {
		for _, tc := range []struct {
			typ                 schema.ObfuscateType
			original, canonical string
		}{
			{schema.ObfuscateTypeMAC, "aa:bb:cc:dd:ee:ff", "AA:BB:CC:DD:EE:FF"},
			{schema.ObfuscateTypeIP, "192-0-2-1", "192.0.2.1"},
		} {
			t.Run(string(firstType)+string(tc.typ), func(t *testing.T) {
				first := schema.Obfuscate{Type: firstType}
				var firstGroup []reporting.Replacement
				if firstType == schema.ObfuscateTypeExact {
					first.ExactReplacements = []schema.ObfuscateExactReplacementsElem{{Original: "keyword", Replacement: tc.original}}
				} else {
					first.Replacement = schema.ObfuscateReplacement{"keyword": tc.original}
					firstGroup = []reporting.Replacement{{Canonical: "keyword", ReplacedWith: tc.original, Occurrences: []reporting.Occurrence{{Original: "keyword"}}}}
				}
				report := reporting.Report{
					Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{
						first,
						{Type: tc.typ, ReplacementType: schema.ObfuscateReplacementTypeConsistent, Replacement: schema.ObfuscateReplacement{tc.original: "token"}},
					}},
					Replacements: [][]reporting.Replacement{firstGroup, {{Canonical: tc.canonical, ReplacedWith: "token", Occurrences: []reporting.Occurrence{{Original: tc.original}}}}},
				}
				mapping, err := LoadReport(writeReport(t, report))
				require.NoError(t, err)
				require.Equal(t, "token", mapping.Replace("token"))
			})
		}
	}
}

func TestMappingReplaceReaderStreamsAndPreservesLineEndings(t *testing.T) {
	mapping := mappingWithConsistentConfig([][]reporting.Replacement{{
		{Canonical: "10.0.0.1", ReplacedWith: "x-ip-1-x"},
		{Canonical: "cluster.example", ReplacedWith: "x-domain-1-x"},
	}})
	input := &chunkReader{reader: strings.NewReader("first x-ip-1-x\r\nsecond x-domain-1-x"), size: 3}
	var output bytes.Buffer

	require.NoError(t, mapping.ReplaceReader(input, &output))
	require.Equal(t, "first 10.0.0.1\r\nsecond cluster.example", output.String())
}

type chunkReader struct {
	reader *strings.Reader
	size   int
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if len(p) > r.size {
		p = p[:r.size]
	}
	return r.reader.Read(p)
}

func TestLoadReport(t *testing.T) {
	report := []byte("config:\n  obfuscate:\n    - type: IP\n      replacementType: Consistent\n      replacement: {original: token}\nreplacements:\n  - - canonical: original\n      replacedWith: token\n      occurrences: [{original: original}]\n")
	path := t.TempDir() + "/report.yaml"
	require.NoError(t, writeFile(path, report))

	mapping, err := LoadReport(path)
	require.NoError(t, err)
	require.Equal(t, "original", mapping.Replace("token"))
}

func TestLoadReportReturnsReadAndParseErrors(t *testing.T) {
	_, err := LoadReport(filepath.Join(t.TempDir(), "missing.yaml"))
	require.ErrorIs(t, err, os.ErrNotExist)

	path := filepath.Join(t.TempDir(), "invalid.yaml")
	require.NoError(t, writeFile(path, []byte("replacements: [")))
	_, err = LoadReport(path)
	require.Error(t, err)
}

func TestLoadReportRejectsReplacementGroupsWithoutConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-report.yaml")
	report := []byte("replacements:\n  - - canonical: original\n      replacedWith: token\n")
	require.NoError(t, writeFile(path, report))

	_, err := LoadReport(path)
	require.ErrorContains(t, err, "obfuscation config and replacement groups do not match")
}

func TestLoadReportRejectsSameCountMismatchedReplacementGroups(t *testing.T) {
	report := reporting.Report{
		Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{
			{
				Type:            schema.ObfuscateTypeIP,
				ReplacementType: schema.ObfuscateReplacementTypeConsistent,
				Replacement:     schema.ObfuscateReplacement{"ip-original": "ip-token"},
			},
			{
				Type:            schema.ObfuscateTypeMAC,
				ReplacementType: schema.ObfuscateReplacementTypeConsistent,
				Replacement:     schema.ObfuscateReplacement{"mac-original": "mac-token"},
			},
		}},
		Replacements: [][]reporting.Replacement{
			{{Canonical: "mac-original", ReplacedWith: "mac-token", Occurrences: []reporting.Occurrence{{Original: "mac-original"}}}},
			{{Canonical: "ip-original", ReplacedWith: "ip-token", Occurrences: []reporting.Occurrence{{Original: "ip-original"}}}},
		},
	}

	_, err := LoadReport(writeReport(t, report))
	require.ErrorContains(t, err, "replacement group 1 do not match")
}

func TestLoadReportRejectsMetadataFreeSameCountMismatchedGroups(t *testing.T) {
	report := []byte("config:\n  obfuscate:\n    - type: IP\n      replacementType: Consistent\n    - type: AzureResources\n      replacementType: Consistent\nreplacements:\n  - - canonical: azure-resource\n      replacedWith: azure-token\n  - - canonical: 192.0.2.1\n      replacedWith: ip-token\n")
	path := filepath.Join(t.TempDir(), "report.yaml")
	require.NoError(t, writeFile(path, report))

	_, err := LoadReport(path)
	require.ErrorContains(t, err, "missing occurrence metadata")
}

func TestLoadReportAcceptsMatchingReplacementGroupConfiguration(t *testing.T) {
	report := reporting.Report{
		Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{{
			Type:            schema.ObfuscateTypeIP,
			ReplacementType: schema.ObfuscateReplacementTypeConsistent,
			Replacement:     schema.ObfuscateReplacement{"ip-original": "ip-token"},
		}}},
		Replacements: [][]reporting.Replacement{{
			{Canonical: "ip-original", ReplacedWith: "ip-token", Occurrences: []reporting.Occurrence{{Original: "ip-original"}}},
		}},
	}

	mapping, err := LoadReport(writeReport(t, report))
	require.NoError(t, err)
	require.Equal(t, "ip-original", mapping.Replace("ip-token"))
}

func TestLoadReportAcceptsMultipleReportedOutputsForOneOccurrence(t *testing.T) {
	report := reporting.Report{
		Config: schema.SchemaJsonConfig{Obfuscate: []schema.Obfuscate{{
			Type:            schema.ObfuscateTypeIP,
			ReplacementType: schema.ObfuscateReplacementTypeConsistent,
			Replacement:     schema.ObfuscateReplacement{"192.0.2.1": "ip-token-a"},
		}}},
		Replacements: [][]reporting.Replacement{{
			{Canonical: "192.0.2.1", ReplacedWith: "ip-token-a", Occurrences: []reporting.Occurrence{{Original: "192.0.2.1"}}},
			{Canonical: "198.51.100.1", ReplacedWith: "ip-token-b", Occurrences: []reporting.Occurrence{{Original: "192.0.2.1"}}},
		}},
	}

	mapping, err := LoadReport(writeReport(t, report))
	require.NoError(t, err)
	require.Equal(t, "192.0.2.1", mapping.Replace("ip-token-a"))
	require.Equal(t, "ip-token-b", mapping.Replace("ip-token-b"))
}

func mappingWithConsistentConfig(groups [][]reporting.Replacement) Mapping {
	obfuscators := make([]schema.Obfuscate, len(groups))
	for i := range obfuscators {
		obfuscators[i] = schema.Obfuscate{
			Type:            schema.ObfuscateTypeIP,
			ReplacementType: schema.ObfuscateReplacementTypeConsistent,
		}
	}
	return newMapping(reporting.Report{
		Config:       schema.SchemaJsonConfig{Obfuscate: obfuscators},
		Replacements: groups,
	})
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0600)
}

func writeReport(t *testing.T, report reporting.Report) string {
	t.Helper()
	data, err := yaml.Marshal(report)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "report.yaml")
	require.NoError(t, writeFile(path, data))
	return path
}
