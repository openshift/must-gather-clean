package obfuscator

import (
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
	ipv4re = `(([1-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])[.]([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])[.]([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])[.]|([1-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])[_-]([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])[_-]([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])[_-])([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]){1,3}`
	// ipv6re is not perfect. it can still catch words like :face:bad as a valid ipv6 address
	ipv6re      = `(([a-f0-9]{0,4}[:]){1,8}([0-9a-fA-F]{1,4}|::)+)`
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
	replacements []replacementGenerator
}

type replacementGenerator struct {
	pattern   *regexp.Regexp
	generator *generator
}

func (o *ipObfuscator) Path(s string) string {
	return o.replace(s)
}

func (o *ipObfuscator) Contents(s string) string {
	return o.replace(s)
}

func (o *ipObfuscator) replace(s string) string {
	output := s

	for _, r := range o.replacements {
		ipMatches := r.pattern.FindAllString(output, -1)
		for _, m := range ipMatches {
			// if the match is in the exclude-list then do not replace.
			if _, ok := excludedIPs[m]; ok {
				continue
			}

			cleaned := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(m, "_", "."), "-", "."))
			if ip := net.ParseIP(cleaned); ip != nil {
				replacement := r.generator.generateReplacement(cleaned, m, 1, o.ReplacementTracker)
				// TODO(thomas): should just replace that one matching occurrence instead of all
				output = strings.ReplaceAll(output, m, replacement)
			}
		}
	}
	return output
}

func NewIPObfuscator(replacementType schema.ObfuscateReplacementType, tracker ReplacementTracker) (ReportingObfuscator, error) {
	genIPv4, err := newGenerator(consistentIPv4Template, obfuscatedStaticIPv4, maximumSupportedObfuscationsIP, replacementType)
	if err != nil {
		return nil, err
	}
	genIPv6, err := newGenerator(consistentIPv6Template, obfuscatedStaticIPv6, maximumSupportedObfuscationsIP, replacementType)
	if err != nil {
		return nil, err
	}
	return &ipObfuscator{
		ReplacementTracker: tracker,
		replacements: []replacementGenerator{
			{pattern: ipv4Pattern, generator: genIPv4},
			{pattern: ipv6Pattern, generator: genIPv6},
		},
	}, nil
}
