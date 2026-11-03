package obfuscator

import (
	"fmt"
	"net"
	"strings"

	"github.com/openshift/must-gather-clean/pkg/schema"
)

// IsCanonicalIP reports whether value is already in the exact form written to
// an obfuscation report. Alternate separators and casing are accepted in
// replacement-map keys, but reports must contain one unambiguous spelling.
func IsCanonicalIP(value string) bool {
	return value == canonicalIP(value) && net.ParseIP(value) != nil
}

// IsCanonicalMAC reports whether value is a six-group MAC using uppercase
// hexadecimal digits and colons.
func IsCanonicalMAC(value string) bool {
	canonical, valid := parseCanonicalMAC(value)
	return valid && canonical == value
}

// IsCanonicalDomain reports whether value is the lowercase printable form
// written to a new domain replacement report.
func IsCanonicalDomain(value string) bool {
	return value != "" && value == lowerASCIIString(value) && validDomainText(value)
}

func validateReplacementKey(original string, obfuscationType schema.ObfuscateType) error {
	switch obfuscationType {
	case schema.ObfuscateTypeIP:
		if net.ParseIP(canonicalIP(decodePercentEncoded(original))) == nil {
			return fmt.Errorf("invalid replacement key %q: not an IP address", original)
		}
	case schema.ObfuscateTypeMAC:
		if _, valid := parseCanonicalMAC(original); !valid {
			return fmt.Errorf("invalid replacement key %q: not a MAC address", original)
		}
	case schema.ObfuscateTypeDomain:
		decoded := decodePercentEncoded(original)
		if decoded == "" || !validDomainText(decoded) {
			return fmt.Errorf("invalid replacement key %q: domain must be non-empty printable ASCII", original)
		}
	}
	return nil
}

func validDomainText(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] >= 0x80 || value[index] <= ' ' || value[index] == 0x7f {
			return false
		}
	}
	return true
}

func canonicalIP(value string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(value, "_", "."), "-", "."))
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

func lowerASCIIByte(value byte) byte {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
}

func domainPatternLength(domain string) int {
	return len(domain) + strings.Count(domain, ".")
}
