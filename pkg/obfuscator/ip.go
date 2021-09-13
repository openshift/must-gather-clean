package obfuscator

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/openshift/must-gather-clean/pkg/schema"
)

const (
	obfuscatedStaticIPv4 = "xxx.xxx.xxx.xxx"
	obfuscatedStaticIPv6 = "xxxx:xxxx:xxxx:xxxx:xxxx:xxxx:xxxx:xxxx"

	maximumSupportedObfuscationsIP = 9999999999
	// there are 2^32 (4,294,967,296) addresses in total, we can support that with 10 characters
	consistentIPv4Template = "x-ipv4-%010d-x"
	// there are 2^128 possible v6 IPs, but we keep them down to the same amount as the v4s.
	// must-gathers today don't have any v6 IPs in them yet, so this should be enough to be future-proof
	consistentIPv6Template = "x-ipv6-%010d-x"
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
				var replacement string
				switch o.replacementType {
				case schema.ObfuscateReplacementTypeStatic:
					replacement = o.GenerateIfAbsent(cleaned, gen.generateStaticReplacement)
				case schema.ObfuscateReplacementTypeConsistent:
					replacement = o.GenerateIfAbsent(cleaned, gen.generateConsistentReplacement)
				}
				output = strings.ReplaceAll(output, m, replacement)
				// also add the original (non-cleaned) string, this is only used for human review in the final report
				o.ReplacementTracker.AddReplacement(m, replacement)
			}
		}
	}
	return output
}

func NewIPObfuscator(replacementType schema.ObfuscateReplacementType) (ReportingObfuscator, error) {
	if replacementType != schema.ObfuscateReplacementTypeStatic && replacementType != schema.ObfuscateReplacementTypeConsistent {
		return nil, fmt.Errorf("unsupported replacement type: %s", replacementType)
	}
	return &ipObfuscator{
		ReplacementTracker: NewSimpleTracker(),
		replacements: map[*regexp.Regexp]*generator{
			ipv4Pattern: newGenerator(consistentIPv4Template, obfuscatedStaticIPv4, maximumSupportedObfuscationsIP),
			ipv6Pattern: newGenerator(consistentIPv6Template, obfuscatedStaticIPv6, maximumSupportedObfuscationsIP),
		},
		replacementType: replacementType,
	}, nil
}
