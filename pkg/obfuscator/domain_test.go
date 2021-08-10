package obfuscator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDomainObfuscatorContents(t *testing.T) {
	for _, tc := range []struct {
		name   string
		tlds   []string
		input  []string
		output []string
		report map[string]string
	}{
		{
			name: "basic",
			tlds: []string{".com", ".info", ".org"},
			input: []string{
				"received request on test.com",
				"received request on https://test.com",
			},
			output: []string{
				"received request on obfuscated0001.ext",
				"received request on https://obfuscated0001.ext",
			},
			report: map[string]string{"test.com": "obfuscated0001.ext"},
		},
		{
			name: "subdomain",
			tlds: []string{".com", ".info", ".org"},
			input: []string{
				"received request on abc.test.com",
				"received request on def.test.com",
			},
			output: []string{
				"received request on abc.obfuscated0001.ext",
				"received request on def.obfuscated0001.ext",
			},
			report: map[string]string{
				"abc.test.com": "abc.obfuscated0001.ext",
				"def.test.com": "def.obfuscated0001.ext",
			},
		},
		{
			name: "multiple domains",
			tlds: []string{".com", ".info", ".org"},
			input: []string{
				"request to test1.com failed",
				"request to test2.com succeeded",
			},
			output: []string{
				"request to obfuscated0001.ext failed",
				"request to obfuscated0002.ext succeeded",
			},
			report: map[string]string{
				"test1.com": "obfuscated0001.ext",
				"test2.com": "obfuscated0002.ext",
			},
		},
		{
			name: "non tld host",
			tlds: []string{".com", ".info", ".org"},
			input: []string{
				"request to test1.remotehost failed",
				"request to test2.remotehost succeeded",
			},
			output: []string{
				"request to test1.obfuscated0001 failed",
				"request to test2.obfuscated0001 succeeded",
			},
			report: map[string]string{
				"test1.remotehost": "test1.obfuscated0001",
				"test2.remotehost": "test2.obfuscated0001",
			},
		},
		{
			name: "ignore ip addresses",
			input: []string{
				"calling 192.168.1.10 for backlog",
				"calling http://192.168.1.10 for backlog",
			},
			output: []string{
				"calling 192.168.1.10 for backlog",
				"calling http://192.168.1.10 for backlog",
			},
			report: map[string]string{},
		},
		{
			name: "ip address and host",
			tlds: []string{".com"},
			input: []string{
				"received request for https://service.example.com from 192.168.1.20",
			},
			output: []string{
				"received request for https://service.obfuscated0001.ext from 192.168.1.20",
			},
			report: map[string]string{
				"service.example.com": "service.obfuscated0001.ext",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := NewDomainObfuscator(tc.tlds)
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
		name   string
		input  string
		output string
		report map[string]string
	}{
		{
			name:   "domain with extension",
			input:  "test.host1.log",
			output: "test.obfuscated0001.log",
			report: map[string]string{
				"test.host1": "test.obfuscated0001",
			},
		},
		{
			name:   "no domain with extension",
			input:  "node1.log",
			output: "node1.log",
			report: map[string]string{},
		},
		{
			name:   "no domain and no extension",
			input:  "report",
			output: "report",
			report: map[string]string{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := NewDomainObfuscator(nil)
			output := o.FileName(tc.input)
			assert.Equal(t, tc.output, output)
			assert.Equal(t, tc.report, o.ReportingResult())
		})
	}
}
