package deobfuscator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/reporting"
	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

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
	data, err := yaml.Marshal(report)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "report.yaml")
	require.NoError(t, os.WriteFile(path, data, 0600))

	_, err = LoadReport(path)
	require.ErrorContains(t, err, "replacement group 1 do not match")
}

func TestLoadReportRejectsMetadataFreeSameCountMismatchedGroups(t *testing.T) {
	report := []byte("config:\n  obfuscate:\n    - type: IP\n      replacementType: Consistent\n    - type: AzureResources\n      replacementType: Consistent\nreplacements:\n  - - canonical: azure-resource\n      replacedWith: azure-token\n  - - canonical: 192.0.2.1\n      replacedWith: ip-token\n")
	path := filepath.Join(t.TempDir(), "report.yaml")
	require.NoError(t, os.WriteFile(path, report, 0600))

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
	data, err := yaml.Marshal(report)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "report.yaml")
	require.NoError(t, os.WriteFile(path, data, 0600))

	mapping, err := LoadReport(path)
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
	data, err := yaml.Marshal(report)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "report.yaml")
	require.NoError(t, os.WriteFile(path, data, 0600))

	mapping, err := LoadReport(path)
	require.NoError(t, err)
	require.Equal(t, "192.0.2.1", mapping.Replace("ip-token-a"))
	require.Equal(t, "ip-token-b", mapping.Replace("ip-token-b"))
}
