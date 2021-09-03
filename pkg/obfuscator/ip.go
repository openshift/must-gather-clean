package obfuscator

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/openshift/must-gather-clean/pkg/schema"
)

const (
	obfuscatedStaticIPv4         = "xxx.xxx.xxx.xxx"
	obfuscatedStaticIPv6         = "xxxx:xxxx:xxxx:xxxx:xxxx:xxxx:xxxx:xxxx"
	consistentIPv4Template       = "x-ipv4-%06d-x"
	consistentIPv6Template       = "xxxxxxxxxxxxx-ipv6-%06d-xxxxxxxxxxxxx"
	maximumSupportedObfuscations = 999999
)

var (
	ipv4re = `\b(([0-9]{1,3}[.]){3}|([0-9]{1,3}[-]){3})([0-9]{1,3})`
	// ipv6re is not perfect. it can still catch words like :face:bad as a valid ipv6 address
	ipv6re      = `([a-f0-9]{0,4}[:]){1,8}[a-f0-9]{1,4}`
	ipv6Pattern = regexp.MustCompile(ipv6re)
	ipv4Pattern = regexp.MustCompile(ipv4re)
	excludedIPs = map[string]struct{}{
		"127.0.0.1": {},
		"0.0.0.0":   {},
		"::1":       {},
	}
)

type ipObfuscator struct {
	ReplacementTracker
	replacements    map[*regexp.Regexp]*generator
	replacementType schema.ObfuscateReplacementType
}

func (o *ipObfuscator) Path(s string) string {
	return o.replace(s)
}

func (o *ipObfuscator) Contents(s string) string {
	return o.replace(s)
}

func (o *ipObfuscator) replace(s string) string {
	output := s
	for pattern, gen := range o.replacements {

		ipMatches := pattern.FindAllString(output, -1)

		for _, m := range ipMatches {
			// if the match is in the exclude-list then do not replace.
			if _, ok := excludedIPs[m]; ok {
				continue
			}

			cleaned := strings.ReplaceAll(m, "-", ".")
			if ip := net.ParseIP(cleaned); ip != nil {
				var (
					replacement string
					isPresent   bool
				)
				switch o.replacementType {
				case schema.ObfuscateReplacementTypeStatic:
					replacement, isPresent = o.GenerateIfAbsent(cleaned, gen.generateStaticReplacement)
				case schema.ObfuscateReplacementTypeConsistent:
					replacement, isPresent = o.GenerateIfAbsent(cleaned, gen.generateConsistentReplacement)
				}
				// replacement would be either consistent or static and hence not appending to the report if it is an empty string
				if replacement != "" {
					// replacing the input string with the obtained replacement
					output = strings.ReplaceAll(output, m, replacement)
					// adding the replacement to the report if not already present
					if !isPresent {
						o.ReplacementTracker.AddReplacement(cleaned, replacement)
					}
				}
			}
		}
	}
	return output
}

func NewIPObfuscator(replacementType schema.ObfuscateReplacementType) (Obfuscator, error) {
	if replacementType != schema.ObfuscateReplacementTypeStatic && replacementType != schema.ObfuscateReplacementTypeConsistent {
		return nil, fmt.Errorf("unsupported replacement type: %s", replacementType)
	}
	return &ipObfuscator{
		ReplacementTracker: NewSimpleTracker(),
		replacements: map[*regexp.Regexp]*generator{
			ipv4Pattern: {template: consistentIPv4Template, static: obfuscatedStaticIPv4},
			ipv6Pattern: {template: consistentIPv6Template, static: obfuscatedStaticIPv6},
		},
		replacementType: replacementType,
	}, nil
}
