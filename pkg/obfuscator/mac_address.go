package obfuscator

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/klog/v2"

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
	// count helps in keeping track of the number of MAC obfuscations.
	count int
}

// consistentObfuscation helps in generating the consistent MAC obfuscation that happens within the count limit
func (m *macAddressObfuscator) consistentObfuscation() string {
	m.count++
	if m.count > maximumSupportedObfuscations {
		klog.Exitf("maximum number of mac obfuscations exceeded: %d", maximumSupportedObfuscations)
	}
	r := fmt.Sprintf(consistentMACTemplate, m.count)
	return r
}

func (m *macAddressObfuscator) FileName(s string) string {
	return m.Contents(s)
}

func (m *macAddressObfuscator) Contents(s string) string {
	matches := m.regex.FindAllString(s, -1)
	for _, match := range matches {
		var replacement string
		switch m.replacementType {
		case schema.ObfuscateReplacementTypeStatic:
			replacement = staticMacReplacement
		case schema.ObfuscateReplacementTypeConsistent:
			replacement = m.consistentObfuscation()
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
	return &macAddressObfuscator{
		ReplacementTracker: reporter,
		replacementType:    replacementType,
		regex:              regex,
	}, nil
}
