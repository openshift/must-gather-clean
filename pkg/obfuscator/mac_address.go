package obfuscator

import (
	"regexp"
	"strings"

	"github.com/openshift/must-gather-clean/pkg/schema"
)

const (
	// staticMacReplacement refers to a static replacement for any identified MAC address.
	staticMacReplacement = "xx:xx:xx:xx:xx:xx"
	// there are 2^32 (4,294,967,296) addresses in total, we can support that with 10 characters
	consistentMACTemplate           = "x-mac-%010d-x"
	maximumSupportedObfuscationsMAC = 9999999999
)

type macAddressObfuscator struct {
	ReplacementTracker
	regex         *regexp.Regexp
	obfsGenerator generator
}

func (m *macAddressObfuscator) Path(s string) string {
	return m.Contents(s)
}

func (m *macAddressObfuscator) Contents(s string) string {
	matches := m.regex.FindAllString(s, -1)
	for _, mac := range matches {
		// normalizing the MAC Address string to the Uppercase to avoid the duplicate reporting
		match := strings.ToUpper(strings.ReplaceAll(mac, "-", ":"))
		replacement := m.obfsGenerator.generateReplacement(match, m.ReplacementTracker)
		s = strings.ReplaceAll(s, mac, replacement)
		// also add the original (non-cleaned) string, this is only used for human review in the final report
		m.ReplacementTracker.AddReplacement(mac, replacement)
	}
	return s
}

func (m *macAddressObfuscator) Report() map[string]string {
	return m.ReplacementTracker.Report()
}

func NewMacAddressObfuscator(replacementType schema.ObfuscateReplacementType) (ReportingObfuscator, error) {
	// this regex differs from the standard `(?:[0-9a-fA-F]([:-])?){12}`, to not match very frequently happening UUIDs in K8s
	// the main culprit is the support for squashed MACs like '69806FE67C05', which won't be supported with the below
	regex := regexp.MustCompile(`([0-9a-fA-F]{2}[:-]){5}[0-9a-fA-F]{2}`)

	reporter := NewSimpleTracker()
	// creating a new generator object
	generator, err := newGenerator(consistentMACTemplate, staticMacReplacement, maximumSupportedObfuscationsMAC, replacementType)
	if err != nil {
		return nil, err
	}
	return &macAddressObfuscator{
		ReplacementTracker: reporter,
		regex:              regex,
		obfsGenerator:      *generator,
	}, nil
}
