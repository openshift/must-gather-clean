package obfuscator

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/openshift/must-gather-clean/pkg/schema"
)

const (
	domainPattern                      = `([a-zA-Z0-9\.]*\.)?(%s)`
	obfuscatedTemplate                 = "domain%010d"
	staticDomainReplacement            = "obfuscated.com"
	maximumSupportedObfuscationDomains = 9999999999
)

type domainObfuscator struct {
	ReplacementTracker
	domainPatterns []*regexp.Regexp
	obfsGenerator  generator
}

func (d *domainObfuscator) Path(s string) string {
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
			obfuscatedBaseDomain := d.obfsGenerator.generateReplacement(baseDomain, m[0], 1, d.ReplacementTracker)
			var replacement string
			if subDomain != "" {
				replacement = fmt.Sprintf("%s%s", subDomain, obfuscatedBaseDomain)
			} else {
				replacement = obfuscatedBaseDomain
			}
			// TODO(thomas): should just replace that one matching occurrence instead of all
			output = strings.ReplaceAll(output, m[0], replacement)
		}
	}
	return output
}

func NewDomainObfuscator(domains []string, replacementType schema.ObfuscateReplacementType, tracker ReplacementTracker) (ReportingObfuscator, error) {
	if len(domains) == 0 {
		return nil, fmt.Errorf("no domainNames supplied for the obfuscation type: Domain")
	}
	patterns := make([]*regexp.Regexp, len(domains))
	for i, d := range domains {
		dd := strings.ReplaceAll(d, ".", "\\.")
		p, err := regexp.Compile(fmt.Sprintf(domainPattern, dd))
		if err != nil {
			return nil, fmt.Errorf("failed to generate regex for domain %s: %w", d, err)
		}
		patterns[i] = p
	}

	// we are sorting descending to always match the most specific pattern first
	sort.Slice(patterns, func(i, j int) bool {
		return len(patterns[i].String()) > len(patterns[j].String())
	})

	// creating a new generator object
	generator, err := newGenerator(obfuscatedTemplate, staticDomainReplacement, maximumSupportedObfuscationDomains, replacementType)
	if err != nil {
		return nil, err
	}
	return &domainObfuscator{
		ReplacementTracker: tracker,
		domainPatterns:     patterns,
		obfsGenerator:      *generator,
	}, nil
}
