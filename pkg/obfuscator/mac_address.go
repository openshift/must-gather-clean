package obfuscator

import (
	"regexp"
	"strings"
)

const StaticMacReplacement = "xx:xx:xx:xx:xx:xx"

type macAddressObfuscator struct {
	ReplacementReporter
	regex *regexp.Regexp
}

func (m *macAddressObfuscator) FileName(s string) string {
	return m.Contents(s)
}

func (m *macAddressObfuscator) Contents(s string) string {
	matches := m.regex.FindAllString(s, -1)
	for _, match := range matches {
		s = strings.Replace(s, match, StaticMacReplacement, -1)
		m.ReplacementReporter.ReportReplacement(match, StaticMacReplacement)
	}
	return s
}

func (m *macAddressObfuscator) ReportingResult() map[string]string {
	return m.ReplacementReporter.ReportingResult()
}

func NewMacAddressObfuscator() Obfuscator {
	// this regex differs from the standard `(?:[0-9a-fA-F]([:-])?){12}`, to not match very frequently happening UUIDs in K8s
	// the main culprit is the support for squashed MACs like '69806FE67C05', which won't be supported with the below
	regex := regexp.MustCompile(`([0-9a-fA-F]{2}[:-]){5}[0-9a-fA-F]{2}`)

	reporter := NewSimpleReporter()
	return &macAddressObfuscator{
		ReplacementReporter: reporter,
		regex:               regex,
	}
}
