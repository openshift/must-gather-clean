package obfuscator

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestDomainObfuscatorContents(t *testing.T) {
	for _, tc := range []struct {
		name    string
		domains []string
		input   []string
		output  []string
		report  map[string]string
	}{
		{
			name:    "basic",
			domains: []string{"redhat.com", "openshift.com", "okd.io"},
			input: []string{
				"received request on openshift.com",
				"received request on https://docs.okd.io",
			},
			output: []string{
				"received request on domain0000001",
				"received request on https://docs.domain0000002",
			},
			report: map[string]string{
				"openshift.com": "domain0000001",
				"docs.okd.io":   "docs.domain0000002",
			},
		},
		{
			name: "subdomains",
			domains: []string{
				"docs.okd.io",
				"cloud.redhat.com",
			},
			input: []string{
				"okd.io",
				"docs.okd.io",
				"cloud.redhat.com",
				"beta.cloud.redhat.com",
			},
			output: []string{
				"okd.io",
				"domain0000001",
				"domain0000002",
				"beta.domain0000002",
			},
			report: map[string]string{
				"docs.okd.io":           "domain0000001",
				"cloud.redhat.com":      "domain0000002",
				"beta.cloud.redhat.com": "beta.domain0000002",
			},
		},
		{
			name:    "multi-level subdomains",
			domains: []string{"test.com", "test.info", "test.org"},
			input: []string{
				"received request on abc.test.com",
				"received request on def.test.info",
				"received request on ghi.abc.test.com",
				"received request on pqr.ghi.abc.test.com",
			},
			output: []string{
				"received request on abc.domain0000001",
				"received request on def.domain0000002",
				"received request on ghi.abc.domain0000001",
				"received request on pqr.ghi.abc.domain0000001",
			},
			report: map[string]string{
				"abc.test.com":         "abc.domain0000001",
				"def.test.info":        "def.domain0000002",
				"ghi.abc.test.com":     "ghi.abc.domain0000001",
				"pqr.ghi.abc.test.com": "pqr.ghi.abc.domain0000001",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewDomainObfuscator(tc.domains)
			require.NoError(t, err)
			for idx, i := range tc.input {
				output := o.Contents(i)
				assert.Equal(t, tc.output[idx], output)
			}
			assert.Equal(t, tc.report, o.ReportingResult())
		})
	}
}

func TestDomainObfuscator_FileName(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   string
		output  string
		domains []string
		report  map[string]string
	}{
		{
			name:    "domain with extension",
			domains: []string{"test.com"},
			input:   "requests.test.com.log",
			output:  "requests.domain0000001.log",
			report: map[string]string{
				"requests.test.com": "requests.domain0000001",
			},
		},
		{
			name:    "non-matching domain",
			domains: []string{"test.com"},
			input:   "report.test",
			output:  "report.test",
			report:  map[string]string{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewDomainObfuscator(tc.domains)
			require.NoError(t, err)
			output := o.FileName(tc.input)
			assert.Equal(t, tc.output, output)
			assert.Equal(t, tc.report, o.ReportingResult())
		})
	}
}

func TestBadDomainInput(t *testing.T) {
	_, err := NewDomainObfuscator([]string{"[mustgather.com"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate regex")
}
