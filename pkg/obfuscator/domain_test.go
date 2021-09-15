package obfuscator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/must-gather-clean/pkg/schema"
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
				"received request on domain0000000001",
				"received request on https://docs.domain0000000002",
			},
			report: map[string]string{
				"openshift.com": "domain0000000001",
				"okd.io":        "domain0000000002",
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
				"domain0000000001",
				"domain0000000002",
				"beta.domain0000000002",
			},
			report: map[string]string{
				"docs.okd.io":      "domain0000000001",
				"cloud.redhat.com": "domain0000000002",
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
				"received request on abc.domain0000000001",
				"received request on def.domain0000000002",
				"received request on ghi.abc.domain0000000001",
				"received request on pqr.ghi.abc.domain0000000001",
			},
			report: map[string]string{
				"test.com":  "domain0000000001",
				"test.info": "domain0000000002",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewDomainObfuscator(tc.domains, schema.ObfuscateReplacementTypeConsistent)
			require.NoError(t, err)
			for idx, i := range tc.input {
				output := o.Contents(i)
				assert.Equal(t, tc.output[idx], output)
			}
			assert.Equal(t, tc.report, o.Report())
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
			output:  "requests.domain0000000001.log",
			report: map[string]string{
				"test.com": "domain0000000001",
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
			o, err := NewDomainObfuscator(tc.domains, schema.ObfuscateReplacementTypeConsistent)
			require.NoError(t, err)
			output := o.Path(tc.input)
			assert.Equal(t, tc.output, output)
			assert.Equal(t, tc.report, o.Report())
		})
	}
}

func TestBadDomainInput(t *testing.T) {
	_, err := NewDomainObfuscator([]string{"[mustgather.com"}, schema.ObfuscateReplacementTypeConsistent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate regex")
}

func TestNoDomainInput(t *testing.T) {
	_, err := NewDomainObfuscator([]string{}, schema.ObfuscateReplacementTypeConsistent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no domainNames supplied for the obfuscation type: Domain")
}

func TestDomainObfuscationStatic(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   []string
		output  []string
		domains []string
		report  map[string]string
	}{
		{
			// These are the test cases with the static domain obfuscation.
			name:    "Domain with extension",
			domains: []string{"test.com"},
			input:   []string{"requests.test.com.log"},
			output:  []string{"requests." + staticDomainReplacement + ".log"},
			report: map[string]string{
				"test.com": staticDomainReplacement,
			},
		},
		{
			name:    "non-matching domain",
			domains: []string{"test.com"},
			input:   []string{"report.test"},
			output:  []string{"report.test"},
			report:  map[string]string{},
		},
		{
			name:    "Multiple-Matching Domains",
			domains: []string{"test.com"},
			input: []string{
				"The first domain is report.test.com and the second domain is example-test.com",
			},
			output: []string{
				"The first domain is report." + staticDomainReplacement + " and the second domain is example-" + staticDomainReplacement,
			},
			report: map[string]string{
				"test.com": staticDomainReplacement,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewDomainObfuscator(tc.domains, schema.ObfuscateReplacementTypeStatic)
			require.NoError(t, err)
			for idx, i := range tc.input {
				output := o.Contents(i)
				assert.Equal(t, tc.output[idx], output)
			}
			assert.Equal(t, tc.report, o.Report())
		})
	}
}
