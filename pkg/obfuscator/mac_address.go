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
	consistentMACTemplate = "xxx-mac-%06d-xxx"
)

type macAddressObfuscator struct {
	ReplacementTracker
	replacementType schema.ObfuscateReplacementType
	regex           *regexp.Regexp
	obfsGenerator   generator
}

func (m *macAddressObfuscator) Path(s string) string {
	return m.Contents(s)
}

func (m *macAddressObfuscator) Contents(s string) string {
	matches := m.regex.FindAllString(s, -1)
	for _, mac := range matches {
		// normalizing the MAC Address string to the Uppercase so as to avoid the duplicate reporting
		match := strings.ToUpper(strings.ReplaceAll(mac, "-", ":"))
		replacement := m.obfsGenerator.generateReplacement(m.replacementType, match, m.ReplacementTracker)
		s = strings.ReplaceAll(s, mac, replacement)
		m.ReplacementTracker.AddReplacement(mac, replacement)
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
	// creating a new generator object
	generator, err := newGenerator(consistentMACTemplate, staticMacReplacement, replacementType)
	if err != nil {
		return nil, err
	}
	return &macAddressObfuscator{
		ReplacementTracker: reporter,
		replacementType:    replacementType,
		regex:              regex,
		obfsGenerator:      *generator,
	}, nil
}
