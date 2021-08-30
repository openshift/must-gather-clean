package obfuscator

import (
	"regexp"
	"strings"
)

const StaticMacReplacement = "xx:xx:xx:xx:xx:xx"

type macAddressObfuscator struct {
	ReplacementTracker
	regex *regexp.Regexp
}

func (m *macAddressObfuscator) Path(s string) string {
	return m.Contents(s)
}

func (m *macAddressObfuscator) Contents(s string) string {
	matches := m.regex.FindAllString(s, -1)
	for _, match := range matches {
		s = strings.Replace(s, match, StaticMacReplacement, -1)
		m.ReplacementTracker.AddReplacement(match, StaticMacReplacement)
	}
	return s
}

func (m *macAddressObfuscator) Report() map[string]string {
	return m.ReplacementTracker.Report()
}

func NewMacAddressObfuscator() Obfuscator {
	// this regex differs from the standard `(?:[0-9a-fA-F]([:-])?){12}`, to not match very frequently happening UUIDs in K8s
	// the main culprit is the support for squashed MACs like '69806FE67C05', which won't be supported with the below
	regex := regexp.MustCompile(`([0-9a-fA-F]{2}[:-]){5}[0-9a-fA-F]{2}`)

	reporter := NewSimpleTracker()
	return &macAddressObfuscator{
		ReplacementTracker: reporter,
		regex:              regex,
	}
}
