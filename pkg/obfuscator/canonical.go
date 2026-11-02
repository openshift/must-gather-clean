package obfuscator

import (
	"fmt"
	"net"

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
// written to a domain replacement report.
func IsCanonicalDomain(value string) bool {
	return value != "" && value == lowerASCIIString(value) && validDomainText(value)
}

func validateReplacementKey(original string, obfuscationType schema.ObfuscateType) error {
	decoded := decodePercentEncoded(original)
	switch obfuscationType {
	case schema.ObfuscateTypeIP:
		if net.ParseIP(canonicalIP(decoded)) == nil {
			return fmt.Errorf("invalid replacement key %q: not an IP address", original)
		}
	case schema.ObfuscateTypeMAC:
		if _, valid := parseCanonicalMAC(decoded); !valid {
			return fmt.Errorf("invalid replacement key %q: not a MAC address", original)
		}
	case schema.ObfuscateTypeDomain:
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
