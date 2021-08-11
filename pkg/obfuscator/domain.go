package obfuscator

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type domainObfuscator struct {
	ReplacementReporter
	hostPattern *regexp.Regexp
	domainCount int
	tlds        map[string]struct{}
	mainDomains map[string]string
	ipPattern   *regexp.Regexp
}

func (d *domainObfuscator) FileName(s string) string {
	// if the filename has an extension omit it from obfuscation
	// this is still error-prone because the last part of the domain name could still be mistaken for an extension
	extension := filepath.Ext(s)
	if extension != "" {
		return fmt.Sprintf("%s%s", d.findDomains(s[:len(s)-len(extension)]), extension)
	}
	return d.findDomains(s)
}

func (d *domainObfuscator) findDomains(input string) string {
	domains := d.hostPattern.FindAllString(input, -1)

	output := input
	for _, domain := range domains {
		// if this is an IP address then do nothing
		if d.ipPattern.MatchString(domain) {
			continue
		}
		output = strings.ReplaceAll(output, domain, d.obfuscatedDomain(domain))
	}
	return output
}

func (d *domainObfuscator) Contents(s string) string {
	return d.findDomains(s)
}

func (d *domainObfuscator) obfuscatedDomain(domain string) string {
	parts := strings.Split(domain, ".")
	var (
		mainDomain   string
		subDomain    string
		hasExtension bool
	)
	// if the last part is present in known list of tlds then combine last and second-to-last parts to get
	// the main domain otherwise the last part is the main domain
	if _, ok := d.tlds[parts[len(parts)-1]]; ok {
		mainDomain = strings.Join(parts[len(parts)-2:], ".")
		subDomain = strings.Join(parts[:len(parts)-2], ".")
		hasExtension = true
	} else {
		mainDomain = parts[len(parts)-1]
		subDomain = strings.Join(parts[:len(parts)-1], ".")
	}

	var (
		obfuscatedMainDomain string
		ok                   bool
	)
	// if the obfuscated main domain is present then use it, otherwise generate one and store it.
	if obfuscatedMainDomain, ok = d.mainDomains[mainDomain]; !ok {
		d.domainCount++
		if hasExtension {
			obfuscatedMainDomain = fmt.Sprintf("obfuscated%04d.ext", d.domainCount)
		} else {
			obfuscatedMainDomain = fmt.Sprintf("obfuscated%04d", d.domainCount)
		}
		d.mainDomains[mainDomain] = obfuscatedMainDomain
	}

	var obfuscatedDomain string
	// if there is no subdomain then leave it out
	if subDomain != "" {
		obfuscatedDomain = fmt.Sprintf("%s.%s", subDomain, obfuscatedMainDomain)
	} else {
		obfuscatedDomain = obfuscatedMainDomain
	}
	d.ReportReplacement(domain, obfuscatedDomain)
	return obfuscatedDomain
}

func NewDomainObfuscator(tlds []string) Obfuscator {
	tldSet := map[string]struct{}{}
	for _, tld := range tlds {
		tldSet[strings.TrimLeft(tld, ".")] = struct{}{}
	}
	return &domainObfuscator{
		hostPattern:         regexp.MustCompile(`[a-zA-Z0-9-\.]{1,200}\.[a-zA-Z0-9]{1,63}`),
		ipPattern:           regexp.MustCompile(`([0-9]{1,3}\.){3}[0-9]{1,3}`),
		ReplacementReporter: NewSimpleReporter(),
		tlds:                tldSet,
		mainDomains:         map[string]string{},
	}
}
