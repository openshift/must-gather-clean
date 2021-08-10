package obfuscator

import (
	"fmt"
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
		m.ReplacementReporter.UpsertReplacement(match, StaticMacReplacement)
	}
	return s
}

func (m *macAddressObfuscator) Report() map[string]string {
	return m.ReplacementReporter.Report()
}

func NewMacAddressObfuscator() (Obfuscator, error) {
	regex, err := regexp.Compile(`(?:[0-9a-fA-F]([:-])?){12}`)
	if err != nil {
		return nil, fmt.Errorf("failed to compile mac address regex: %w", err)
	}

	reporter := NewSimpleReporter()
	return &macAddressObfuscator{
		ReplacementReporter: reporter,
		regex:               regex,
	}, nil
}
