package obfuscator

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	domainPattern      = `([a-zA-Z0-9\.]*\.)?(%s)`
	obfuscatedTemplate = "domain%07d"
)

type domainObfuscator struct {
	ReplacementTracker
	domainCount    int
	domainPatterns []*regexp.Regexp
	domainMapping  map[string]string
}

func (d *domainObfuscator) FileName(s string) string {
	return d.replaceDomains(s)
}

func (d *domainObfuscator) Contents(s string) string {
	return d.replaceDomains(s)
}

func (d *domainObfuscator) replaceDomains(input string) string {
	output := input
	for _, p := range d.domainPatterns {
		matches := p.FindAllStringSubmatch(output, -1)
		for _, m := range matches {
			if len(m) != 3 {
				continue
			}
			baseDomain := m[2]
			subDomain := m[1]
			obfuscatedBaseDomain := d.obfuscatedDomain(baseDomain)
			var replacement string
			if subDomain != "" {
				replacement = fmt.Sprintf("%s%s", subDomain, obfuscatedBaseDomain)
			} else {
				replacement = obfuscatedBaseDomain
			}
			output = strings.ReplaceAll(output, m[0], replacement)
			d.AddReplacement(m[0], replacement)
		}
	}
	return output
}

func (d *domainObfuscator) obfuscatedDomain(domain string) string {
	if replacement, ok := d.domainMapping[domain]; ok {
		return replacement
	}
	d.domainCount++
	replacement := fmt.Sprintf(obfuscatedTemplate, d.domainCount)
	d.domainMapping[domain] = replacement
	return replacement
}

func NewDomainObfuscator(domains []string) (Obfuscator, error) {
	patterns := make([]*regexp.Regexp, len(domains))
	for i, d := range domains {
		dd := strings.ReplaceAll(d, ".", "\\.")
		p, err := regexp.Compile(fmt.Sprintf(domainPattern, dd))
		if err != nil {
			return nil, fmt.Errorf("failed to generate regex for domain %s: %w", d, err)
		}
		patterns[i] = p
	}
	return &domainObfuscator{
		ReplacementTracker: NewSimpleTracker(),
		domainPatterns:     patterns,
		domainMapping:      map[string]string{},
	}, nil
}
