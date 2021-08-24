package obfuscator

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"

	"github.com/openshift/must-gather-clean/pkg/schema"
	"k8s.io/klog/v2"
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

type ipGenerator struct {
	ipReplacements map[string]string
	template       string
	obfuscated     string
	count          int
	lock           sync.RWMutex
}

func (g *ipGenerator) consistent(ip string) string {
	g.lock.Lock()
	defer g.lock.Unlock()

	if r, ok := g.ipReplacements[ip]; ok {
		return r
	}

	g.count++
	if g.count > maximumSupportedObfuscations {
		klog.Exitf("maximum number of ip obfuscations exceeded: %d", maximumSupportedObfuscations)
	}
	r := fmt.Sprintf(g.template, g.count)
	g.ipReplacements[ip] = r
	return r
}

func (g *ipGenerator) static(ip string) string {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.ipReplacements[ip] = g.obfuscated
	return g.obfuscated
}

func (g *ipGenerator) replacements() map[string]string {
	g.lock.RLock()
	defer g.lock.RUnlock()
	rCopy := make(map[string]string)
	for k, v := range g.ipReplacements {
		rCopy[k] = v
	}
	return rCopy
}

type ipObfuscator struct {
	replacements    map[*regexp.Regexp]*ipGenerator
	replacementType schema.ObfuscateReplacementType
}

func (o *ipObfuscator) ReportingResult() map[string]string {
	result := make(map[string]string)
	for _, replacers := range o.replacements {
		for k, v := range replacers.replacements() {
			result[k] = v
		}
	}
	return result
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
			// if the match is in the exclude-list then do not replace.
			if _, ok := excludedIPs[m]; ok {
				continue
			}

			cleaned := strings.ReplaceAll(m, "-", ".")
			if ip := net.ParseIP(cleaned); ip != nil {
				var replacement string
				switch o.replacementType {
				case schema.ObfuscateReplacementTypeStatic:
					replacement = gen.static(m)
				case schema.ObfuscateReplacementTypeConsistent:
					replacement = gen.consistent(m)
				}
				output = strings.ReplaceAll(output, m, replacement)
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
		replacements: map[*regexp.Regexp]*ipGenerator{
			ipv4Pattern: {template: consistentIPv4Template, obfuscated: obfuscatedStaticIPv4, ipReplacements: map[string]string{}},
			ipv6Pattern: {template: consistentIPv6Template, obfuscated: obfuscatedStaticIPv6, ipReplacements: map[string]string{}},
		},
		replacementType: replacementType,
	}, nil
}
