package obfuscator

import (
	"strings"

	"github.com/openshift/must-gather-clean/pkg/schema"
)

const (
	// staticMacReplacement refers to a static replacement for any identified MAC address.
	staticMacReplacement = "xx:xx:xx:xx:xx:xx"
	// there are 2^32 (4,294,967,296) addresses in total, we can support that with 10 characters
	consistentMACTemplate           = "x-mac-%010d-x"
	maximumSupportedObfuscationsMAC = 9999999999
)

type macAddressObfuscator struct {
	ReplacementTracker
	obfsGenerator generator
}

func (m *macAddressObfuscator) Path(s string) string {
	return m.Contents(s)
}

func (m *macAddressObfuscator) Contents(s string) string {
	matches := m.findMatches(s)
	if len(matches) == 0 {
		return s
	}

	var output strings.Builder
	output.Grow(len(s))
	lastIndex := 0
	for _, match := range matches {
		mac := s[match[0]:match[1]]
		key := canonicalMAC(mac)
		output.WriteString(s[lastIndex:match[0]])
		output.WriteString(m.obfsGenerator.generateReplacement(key, mac, 1, m.ReplacementTracker))
		lastIndex = match[1]
	}
	output.WriteString(s[lastIndex:])
	return output.String()
}

func (m *macAddressObfuscator) findMatches(input string) [][]int {
	var matches [][]int
	for start := 0; start < len(input); {
		if insidePercentEncodingAt(input, start) || !possibleMACStart(input, start) {
			start++
			continue
		}
		end, found := matchMACAt(input, start)
		if !found {
			start++
			continue
		}
		matches = append(matches, []int{start, end})
		start = end
	}
	return matches
}

func possibleMACStart(input string, index int) bool {
	_, _, valid := readMACHexDigit(input, index)
	return valid
}

func matchMACAt(input string, start int) (int, bool) {
	index := start
	for groups := 0; groups < 6; groups++ {
		groupEnd, valid := matchMACGroupAt(input, index)
		if !valid {
			return 0, false
		}
		index = groupEnd
		if groups == 5 {
			break
		}
		separatorWidth, valid := readMACSeparator(input, index)
		if !valid {
			return 0, false
		}
		index += separatorWidth
	}
	return index, true
}

func matchMACGroupAt(input string, start int) (int, bool) {
	index := start
	for digit := 0; digit < 2; digit++ {
		_, width, valid := readMACHexDigit(input, index)
		if !valid {
			return 0, false
		}
		index += width
	}
	return index, true
}

func readMACHexDigit(input string, index int) (byte, int, bool) {
	value, width, valid := decodedByteAt(input, index)
	if valid && isHexDigit(value) {
		return value, width, true
	}
	return 0, 0, false
}

func readMACSeparator(input string, index int) (int, bool) {
	if index >= len(input) {
		return 0, false
	}
	if input[index] == ':' || input[index] == '-' {
		return 1, true
	}
	value, width, valid := decodedByteAt(input, index)
	if valid && (value == ':' || value == '-') {
		return width, true
	}
	return 0, false
}

func canonicalMAC(value string) string {
	canonical, valid := parseCanonicalMAC(value)
	if !valid {
		return value
	}
	return canonical
}

func parseCanonicalMAC(value string) (string, bool) {
	canonical := make([]byte, 0, len(value))
	index := 0
	for group := 0; group < 6; group++ {
		for digit := 0; digit < 2; digit++ {
			byteValue, width, valid := readMACHexDigit(value, index)
			if !valid {
				return "", false
			}
			canonical = append(canonical, byteValue)
			if byteValue >= 'a' && byteValue <= 'f' {
				canonical[len(canonical)-1] -= 'a' - 'A'
			}
			index += width
		}
		if group == 5 {
			break
		}
		separatorWidth, valid := readMACSeparator(value, index)
		if !valid {
			return "", false
		}
		index += separatorWidth
		canonical = append(canonical, ':')
	}
	if index != len(value) {
		return "", false
	}
	return string(canonical), true
}

func NewMacAddressObfuscator(replacementType schema.ObfuscateReplacementType, tracker ReplacementTracker) (ReportingObfuscator, error) {
	// creating a new generator object
	generator, err := newGenerator(consistentMACTemplate, staticMacReplacement, maximumSupportedObfuscationsMAC, replacementType)
	if err != nil {
		return nil, err
	}
	return &macAddressObfuscator{
		ReplacementTracker: tracker,
		obfsGenerator:      *generator,
	}, nil
}
