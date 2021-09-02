package obfuscator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/openshift/must-gather-clean/pkg/schema"
)

const (
	// staticMacReplacement refers to a static replacement for any identified MAC address.
	staticMacReplacement = "xx:xx:xx:xx:xx:xx"
	// consistentMACTemplate refers to a consistent replacement for any identified MAC address
	consistentMACTemplate = "x-mac-%06d-x"
)

type macAddressObfuscator struct {
	ReplacementTracker
	replacementType schema.ObfuscateReplacementType
	regex           *regexp.Regexp
	obfsGenerator   obfsGenerator
}

func (m *macAddressObfuscator) Path(s string) string {
	return m.Contents(s)
}

func (m *macAddressObfuscator) Contents(s string) string {
	matches := m.regex.FindAllString(s, -1)
	for _, match := range matches {
		var replacement string
		switch m.replacementType {
		case schema.ObfuscateReplacementTypeStatic:
			replacement = m.GenerateIfAbsent(match, match, m.obfsGenerator.generateStaticReplacement)
		case schema.ObfuscateReplacementTypeConsistent:
			replacement = m.GenerateIfAbsent(match, match, m.obfsGenerator.generateConsistentReplacement)
		}
		s = strings.Replace(s, match, replacement, -1)
		m.ReplacementTracker.AddReplacement(match, replacement)
	}
	return s
}

func (m *macAddressObfuscator) Report() map[string]string {
	return m.ReplacementTracker.Report()
}

func NewMacAddressObfuscator(replacementType schema.ObfuscateReplacementType) (Obfuscator, error) {
	if replacementType != schema.ObfuscateReplacementTypeStatic && replacementType != schema.ObfuscateReplacementTypeConsistent {
		return nil, fmt.Errorf("unsupported replacement type: %s", replacementType)
	}
	// this regex differs from the standard `(?:[0-9a-fA-F]([:-])?){12}`, to not match very frequently happening UUIDs in K8s
	// the main culprit is the support for squashed MACs like '69806FE67C05', which won't be supported with the below
	regex := regexp.MustCompile(`([0-9a-fA-F]{2}[:-]){5}[0-9a-fA-F]{2}`)

	reporter := NewSimpleTracker()
	generator := obfsGenerator{
		static:   staticMacReplacement,
		template: consistentMACTemplate,
	}
	return &macAddressObfuscator{
		ReplacementTracker: reporter,
		replacementType:    replacementType,
		regex:              regex,
		obfsGenerator:      generator,
	}, nil
}
