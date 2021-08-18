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
	ipv6Pattern = regexp.MustCompile(ipv6re)
	ipv4Pattern = regexp.MustCompile(ipv4re)
)

type ipGenerator struct {
	template   string
	obfuscated string
	count      int
}

func (g *ipGenerator) generateConsistent() string {
	g.count++
	if g.count > maximumSupportedObfuscations {
		panic("maximum number of obfuscated ips exceeded")
	}
	return fmt.Sprintf(g.template, g.count)
}

func (g *ipGenerator) static() string {
	return g.obfuscated
}

var (
	ipv4re = `(([0-9]{1,3})\.){3}([0-9]{1,3})`
	// ipv6re is not perfect. it can still catch words like :face:bad as a valid ipv6 address
	ipv6re = `([a-f0-9]{0,4}:){1,8}[a-f0-9]{1,4}`
)

type ipObfuscator struct {
	ReplacementReporter
	replacements    map[*regexp.Regexp]*ipGenerator
	replacementType schema.ObfuscateReplacementType
}

func (o *ipObfuscator) FileName(s string) string {
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
			if ip := net.ParseIP(m); ip != nil {
				var replacement string
				switch o.replacementType {
				case schema.ObfuscateReplacementTypeStatic:
					replacement = gen.static()
				case schema.ObfuscateReplacementTypeConsistent:
					if replacement = o.GetReplacement(m); replacement == "" {
						replacement = gen.generateConsistent()
					}
				}
				output = strings.ReplaceAll(output, m, replacement)
				o.ReportReplacement(m, replacement)
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
		ReplacementReporter: NewSimpleReporter(),
		replacements: map[*regexp.Regexp]*ipGenerator{
			ipv4Pattern: {template: consistentIPv4Template, obfuscated: obfuscatedStaticIPv4},
			ipv6Pattern: {template: consistentIPv6Template, obfuscated: obfuscatedStaticIPv6},
		},
		replacementType: replacementType,
	}, nil
}
