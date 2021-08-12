package obfuscator

import (
	"net"
	"regexp"
	"strings"
)

const (
	obfuscatedStaticIPv4 = "xxx.xxx.xxx.xxx"
	obfuscatedStaticIPv6 = "xxxx:xxxx:xxxx:xxxx:xxxx:xxxx:xxxx:xxxx"
)

var (
	ipv4re = `(([0-9]{1,3})\.){3}([0-9]{1,3})`
	// ipv6re is not perfect. it can still catch words like :face:bad as a valid ipv6 address
	ipv6re = `([a-f0-9]{0,4}:){1,8}[a-f0-9]{1,4}`
)

type ipObfuscator struct {
	ReplacementReporter
	ipv4pattern *regexp.Regexp
	ipv6pattern *regexp.Regexp
}

func (o *ipObfuscator) FileName(s string) string {
	return o.replace(s)
}

func (o *ipObfuscator) Contents(s string) string {
	return o.replace(s)
}

func (o *ipObfuscator) replace(s string) string {
	output := s
	ipv4matches := o.ipv4pattern.FindAllString(output, -1)
	for _, m := range ipv4matches {
		if ip := net.ParseIP(m); ip != nil {
			output = strings.ReplaceAll(output, m, obfuscatedStaticIPv4)
			o.ReportReplacement(m, obfuscatedStaticIPv4)
		}
	}

	ipv6matches := o.ipv6pattern.FindAllString(output, -1)
	for _, m := range ipv6matches {
		if ip := net.ParseIP(m); ip != nil {
			output = strings.ReplaceAll(output, m, obfuscatedStaticIPv6)
			o.ReportReplacement(m, obfuscatedStaticIPv6)
		}
	}

	return output
}

func NewIPObfuscator() Obfuscator {
	return &ipObfuscator{
		ipv4pattern:         regexp.MustCompile(ipv4re),
		ipv6pattern:         regexp.MustCompile(ipv6re),
		ReplacementReporter: NewSimpleReporter(),
	}
}
