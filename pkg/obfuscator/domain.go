package obfuscator

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/openshift/must-gather-clean/pkg/schema"
)

const (
	obfuscatedTemplate                 = "domain%010d"
	staticDomainReplacement            = "obfuscated.com"
	maximumSupportedObfuscationDomains = 9999999999
	// legacyDomainPattern is used only during construction to preserve the
	// upstream contract for configured domains that cannot compile as regexps.
	legacyDomainPattern = `([a-zA-Z0-9\.]*\.)?%s`
	// These characters changed the meaning of the legacy generated regexp.
	// The new matcher is literal, so configurations using them are ambiguous
	// and are rejected instead of silently changing behavior.
	regexMetacharacters = "+*?()|[]{}^$\\"
)

type domainObfuscator struct {
	ReplacementTracker
	domainsByFirstByte [256][]string
	obfsGenerator      generator
}

func (d *domainObfuscator) Path(s string) string {
	return d.replaceDomains(s)
}

func (d *domainObfuscator) Contents(s string) string {
	return d.replaceDomains(s)
}

func (d *domainObfuscator) replaceDomains(input string) string {
	matches := d.findDomainMatches(input)
	if len(matches) == 0 {
		return input
	}

	var output strings.Builder
	output.Grow(len(input))
	lastIndex := 0
	for _, match := range matches {
		if match.domainStart < lastIndex {
			continue
		}
		// Only scan back as far as the previous replacement. The prefix before
		// it was already preserved in the output and cannot change the report's
		// original value for this match. Capping the scan here also bounds the
		// total work for dense, adjacent domain matches.
		originalStart := domainMatchStart(input, match.domainStart, lastIndex)
		output.WriteString(input[lastIndex:match.domainStart])
		obfuscatedBaseDomain := d.obfsGenerator.generateReplacement(
			match.domain,
			input[originalStart:match.domainEnd],
			1,
			d.ReplacementTracker,
		)
		output.WriteString(obfuscatedBaseDomain)
		lastIndex = match.domainEnd
	}
	output.WriteString(input[lastIndex:])
	return output.String()
}

func NewDomainObfuscator(domains []string, replacementType schema.ObfuscateReplacementType, tracker ReplacementTracker) (ReportingObfuscator, error) {
	if len(domains) == 0 {
		return nil, fmt.Errorf("no domainNames supplied for the obfuscation type: Domain")
	}
	knownDomains := make(map[string]struct{}, len(domains))
	for index, domain := range domains {
		if domain == "" {
			return nil, fmt.Errorf("invalid domain at index %d: domain cannot be empty", index)
		}
		if !validDomainText(domain) {
			return nil, fmt.Errorf(
				"invalid domain at index %d: domain must be printable ASCII",
				index,
			)
		}
		for byteIndex := 0; byteIndex < len(domain); byteIndex++ {
			if domain[byteIndex] >= 0x80 {
				return nil, fmt.Errorf(
					"invalid domain at index %d: domain must be ASCII; encode internationalized names as punycode",
					index,
				)
			}
		}
		// The matcher itself is literal, but keep the upstream constructor
		// behavior for configurations that previously failed regexp compilation.
		if _, err := regexp.Compile(fmt.Sprintf(legacyDomainPattern, strings.ReplaceAll(domain, ".", "\\."))); err != nil {
			return nil, fmt.Errorf("failed to generate regex for domain %s: %w", domain, err)
		}
		if strings.ContainsAny(domain, regexMetacharacters) {
			return nil, fmt.Errorf("invalid domain at index %d: domain %q cannot contain regex metacharacters", index, domain)
		}
		knownDomains[lowerASCIIString(domain)] = struct{}{}
	}

	knownDomainValues := make([]string, 0, len(knownDomains))
	for domain := range knownDomains {
		knownDomainValues = append(knownDomainValues, domain)
	}
	sort.Slice(knownDomainValues, func(first, second int) bool {
		firstLength := domainPatternLength(knownDomainValues[first])
		secondLength := domainPatternLength(knownDomainValues[second])
		if firstLength != secondLength {
			return firstLength > secondLength
		}
		return knownDomainValues[first] < knownDomainValues[second]
	})

	// creating a new generator object
	generator, err := newGenerator(obfuscatedTemplate, staticDomainReplacement, maximumSupportedObfuscationDomains, replacementType)
	if err != nil {
		return nil, err
	}
	var domainsByFirstByte [256][]string
	for _, domain := range knownDomainValues {
		// Domain names are ASCII, so the first decoded input byte can safely
		// select the only bucket that can contain a match.
		firstByte := lowerASCIIByte(domain[0])
		domainsByFirstByte[firstByte] = append(domainsByFirstByte[firstByte], domain)
	}

	return &domainObfuscator{
		ReplacementTracker: tracker,
		domainsByFirstByte: domainsByFirstByte,
		obfsGenerator:      *generator,
	}, nil
}

type domainMatch struct {
	domainStart int
	domainEnd   int
	domain      string
}

func (d *domainObfuscator) findDomainMatches(input string) []domainMatch {
	var matches []domainMatch
	for domainStart := 0; domainStart < len(input); domainStart++ {
		if insidePercentEncodingAt(input, domainStart) {
			continue
		}
		rawFirstByte := input[domainStart]
		firstByte := rawFirstByte
		decodedFirstByte := true
		if rawFirstByte == '%' {
			decoded, _, valid := decodedByteAt(input, domainStart)
			decodedFirstByte = valid
			if valid {
				firstByte = decoded
			}
		}

		var bestDomain string
		var bestDomainEnd int
		if decodedFirstByte {
			bestDomain, bestDomainEnd = bestDomainMatch(
				input, domainStart, d.domainsByFirstByte[lowerASCIIByte(firstByte)],
			)
		}
		// A configured percent sign is literal, but it can also be written as
		// %25 in the input. Raw %HH is instead the encoding of another byte,
		// so both interpretations have to be considered only for domains that
		// themselves start with a percent sign. An invalid escape remains a
		// literal percent rather than skipping the position.
		if rawFirstByte == '%' && (!decodedFirstByte || firstByte != '%') {
			domain, domainEnd := bestDomainMatch(
				input, domainStart, d.domainsByFirstByte['%'],
			)
			if domain != "" && (domainPatternLength(domain) > domainPatternLength(bestDomain) ||
				(domainPatternLength(domain) == domainPatternLength(bestDomain) && domain < bestDomain)) {
				bestDomain, bestDomainEnd = domain, domainEnd
			}
		}
		if bestDomain != "" {
			matches = append(matches, domainMatch{
				domainStart: domainStart,
				domainEnd:   bestDomainEnd,
				domain:      bestDomain,
			})
		}
	}
	// Select non-overlapping matches in the same order in which the legacy
	// regexes were applied: the most specific configured domain wins globally.
	// A shorter match that merely starts earlier must not hide a longer,
	// overlapping match. Replacement itself still needs textual order because it
	// advances one lastIndex across the output.
	sort.SliceStable(matches, func(first, second int) bool {
		firstLength := domainPatternLength(matches[first].domain)
		secondLength := domainPatternLength(matches[second].domain)
		if firstLength != secondLength {
			return firstLength > secondLength
		}
		if matches[first].domainStart != matches[second].domainStart {
			return matches[first].domainStart < matches[second].domainStart
		}
		return matches[first].domain < matches[second].domain
	})

	selected := make([]domainMatch, 0, len(matches))
	for _, match := range matches {
		overlaps := false
		for _, chosen := range selected {
			if match.domainStart < chosen.domainEnd && chosen.domainStart < match.domainEnd {
				overlaps = true
				break
			}
		}
		if !overlaps {
			selected = append(selected, match)
		}
	}

	sort.SliceStable(selected, func(first, second int) bool {
		if selected[first].domainStart != selected[second].domainStart {
			return selected[first].domainStart < selected[second].domainStart
		}
		return domainPatternLength(selected[first].domain) > domainPatternLength(selected[second].domain)
	})
	return selected
}

func bestDomainMatch(input string, domainStart int, domains []string) (string, int) {
	// Buckets are sorted by the length of the legacy generated pattern, so
	// the first match is also the most specific match. Literal matches from
	// the same start are nested, and the longer domain is ordered first.
	for _, domain := range domains {
		domainEnd, matchesDomain := matchEncodedDomainAt(input, domainStart, domain)
		if matchesDomain {
			return domain, domainEnd
		}
	}
	return "", 0
}

func domainPatternLength(domain string) int {
	return len(domain) + strings.Count(domain, ".")
}

func matchEncodedDomainAt(input string, domainStart int, domain string) (int, bool) {
	return matchEncodedDomainFrom(input, domainStart, domain, 0)
}

// matchEncodedDomainFrom tries both meanings of a literal percent sign in a
// configured domain: the raw input byte and the decoded byte from %25. A
// domain such as "%25.example.com" has to match both spellings without making
// "%41.example.com" start inside the percent encoding of an unrelated byte.
func matchEncodedDomainFrom(input string, index int, domain string, domainIndex int) (int, bool) {
	if domainIndex == len(domain) {
		return index, true
	}
	wanted := domain[domainIndex]

	if wanted == '%' && index < len(input) && input[index] == '%' {
		if end, matches := matchEncodedDomainFrom(input, index+1, domain, domainIndex+1); matches {
			return end, true
		}
	}

	value, width, valid := decodedByteAt(input, index)
	if valid && lowerASCIIByte(value) == lowerASCIIByte(wanted) {
		if end, matches := matchEncodedDomainFrom(input, index+width, domain, domainIndex+1); matches {
			return end, true
		}
	}
	return 0, false
}

func lowerASCIIByte(value byte) byte {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
}

func domainMatchStart(input string, domainStart, minimum int) int {
	previous, previousWidth, hasPrevious := decodedByteBefore(input, domainStart)
	if domainStart <= minimum || !hasPrevious || previous != '.' {
		return domainStart
	}
	start := domainStart - previousWidth
	for start > minimum {
		value, width, valid := decodedByteBefore(input, start)
		if !valid || !isDomainPrefixByte(value) {
			break
		}
		start -= width
	}
	if start < minimum {
		return minimum
	}
	return start
}

func isDomainPrefixByte(value byte) bool {
	return value >= 'A' && value <= 'Z' ||
		value >= 'a' && value <= 'z' ||
		value >= '0' && value <= '9' ||
		value == '.'
}
