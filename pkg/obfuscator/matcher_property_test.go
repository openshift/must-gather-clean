package obfuscator

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/schema"
)

const matcherPropertyAlphabet = "0123456789abcdefABCDEF:.%_-/:;=@[]{}()?"

type matcherPropertyResult struct {
	ipContents     string
	ipPath         string
	macContents    string
	macPath        string
	domainContents string
	domainPath     string
	ipReport       ReplacementReport
	macReport      ReplacementReport
	domainReport   ReplacementReport
}

func TestIPMatcherAcceptsValidIPv4Spellings(t *testing.T) {
	random := rand.New(rand.NewPCG(0x4950, 0x7634))
	obfuscator := newStaticIPObfuscatorForProperty(t)

	for iteration := 0; iteration < 1000; iteration++ {
		octets := randomBytes(random, 4)
		address := fmt.Sprintf("%d.%d.%d.%d", octets[0], octets[1], octets[2], octets[3])
		if address == "127.0.0.1" || address == "0.0.0.0" {
			continue
		}

		for variant := 0; variant < 5; variant++ {
			value := ipv4PropertyForm(octets, variant)
			input := "value=" + value + " trailing"
			want := "value=" + obfuscatedStaticIPv4 + " trailing"
			if got := obfuscator.Contents(input); got != want {
				t.Fatalf("variant %d: Contents(%q) = %q, want %q", variant, input, got, want)
			}
			if got := obfuscator.Path(input); got != want {
				t.Fatalf("variant %d: Path(%q) = %q, want %q", variant, input, got, want)
			}
		}
	}

	for _, replacement := range obfuscator.Report().Replacements {
		if !IsCanonicalIP(replacement.Canonical) {
			t.Fatalf("report contains non-canonical IP %q", replacement.Canonical)
		}
	}
}

func TestIPMatcherRejectsSuffixInsideInvalidNumericToken(t *testing.T) {
	random := rand.New(rand.NewPCG(0x4950, 0x256))
	for iteration := 0; iteration < 1000; iteration++ {
		suffix := randomBytes(random, 4)
		raw := fmt.Sprintf("x=192.0.2.256.%d.%d.%d.%d", suffix[0], suffix[1], suffix[2], suffix[3])
		for _, input := range []string{raw, percentEncodeMatcherPropertyValue(raw)} {
			obfuscator := newStaticIPObfuscatorForProperty(t)
			if got := obfuscator.Contents(input); got != input {
				t.Fatalf("invalid numeric token %q changed to %q", input, got)
			}
			if got := obfuscator.Path(input); got != input {
				t.Fatalf("invalid numeric path token %q changed to %q", input, got)
			}
			if report := obfuscator.Report().AsMap(); len(report) != 0 {
				t.Fatalf("invalid numeric token %q produced report %#v", input, report)
			}
		}
	}
}

func TestIPv4InIPv6PrefilterAcceptsEmbeddedValues(t *testing.T) {
	random := rand.New(rand.NewPCG(0x4950, 0x7636))
	prefixes := []string{
		"::",
		"64:ff9b::",
		"2001:db8::",
		"1:2:3:4:5:6:",
	}

	for iteration := 0; iteration < 1000; iteration++ {
		octets := randomBytes(random, 4)
		for variant := 0; variant < 5; variant++ {
			value := decodePercentEncoded(ipv4PropertyForm(octets, variant))
			for _, prefix := range prefixes {
				if !mightContainIPv4InIPv6(prefix + value) {
					t.Fatalf("prefilter rejected embedded IPv4 value %q", prefix+value)
				}
			}
		}
	}
}

func TestMACMatcherAcceptsValidSpellings(t *testing.T) {
	random := rand.New(rand.NewPCG(0x4d4143, 0x7634))
	obfuscator, err := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeStatic, NewSimpleTracker())
	if err != nil {
		t.Fatal(err)
	}

	for iteration := 0; iteration < 1000; iteration++ {
		octets := randomBytes(random, 6)
		for variant := 0; variant < 5; variant++ {
			value := macPropertyForm(octets, variant)
			input := "value=" + value + " trailing"
			want := "value=" + staticMacReplacement + " trailing"
			if got := obfuscator.Contents(input); got != want {
				t.Fatalf("variant %d: Contents(%q) = %q, want %q", variant, input, got, want)
			}
			if got := obfuscator.Path(input); got != want {
				t.Fatalf("variant %d: Path(%q) = %q, want %q", variant, input, got, want)
			}
		}
	}

	for _, replacement := range obfuscator.Report().Replacements {
		if !IsCanonicalMAC(replacement.Canonical) {
			t.Fatalf("report contains non-canonical MAC %q", replacement.Canonical)
		}
	}
}

func TestDomainMatcherAcceptsCaseAndEncodingVariants(t *testing.T) {
	random := rand.New(rand.NewPCG(0x646f6d, 0x7634))
	obfuscator, err := NewDomainObfuscator(
		[]string{"example.com"},
		schema.ObfuscateReplacementTypeStatic,
		NewSimpleTracker(),
	)
	if err != nil {
		t.Fatal(err)
	}

	suffixes := []string{
		"example.com",
		"EXAMPLE.COM",
		"eXaMpLe.CoM",
		percentEncodeMatcherPropertyValue("example.com"),
		"example%2Ecom",
	}
	for iteration := 0; iteration < 1000; iteration++ {
		prefix := fmt.Sprintf("url=https://api.host-%d.", random.Uint32())
		for _, suffix := range suffixes {
			input := prefix + suffix + "/metrics"
			want := prefix + staticDomainReplacement + "/metrics"
			if got := obfuscator.Contents(input); got != want {
				t.Fatalf("Contents(%q) = %q, want %q", input, got, want)
			}
			if got := obfuscator.Path(input); got != want {
				t.Fatalf("Path(%q) = %q, want %q", input, got, want)
			}
		}
	}

	for _, replacement := range obfuscator.Report().Replacements {
		if !IsCanonicalDomain(replacement.Canonical) {
			t.Fatalf("report contains non-canonical domain %q", replacement.Canonical)
		}
	}
}

func TestMatchersAreDeterministicOnArbitraryInput(t *testing.T) {
	random := rand.New(rand.NewPCG(0x617262, 0x6974))
	for iteration := 0; iteration < 1000; iteration++ {
		input := randomMatcherInput(random, int(random.Uint32()%257))
		first := runMatcherProperties(t, input)
		second := runMatcherProperties(t, input)
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("matcher result is not deterministic for %q", input)
		}
	}
}

func FuzzMatcherArbitraryInput(f *testing.F) {
	// Arbitrary input exercises parser boundaries, panic resistance, deterministic
	// output, and canonical report values. Global idempotence is intentionally not
	// asserted: each obfuscator is a single-pass transformation, and replacement
	// text can change how the surrounding text is tokenized on a later pass.
	for _, seed := range []string{
		"",
		"192.0.2.1",
		"%31%39%32%2E%30%2E%32%2E%31%2Fpath",
		"192.0.2.256.1.2.3",
		"client-ca%3A%3A%2Fetc%2Ftls%2Fclient%2Fclient-ca.crt",
		"V6OVNMasqueradeIP:fd69::5",
		"mac=52%3Adf%3A20%3A08%3A6c%3Aff",
		"url=https://api.EXAMPLE.com/metrics",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 8192 {
			t.Skip()
		}
		first := runMatcherProperties(t, input)
		second := runMatcherProperties(t, input)
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("matcher result is not deterministic for %q", input)
		}
	})
}

func FuzzIPMatcherValidIPv4Forms(f *testing.F) {
	f.Add(byte(192), byte(0), byte(2), byte(1), byte(0))
	f.Add(byte(255), byte(255), byte(255), byte(255), byte(3))

	f.Fuzz(func(t *testing.T, first, second, third, fourth, variant byte) {
		octets := []byte{first, second, third, fourth}
		address := fmt.Sprintf("%d.%d.%d.%d", first, second, third, fourth)
		if address == "127.0.0.1" || address == "0.0.0.0" {
			return
		}
		value := ipv4PropertyForm(octets, int(variant)%5)
		obfuscator := newStaticIPObfuscatorForProperty(t)
		input := "value=" + value + " trailing"
		want := "value=" + obfuscatedStaticIPv4 + " trailing"
		if got := obfuscator.Contents(input); got != want {
			t.Fatalf("Contents(%q) = %q, want %q", input, got, want)
		}
	})
}

func FuzzIPMatcherRejectsInvalidNumericSuffix(f *testing.F) {
	f.Add(byte(56), byte(1), byte(2), byte(3))
	f.Add(byte(255), byte(255), byte(255), byte(255))

	f.Fuzz(func(t *testing.T, first, second, third, fourth byte) {
		raw := fmt.Sprintf("x=192.0.2.256.%d.%d.%d.%d", first, second, third, fourth)
		for _, input := range []string{raw, percentEncodeMatcherPropertyValue(raw)} {
			obfuscator := newStaticIPObfuscatorForProperty(t)
			if got := obfuscator.Contents(input); got != input {
				t.Fatalf("invalid numeric token %q changed to %q", input, got)
			}
		}
	})
}

func runMatcherProperties(t *testing.T, input string) matcherPropertyResult {
	t.Helper()

	ip := newStaticIPObfuscatorForProperty(t)
	mac, err := NewMacAddressObfuscator(schema.ObfuscateReplacementTypeStatic, NewSimpleTracker())
	if err != nil {
		t.Fatal(err)
	}
	domain, err := NewDomainObfuscator(
		[]string{"example.com"},
		schema.ObfuscateReplacementTypeStatic,
		NewSimpleTracker(),
	)
	if err != nil {
		t.Fatal(err)
	}

	ipContents := ip.Contents(input)
	ipPath := ip.Path(input)

	macContents := mac.Contents(input)
	macPath := mac.Path(input)

	domainContents := domain.Contents(input)
	domainPath := domain.Path(input)

	result := matcherPropertyResult{
		ipContents:     ipContents,
		ipPath:         ipPath,
		macContents:    macContents,
		macPath:        macPath,
		domainContents: domainContents,
		domainPath:     domainPath,
		ipReport:       ip.Report(),
		macReport:      mac.Report(),
		domainReport:   domain.Report(),
	}
	validateMatcherPropertyReports(t, result)
	return result
}

func validateMatcherPropertyReports(t *testing.T, result matcherPropertyResult) {
	t.Helper()
	for _, replacement := range result.ipReport.Replacements {
		if !IsCanonicalIP(replacement.Canonical) {
			t.Fatalf("report contains non-canonical IP %q", replacement.Canonical)
		}
	}
	for _, replacement := range result.macReport.Replacements {
		if !IsCanonicalMAC(replacement.Canonical) {
			t.Fatalf("report contains non-canonical MAC %q", replacement.Canonical)
		}
	}
	for _, replacement := range result.domainReport.Replacements {
		if !IsCanonicalDomain(replacement.Canonical) {
			t.Fatalf("report contains non-canonical domain %q", replacement.Canonical)
		}
	}
}

func newStaticIPObfuscatorForProperty(t *testing.T) ReportingObfuscator {
	t.Helper()
	obfuscator, err := NewIPObfuscator(schema.ObfuscateReplacementTypeStatic, NewSimpleTracker())
	if err != nil {
		t.Fatal(err)
	}
	return obfuscator
}

func randomBytes(random *rand.Rand, count int) []byte {
	values := make([]byte, count)
	for index := range values {
		values[index] = byte(random.Uint32())
	}
	return values
}

func randomMatcherInput(random *rand.Rand, length int) string {
	input := make([]byte, length)
	for index := range input {
		input[index] = matcherPropertyAlphabet[random.Uint32()%uint32(len(matcherPropertyAlphabet))]
	}
	return string(input)
}

func percentEncodeMatcherPropertyValue(value string) string {
	var encoded strings.Builder
	encoded.Grow(len(value) * 3)
	for index := 0; index < len(value); index++ {
		fmt.Fprintf(&encoded, "%%%02X", value[index])
	}
	return encoded.String()
}

func ipv4PropertyForm(octets []byte, variant int) string {
	value := fmt.Sprintf("%d.%d.%d.%d", octets[0], octets[1], octets[2], octets[3])
	switch variant % 5 {
	case 1:
		return strings.ReplaceAll(value, ".", "-")
	case 2:
		return strings.ReplaceAll(value, ".", "_")
	case 3:
		return percentEncodeMatcherPropertyValue(value)
	case 4:
		return strings.ReplaceAll(value, ".", "%2E")
	default:
		return value
	}
}

func macPropertyForm(octets []byte, variant int) string {
	lowerColon := fmt.Sprintf(
		"%02x:%02x:%02x:%02x:%02x:%02x",
		octets[0], octets[1], octets[2], octets[3], octets[4], octets[5],
	)
	upperColon := strings.ToUpper(lowerColon)
	switch variant % 5 {
	case 1:
		return strings.ReplaceAll(upperColon, ":", "-")
	case 2:
		return fmt.Sprintf(
			"%02X-%02X:%02X-%02X:%02X-%02X",
			octets[0], octets[1], octets[2], octets[3], octets[4], octets[5],
		)
	case 3:
		return percentEncodeMatcherPropertyValue(lowerColon)
	case 4:
		return strings.ReplaceAll(lowerColon, ":", "%3A")
	default:
		return upperColon
	}
}
