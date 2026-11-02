package obfuscator

import (
	"net"
	"regexp"
	"sort"
	"strings"

	"github.com/openshift/must-gather-clean/pkg/schema"
)

const (
	obfuscatedStaticIPv4 = "xxx.xxx.xxx.xxx"
	obfuscatedStaticIPv6 = "xxxx:xxxx:xxxx:xxxx:xxxx:xxxx:xxxx:xxxx"

	maximumSupportedObfuscationsIP = 9999999999
	// there are 2^32 (4,294,967,296) addresses in total, we can support that with 10 characters
	consistentIPv4Template = "x-ipv4-%010d-x"
	// there are 2^128 possible v6 IPs, but we keep them down to the same amount as the v4s.
	// must-gathers today don't have any v6 IPs in them yet, so this should be enough to be future-proof
	consistentIPv6Template = "x-ipv6-%010d-x"
)

var (
	// ipv6re is not perfect. it can still catch words like :face:bad as a valid ipv6 address
	ipv6re      = `(([a-fA-F0-9]{0,4}[:]){1,8}([0-9a-fA-F]{1,4}|::)+|([a-fA-F0-9]{1,4}[:]){1,7}:)`
	ipv6Pattern = regexp.MustCompile(ipv6re)
	excludedIPs = []net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("0.0.0.0"),
		net.ParseIP("::1"),
	}
)

type ipObfuscator struct {
	ReplacementTracker
	ipv4Generator *generator
	ipv6Generator *generator
}

// ipMatch is a validated match in either a raw input string or a decoded
// percent-encoded run. IPv4-in-IPv6 values are marked as IPv6 so that they use
// the IPv6 replacement generator even though net.IP.To4 returns a non-nil
// address for some of them.
type ipMatch struct {
	start int
	end   int
	ipv6  bool
}

// ipv6Context carries the bytes and token state around a candidate. Decoded
// percent-encoding runs have no access to the original input outside the run,
// so their callers supply this context explicitly. raw also allows the common
// scanner to work directly on the original input.
type ipv6Context struct {
	leadingLabel            bool
	trailingLabel           bool
	trailingStructuredLabel bool
	trailingNonCIDRPath     bool
	structuredIdentifier    bool
	leadingByte             byte
	hasLeadingByte          bool
	raw                     bool
}

// ipReplacementMatch is a selected match in the original input. The candidate
// is the decoded value used for validation and canonicalization, while the
// original span is retained for output and report accounting.
type ipReplacementMatch struct {
	start     int
	end       int
	candidate string
	ipv6      bool
	excluded  bool
}

func (o *ipObfuscator) Path(s string) string {
	return o.replace(s)
}

func (o *ipObfuscator) Contents(s string) string {
	return o.replace(s)
}

func (o *ipObfuscator) replace(input string) string {
	matches := o.findReplacementMatches(input)
	if len(matches) == 0 {
		return input
	}

	// Selection preserves the matcher priorities, but replacement generation is
	// deliberately separate from selection. Consistent IDs must follow textual
	// order, including when an encoded value appears after a raw value.
	sort.SliceStable(matches, func(first, second int) bool {
		if matches[first].start != matches[second].start {
			return matches[first].start < matches[second].start
		}
		return matches[first].end > matches[second].end
	})

	var output strings.Builder
	output.Grow(len(input))
	lastIndex := 0
	for _, match := range matches {
		generator := o.ipv4Generator
		if match.ipv6 {
			generator = o.ipv6Generator
		}
		output.WriteString(input[lastIndex:match.start])
		if match.excluded {
			output.WriteString(input[match.start:match.end])
			lastIndex = match.end
			continue
		}
		output.WriteString(generator.generateReplacement(
			match.candidate,
			input[match.start:match.end],
			1,
			o.ReplacementTracker,
		))
		lastIndex = match.end
	}
	output.WriteString(input[lastIndex:])
	return output.String()
}

func (o *ipObfuscator) findReplacementMatches(input string) []ipReplacementMatch {
	var candidates []ipReplacementMatch
	if strings.Contains(input, "%") {
		candidates = append(candidates, o.findEncodedReplacementMatches(input)...)
	}

	containIPv4 := mightContainIPv4(input)
	containIPv6 := mightContainIPv6(input)
	var ipv4Matches []ipMatch
	if containIPv4 {
		ipv4Matches = findIPv4Matches(input)
	}
	if containIPv6 && len(ipv4Matches) > 0 && mightContainIPv4InIPv6(input) {
		candidates = append(candidates, rawIPMatches(
			input, findIPv6Matches(input, ipv6Context{raw: true}, ipv4Matches, true),
		)...)
	}
	if containIPv4 {
		candidates = append(candidates, rawIPMatches(input, ipv4Matches)...)
	}
	if containIPv6 {
		candidates = append(candidates, rawIPMatches(input, findRawIPv6Matches(input))...)
	}
	return selectIPMatches(candidates)
}

func (o *ipObfuscator) findEncodedReplacementMatches(input string) []ipReplacementMatch {
	var candidates []ipReplacementMatch
	seenRuns := make(map[int]struct{})

	processRun := func(runStart int) int {
		decoded, rawStarts, runEnd, encoded := decodeIPCandidateRun(input, runStart)
		if !encoded {
			return runEnd
		}

		context := ipv6Context{}
		if previous, _, valid := decodedByteBefore(input, runStart); valid {
			context.leadingByte, context.hasLeadingByte = previous, true
			context.leadingLabel = isIPv6FieldLabelByte(previous)
		}
		if next, _, valid := decodedByteAt(input, runEnd); valid {
			context.trailingLabel = isIPv6FieldLabelByte(next)
			context.trailingStructuredLabel = isIPv6StructuredLabelByte(next)
		}
		context.trailingNonCIDRPath = nonCIDRPathAt(input, runEnd)
		context.structuredIdentifier = insideStructuredIdentifier(input, runStart)
		for _, match := range findEncodedIPMatches(decoded, context) {
			rawStart := rawStarts[match.start]
			rawEnd := rawStarts[match.end]
			rawMatch := input[rawStart:rawEnd]
			if !strings.Contains(rawMatch, "%") {
				// Entirely raw matches are left to the raw passes. This also
				// keeps raw and encoded values in the same textual ordering.
				continue
			}
			if replacement, ok := makeIPReplacementMatch(rawMatch, decoded[match.start:match.end], rawStart, rawEnd, match.ipv6); ok {
				candidates = append(candidates, replacement)
			}
		}
		return runEnd
	}

	index := 0
	for index < len(input) {
		if input[index] != '%' {
			index++
			continue
		}
		character, width, valid := decodedByteAt(input, index)
		if !valid {
			index++
			continue
		}
		nextIndex := index + width
		if !isIPCandidateByte(character) {
			index = nextIndex
			continue
		}

		// Include a raw candidate prefix so that "fe80%3A%3A1" decodes as the
		// complete address. If that run cannot reach this encoding, as happens
		// after a non-hexadecimal field name, retry from an encoded delimiter.
		prefixStart := index
		for prefixStart > 0 {
			previous, previousWidth, valid := decodedByteBefore(input, prefixStart)
			if !valid || !isIPCandidateByte(previous) {
				break
			}
			prefixStart -= previousWidth
		}

		var runEnd int
		if _, seen := seenRuns[prefixStart]; !seen {
			seenRuns[prefixStart] = struct{}{}
			runEnd = processRun(prefixStart)
		}
		if character == ':' && prefixStart != index && runEnd <= nextIndex {
			if _, seen := seenRuns[index]; !seen {
				seenRuns[index] = struct{}{}
				runEnd = processRun(index)
			}
		}
		if runEnd == 0 {
			runEnd = nextIndex
		}
		if runEnd > nextIndex {
			index = runEnd
		} else {
			index = nextIndex
		}
	}
	return candidates
}

func rawIPMatches(input string, matches []ipMatch) []ipReplacementMatch {
	replacements := make([]ipReplacementMatch, 0, len(matches))
	for _, match := range matches {
		original := input[match.start:match.end]
		if replacement, ok := makeIPReplacementMatch(original, original, match.start, match.end, match.ipv6); ok {
			replacements = append(replacements, replacement)
		}
	}
	return replacements
}

func makeIPReplacementMatch(original, candidate string, start, end int, ipv6 bool) (ipReplacementMatch, bool) {
	canonical := canonicalIP(candidate)
	ip := parseIPValue(candidate)
	if ip == nil {
		return ipReplacementMatch{}, false
	}
	return ipReplacementMatch{
		start:     start,
		end:       end,
		candidate: canonical,
		ipv6:      ipv6,
		excluded:  isExcludedIP(ip),
	}, true
}

func selectIPMatches(candidates []ipReplacementMatch) []ipReplacementMatch {
	ipv4Ranges := make([]ipReplacementMatch, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidate.ipv6 {
			ipv4Ranges = append(ipv4Ranges, candidate)
		}
	}
	sort.SliceStable(ipv4Ranges, func(first, second int) bool {
		if ipv4Ranges[first].start != ipv4Ranges[second].start {
			return ipv4Ranges[first].start < ipv4Ranges[second].start
		}
		return ipv4Ranges[first].end > ipv4Ranges[second].end
	})

	sort.SliceStable(candidates, func(first, second int) bool {
		if candidates[first].start != candidates[second].start {
			return candidates[first].start < candidates[second].start
		}
		if candidates[first].end != candidates[second].end {
			return candidates[first].end > candidates[second].end
		}
		if candidates[first].ipv6 != candidates[second].ipv6 {
			return candidates[first].ipv6
		}
		return candidates[first].candidate < candidates[second].candidate
	})

	selected := make([]ipReplacementMatch, 0, len(candidates))
	lastEnd := -1
	for _, candidate := range candidates {
		if candidate.start < 0 || candidate.end <= candidate.start {
			continue
		}
		if candidate.ipv6 && endsInsideIPv4(candidate, ipv4Ranges) {
			// An invalid IPv6 candidate can end partway through a valid IPv4
			// tail. Replacing that prefix would hide the IPv4 match behind an
			// overlapping earlier span and leave the rest of the address in the
			// output. A complete IPv6-with-IPv4 value ends at the IPv4 end and
			// is unaffected by this check.
			continue
		}
		if candidate.start < lastEnd {
			continue
		}
		selected = append(selected, candidate)
		lastEnd = candidate.end
	}
	return selected
}

func endsInsideIPv4(candidate ipReplacementMatch, ipv4Ranges []ipReplacementMatch) bool {
	first := sort.Search(len(ipv4Ranges), func(index int) bool {
		return ipv4Ranges[index].start >= candidate.start
	})
	for index := first; index < len(ipv4Ranges) && ipv4Ranges[index].start < candidate.end; index++ {
		if candidate.end < ipv4Ranges[index].end {
			return true
		}
	}
	return false
}

// ipv6ValueAt returns the IPv6 value in a regex match. The regex can consume a
// single leading colon from a field label, such as the colon in "IP:fd69::5".
// Only that case is allowed to be trimmed: arbitrary runs of colons can occur
// in CRNs, CPE identifiers, and empty structured fields, and must not be turned
// into an IPv6 address.
func ipv6ValueAt(input string, matchStart, matchEnd int) (int, int, string, bool, bool) {
	if matchStart < 0 || matchEnd > len(input) || matchStart >= matchEnd {
		return 0, 0, "", false, true
	}
	leadingLabel := false
	leadingByte, hasLeadingByte := byte(0), false
	if previous, _, valid := decodedByteBefore(input, matchStart); valid {
		leadingByte, hasLeadingByte = previous, true
		leadingLabel = isIPv6FieldLabelByte(previous)
	}
	trailingLabel := false
	trailingStructuredLabel := false
	if next, _, valid := decodedByteAt(input, matchEnd); valid {
		trailingLabel = isIPv6FieldLabelByte(next)
		trailingStructuredLabel = isIPv6StructuredLabelByte(next)
	}
	return ipv6ValueInRun(input, matchStart, matchEnd, ipv6Context{
		leadingLabel:            leadingLabel,
		trailingLabel:           trailingLabel,
		trailingStructuredLabel: trailingStructuredLabel,
		structuredIdentifier:    insideStructuredIdentifier(input, matchStart),
		leadingByte:             leadingByte,
		hasLeadingByte:          hasLeadingByte,
	})
}

// ipv6ValueInRun is the decoded-run form of ipv6ValueAt. Percent-encoded runs
// start at the first candidate byte, so context immediately before and after
// the run has to be supplied by the caller.
func ipv6ValueInRun(run string, matchStart, matchEnd int, context ipv6Context) (int, int, string, bool, bool) {
	match := run[matchStart:matchEnd]
	leading := context.leadingByte
	hasLeading := context.hasLeadingByte
	if matchStart > 0 {
		if previous, _, valid := decodedByteBefore(run, matchStart); valid {
			leading = previous
			hasLeading = true
		}
	}
	hasLeadingLabel := context.leadingLabel
	if hasLeading {
		hasLeadingLabel = isIPv6FieldLabelByte(leading)
	}
	hasTrailingLabel := context.trailingLabel
	hasTrailingStructuredLabel := context.trailingStructuredLabel
	if matchEnd < len(run) {
		if next, _, valid := decodedByteAt(run, matchEnd); valid {
			hasTrailingLabel = isIPv6FieldLabelByte(next)
			hasTrailingStructuredLabel = isIPv6StructuredLabelByte(next)
		}
	}
	trailingNonCIDRPath := context.trailingNonCIDRPath
	if matchEnd < len(run) {
		trailingNonCIDRPath = nonCIDRPathAt(run, matchEnd)
	}
	// CPE fields commonly look like ":9::baseos". The IPv6 regexp can split
	// that field into several syntactically valid suffixes, including "::ba".
	// A CPE field is not an address value, so preserve the whole structured
	// identifier once a following label is present.
	if context.structuredIdentifier && hasTrailingLabel && strings.Contains(match, "::") {
		return 0, 0, "", false, false
	}

	// The regex can consume the final label byte before a colon, for example
	// the "e" in "field_name:fd69::5". A colon at the beginning of the match or
	// immediately after that one-byte prefix is a field delimiter. Later
	// colons can belong to an address that merely follows a label.
	relativeDelimiter := strings.IndexByte(match, ':')
	delimiter := matchStart + relativeDelimiter
	delimiterAfterPrefix := relativeDelimiter == 0 || relativeDelimiter == 1
	if (hasLeadingLabel || context.structuredIdentifier) && delimiterAfterPrefix && delimiter+1 < matchEnd {
		candidateStart := delimiter + 1
		candidate := run[candidateStart:matchEnd]
		if parseIPValue(candidate) != nil {
			if !strings.Contains(candidate, "::") {
				// A full-form value immediately after a field delimiter is
				// indistinguishable from a suffix of a longer colon-delimited
				// value. Preserve the field and let the IPv4 pass mask any
				// independently recognizable embedded tail.
				return 0, 0, "", false, false
			}
			if suspiciousStructuredIPv6Suffix(candidate, hasTrailingStructuredLabel, context.structuredIdentifier) {
				// In structured identifiers such as CPE names, the regexp can
				// split fields into a syntactically valid "9::ba" suffix. Do
				// not retry later in this match, or the scanner will accept
				// the same suffix without its field delimiter.
				return 0, 0, "", false, false
			}
			return candidateStart, matchEnd, candidate, true, true
		}
	}

	// A greedy regexp can include the delimiter before a complete address, as
	// in "host:2001:db8::1". Trim that delimiter before considering a shorter
	// compressed suffix; otherwise the fallback retains "2001:" in the output
	// and reports only "db8::1".
	if parseIPValue(match) == nil {
		if candidateStart, candidate, ok := compressedIPv6Suffix(run, matchStart, matchEnd, hasTrailingStructuredLabel, context.structuredIdentifier); ok {
			if startsInsideASCIIWord(run, candidateStart, context.leadingByte, context.hasLeadingByte) {
				// The greedy regexp can enter a hexadecimal label a few bytes
				// before its delimiter. Leaving that prefix outside the token
				// damages identifiers such as "abcdef:2001:db8::1".
				return 0, 0, "", false, true
			}
			return candidateStart, matchEnd, candidate, true, true
		}
	}

	// A valid address remains a valid address when a log field or a textual
	// label is adjacent to it. The labels are outside the returned span and
	// therefore remain untouched in the output.
	if parseIPValue(match) != nil {
		if suspiciousStructuredIPv6Suffix(match, hasTrailingStructuredLabel, context.structuredIdentifier) {
			return 0, 0, "", false, false
		}
		if !strings.Contains(match, "::") && startsAfterIPv6GroupPrefix(run, matchStart) {
			// Dropping a short hexadecimal group from an overlong
			// colon-delimited run must not turn its suffix into a separate
			// IPv6 value. In "1:2:3:4:5:6:7:192.0.2.1", the IPv4 pass still
			// masks the independently recognizable tail.
			return 0, 0, "", false, false
		}
		if isShortTerminalCompressedIPv6(match) && trailingNonCIDRPath {
			// D-Bus object paths such as
			// ".../ActiveConnection/1::/run/NetworkManager/..." are not
			// IPv6 values. A numeric CIDR suffix remains eligible for masking.
			return 0, 0, "", false, false
		}
		if isShortTerminalCompressedIPv6(match) && hasLeading &&
			(leading == '-' || leading == '_' || leading == '.') {
			// In identifiers such as "client-ca::/etc/tls" and "foo-a::/64",
			// the final label can independently parse as a compressed IPv6
			// address. Keep the identifier intact because the separator does
			// not establish an address-token boundary.
			return 0, 0, "", false, false
		}
		if match[0] == ':' {
			// A complete address that starts with "::" is unambiguous even
			// after a field name or delimiter. Only reject it when the tail is
			// still part of a textual word such as "foo::bar".
			if startsInsideTextualWord(run, matchStart, matchEnd, context.leadingByte, context.hasLeadingByte, hasTrailingStructuredLabel) {
				return 0, 0, "", false, false
			}
		} else if startsInsideASCIIWord(run, matchStart, context.leadingByte, context.hasLeadingByte) {
			// A valid suffix inside an ASCII word is part of an identifier.
			// Retry at the next colon so a label such as "abcdef:" remains
			// intact and the address after its delimiter is masked.
			return 0, 0, "", false, true
		}
		return matchStart, matchEnd, match, true, true
	}
	// Retry only when the regexp entered a field label or a word. Retrying
	// inside an otherwise complete colon-delimited numeric run would turn
	// "1:2:3:4:5:6:7:8:9" into the misleading suffix "2:3:4:5:6:7:8:9".
	return 0, 0, "", false, hasLeadingLabel ||
		startsInsideASCIIWord(run, matchStart, context.leadingByte, context.hasLeadingByte) ||
		strings.ContainsAny(match, "._-")
}

// compressedIPv6Suffix handles a regexp match that contains too many groups.
// The regexp is greedy, so a suffix such as "db8::1" can be hidden inside an
// invalid candidate. Anchor the fallback at the group immediately before the
// last "::" instead of accepting the first shorter suffix found by a retry from
// every colon. That keeps the structural prefix intact and masks the value at
// the right edge of the candidate.
// startsAfterIPv6GroupPrefix reports whether a candidate begins immediately
// after a complete hexadecimal IPv6 group. A regexp retry can expose such a
// suffix in an overlong, invalid run; the suffix must remain part of that run.
func startsAfterIPv6GroupPrefix(input string, start int) bool {
	if start < 2 || start > len(input) || input[start-1] != ':' {
		return false
	}
	groupStart := start - 1
	for groupStart > 0 && start-1-groupStart < 4 && isHexDigit(input[groupStart-1]) {
		groupStart--
	}
	length := start - 1 - groupStart
	return length > 0 && length <= 4 && (groupStart == 0 || input[groupStart-1] == ':')
}

func compressedIPv6Suffix(
	input string,
	matchStart, matchEnd int,
	trailingLabel, structuredIdentifier bool,
) (int, string, bool) {
	if matchStart < 0 || matchEnd > len(input) || matchStart >= matchEnd {
		return 0, "", false
	}
	colonRunEnd := matchStart
	for colonRunEnd < matchEnd && input[colonRunEnd] == ':' {
		colonRunEnd++
	}
	if colonRunEnd-matchStart > 1 {
		// ":::" is punctuation, not an address with a recoverable prefix.
		return 0, "", false
	}

	compression := strings.LastIndex(input[matchStart:matchEnd], "::")
	if compression < 0 {
		return 0, "", false
	}
	compression += matchStart
	candidateStart := compression
	for candidateStart > matchStart && compression-candidateStart < 4 && isHexDigit(input[candidateStart-1]) {
		candidateStart--
	}
	if candidateStart == compression {
		return 0, "", false
	}
	candidate := input[candidateStart:matchEnd]
	if parseIPValue(candidate) == nil || suspiciousStructuredIPv6Suffix(candidate, trailingLabel, structuredIdentifier) {
		return 0, "", false
	}
	return candidateStart, candidate, true
}

// recoverAdjacentIPv6Matches handles values such as
// "2001:db8::1:db8::1". A single colon both terminates the first address and
// separates it from the second, so the greedy regexp cannot expose two
// independently parseable spans. A complete partition into IP addresses is a
// much stronger signal than a syntactically valid suffix inside a CRN or a
// run of punctuation.
func recoverAdjacentIPv6Matches(
	input string,
	matchStart, matchEnd int,
	trailingStructuredLabel, structuredIdentifier bool,
	externalLeadingByte byte,
	hasExternalLeadingByte bool,
) []ipMatch {
	if parseIPValue(input[matchStart:matchEnd]) != nil {
		return nil
	}

	memo := make(map[int][]ipMatch)
	var recoverTail func(start int) []ipMatch
	recoverTail = func(start int) []ipMatch {
		if start >= matchEnd {
			return nil
		}
		if cached, ok := memo[start]; ok {
			return cached
		}
		if recoverableIPv6Token(input, start, matchEnd, true, trailingStructuredLabel, structuredIdentifier) {
			memo[start] = []ipMatch{{start: start, end: matchEnd, ipv6: true}}
			return memo[start]
		}
		for separator := start + 1; separator < matchEnd; separator++ {
			if !isSingleColonSeparator(input, separator, matchStart, matchEnd) ||
				!recoverableIPv6Token(input, start, separator, false, false, structuredIdentifier) {
				continue
			}
			rest := recoverTail(separator + 1)
			if len(rest) == 0 {
				continue
			}
			memo[start] = append([]ipMatch{{start: start, end: separator, ipv6: true}}, rest...)
			return memo[start]
		}
		memo[start] = nil
		return nil
	}

	for separator := matchStart + 1; separator < matchEnd; separator++ {
		if !isSingleColonSeparator(input, separator, matchStart, matchEnd) ||
			!recoverableIPv6Token(input, matchStart, separator, false, false, structuredIdentifier) {
			continue
		}
		rest := recoverTail(separator + 1)
		if len(rest) == 0 {
			continue
		}
		first := ipMatch{start: matchStart, end: separator, ipv6: true}
		if input[matchStart] == ':' && startsInsideTextualWord(
			input, matchStart, separator, externalLeadingByte, hasExternalLeadingByte, false,
		) {
			continue
		}
		return append([]ipMatch{first}, rest...)
	}
	return nil
}

func isSingleColonSeparator(input string, offset, matchStart, matchEnd int) bool {
	return offset > matchStart && offset+1 < matchEnd &&
		input[offset] == ':' && input[offset-1] != ':' && input[offset+1] != ':'
}

func recoverableIPv6Token(
	input string,
	start, end int,
	final bool,
	trailingStructuredLabel, structuredIdentifier bool,
) bool {
	token := input[start:end]
	if parseIPValue(token) == nil || strings.Trim(token, ":") == "" {
		return false
	}
	return !suspiciousStructuredIPv6Suffix(token, final && trailingStructuredLabel, structuredIdentifier)
}

// suspiciousStructuredIPv6Suffix rejects a valid-looking suffix that the
// regexp split out of a colon-delimited label, such as the "9::ba" prefix of
// ":9::baseos". Replacing only the prefix would corrupt the field while leaving
// part of its text behind. Values followed by punctuation or ending in a
// complete numeric group remain ordinary addresses.
func suspiciousStructuredIPv6Suffix(candidate string, trailingLabel, structuredIdentifier bool) bool {
	if len(candidate) < 2 || candidate[0] == ':' || strings.Contains(candidate, ".") {
		return false
	}
	if structuredIdentifier && isDecimalIPv6FieldSequence(candidate) {
		// CPE and CRN values are colon-delimited. A sequence of eight short
		// decimal fields is indistinguishable from a full-form IPv6 address,
		// so preserving the structured value avoids corrupting valid metadata.
		return true
	}
	if structuredIdentifier && strings.HasSuffix(candidate, "::") {
		// A terminal CPE field is commonly written as ":9::". The empty
		// final field is syntax, not the compressed tail of an IPv6 value.
		return true
	}
	if !trailingLabel {
		return false
	}
	if structuredIdentifier || strings.HasSuffix(candidate, "::") {
		return true
	}
	lastGroup := candidate[strings.LastIndexByte(candidate, ':')+1:]
	for index := 0; index < len(lastGroup); index++ {
		character := lastGroup[index]
		if character >= 'A' && character <= 'F' || character >= 'a' && character <= 'f' {
			return true
		}
	}
	return false
}

// isDecimalIPv6FieldSequence reports a full-form sequence whose groups all
// consist only of decimal digits. Within CPE and CRN values these are ordinary
// fields; IPv6 values in those grammars remain recognizable when they use
// hexadecimal digits or compression.
func isDecimalIPv6FieldSequence(candidate string) bool {
	if strings.Contains(candidate, "::") {
		return false
	}
	groups := strings.Split(candidate, ":")
	if len(groups) != 8 {
		return false
	}
	for _, group := range groups {
		if len(group) == 0 || len(group) > 4 {
			return false
		}
		for index := 0; index < len(group); index++ {
			if group[index] < '0' || group[index] > '9' {
				return false
			}
		}
	}
	return true
}

func isIPv6StructuredLabelByte(value byte) bool {
	return value >= 'A' && value <= 'Z' ||
		value >= 'a' && value <= 'z' ||
		value >= '0' && value <= '9'
}

// startsInsideTextualWord rejects a bare compressed address that is a prefix of
// a longer textual word, such as the "::ba" prefix of "foo::bar". Replacing only
// that prefix would corrupt the surrounding word and report a value that was
// never present as an address. An address at the end of a word or followed by
// a field separator is complete and must still be replaced.
func startsInsideTextualWord(
	input string,
	matchStart, matchEnd int,
	externalLeadingByte byte,
	hasExternalLeadingByte bool,
	externalTrailingStructured bool,
) bool {
	if matchStart == 0 {
		if !hasExternalLeadingByte {
			return false
		}
	} else {
		previous, _, valid := decodedByteBefore(input, matchStart)
		if valid {
			externalLeadingByte = previous
		}
	}
	previous := externalLeadingByte
	startsInsideWord := previous >= 0x80 ||
		previous >= 'A' && previous <= 'Z' ||
		previous >= 'a' && previous <= 'z' && !isHexDigit(previous) ||
		previous == '_'
	if !startsInsideWord {
		return false
	}
	if matchEnd >= len(input) {
		// Encoded runs stop before a non-IP candidate byte, so the byte
		// immediately after such a run is supplied by the caller.
		return externalTrailingStructured
	}
	next := input[matchEnd]
	return next != '_' && next != '-' && isIPv6FieldLabelByte(next)
}

func startsInsideASCIIWord(input string, matchStart int, externalLeadingByte byte, hasExternalLeadingByte bool) bool {
	previous := externalLeadingByte
	if matchStart > 0 {
		if decoded, _, valid := decodedByteBefore(input, matchStart); valid {
			previous = decoded
			hasExternalLeadingByte = true
		}
	}
	if !hasExternalLeadingByte {
		return false
	}
	return previous >= 0x80 ||
		previous >= 'A' && previous <= 'Z' ||
		previous >= 'a' && previous <= 'z' ||
		previous >= '0' && previous <= '9'
}

// insideStructuredIdentifier reports whether offset is inside a CPE or CRN
// value. In those grammars a colon-delimited field can look exactly like the
// prefix of an IPv6 address, so the matcher uses the surrounding identifier to
// distinguish a field boundary from a suffix of a longer numeric run.
func insideStructuredIdentifier(input string, offset int) bool {
	if offset <= 0 || offset > len(input) {
		return false
	}
	start := offset
	const maximumTokenScan = 512
	for start > 0 && offset-start < maximumTokenScan {
		character := input[start-1]
		if character == ' ' || character == '\t' || character == '\n' || character == '\r' ||
			character == '\'' || character == '"' || character == '=' || character == ',' ||
			character == ';' || character == '[' || character == '{' {
			break
		}
		start--
	}
	token := lowerASCIIString(decodePercentEncoded(input[start:offset]))
	return strings.HasPrefix(token, "cpe:") || strings.HasPrefix(token, "crn:")
}

func isIPv6FieldLabelByte(value byte) bool {
	return value >= 'A' && value <= 'Z' ||
		value >= 'a' && value <= 'z' ||
		value >= '0' && value <= '9' ||
		value == '-' ||
		value == '_'
}

func isShortTerminalCompressedIPv6(candidate string) bool {
	if !strings.HasSuffix(candidate, "::") {
		return false
	}
	group := candidate[:len(candidate)-2]
	if len(group) == 0 || len(group) > 4 {
		return false
	}
	for index := 0; index < len(group); index++ {
		if !isHexDigit(group[index]) {
			return false
		}
	}
	return true
}

// nonCIDRPathAt reports a filesystem-looking path immediately after an IP
// value. CIDR suffixes (for example "/64") deliberately do not match.
func nonCIDRPathAt(input string, offset int) bool {
	if offset < 0 || offset >= len(input) {
		return false
	}
	next, width, valid := decodedByteAt(input, offset)
	if !valid || next != '/' {
		return false
	}
	afterSlash, _, valid := decodedByteAt(input, offset+width)
	return !valid || afterSlash < '0' || afterSlash > '9'
}

// matchIPv4At reproduces the boundaries of the regular expression this code
// used before the manual parser. In particular, an invalid final octet is
// still consumed so that it cannot hide a later match. A value cannot start
// inside an existing numeric run: in "192.0.2.256.1.2.3", the suffix
// "56.1.2.3" belongs to the malformed address rather than being a separate IP.
func matchIPv4At(input string, start int) (int, bool) {
	if startsInsideIPv4NumericToken(input, start) {
		return 0, false
	}
	index, ok := readIPv4OctetAt(input, start, true)
	if !ok || index == len(input) {
		return 0, false
	}

	dotted := input[index] == '.'
	if !dotted && input[index] != '_' && input[index] != '-' {
		return 0, false
	}
	index++

	for group := 0; group < 2; group++ {
		index, ok = readIPv4OctetAt(input, index, true)
		if !ok || index == len(input) {
			return 0, false
		}
		separator := input[index]
		if dotted && separator != '.' {
			return 0, false
		}
		if !dotted && separator != '_' && separator != '-' {
			return 0, false
		}
		index++
	}

	index, ok = readIPv4OctetAt(input, index, true)
	if !ok {
		return 0, false
	}
	return index, true
}

func startsInsideIPv4NumericToken(input string, start int) bool {
	previous, previousWidth, valid := decodedByteBefore(input, start)
	if !valid {
		return false
	}
	if previous >= '0' && previous <= '9' {
		return true
	}
	if previous >= 'A' && previous <= 'Z' || previous >= 'a' && previous <= 'z' || previous >= 0x80 {
		// A digit directly after a word starts inside the textual label, as in
		// "node1.10.0.0.2"; extracting "1.10.0.0" would leave the final ".2".
		return startsInsideTextualLabel(input, start)
	}
	if previous == '.' {
		beforeDot, _, valid := decodedByteBefore(input, start-previousWidth)
		if valid && beforeDot >= '0' && beforeDot <= '9' {
			// A further dotted numeric group on the left means this candidate is
			// a suffix of that token, unless the token already ends in a complete
			// IPv4 value. This keeps invalid suffixes such as 192.0.2.256.1.2.3
			// intact while recognizing adjacent addresses such as
			// 0.0.0.1.0.0.0.1. The latter were replaced independently by the
			// regexp-based implementation and remain separate values here.
			return !validIPv4EndingBeforeSeparator(input, start-previousWidth)
		}
		if valid && (beforeDot == '.' || beforeDot == '_' || beforeDot == '-') {
			// Repeated separators delimit tokens. In "0.0.0.1..0.0.0.1" the
			// right-hand address is independent even though the preceding
			// character is also a dot.
			return false
		}
	}
	if previous != '.' && previous != '_' && previous != '-' {
		return false
	}
	return numericTokenEndsBeforeSeparator(input, start-previousWidth)
}

// validIPv4EndingBeforeSeparator reports whether the dotted numeric token
// immediately before separatorEnd is exactly one valid IPv4 value. The caller
// only uses this to distinguish adjacent complete addresses from suffixes of
// longer numeric tokens.
func validIPv4EndingBeforeSeparator(input string, separatorEnd int) bool {
	if separatorEnd <= 0 || input[separatorEnd-1] < '0' || input[separatorEnd-1] > '9' {
		return false
	}
	const maximumIPv4Length = 15
	earliest := separatorEnd - maximumIPv4Length
	if earliest < 0 {
		earliest = 0
	}
	for start := earliest; start < separatorEnd; start++ {
		if input[start] < '0' || input[start] > '9' {
			continue
		}
		candidate := input[start:separatorEnd]
		groups := strings.Split(candidate, ".")
		if len(groups) != 4 {
			continue
		}
		valid := true
		for _, group := range groups {
			if len(group) == 0 || len(group) > 3 || len(group) > 1 && group[0] == '0' {
				valid = false
				break
			}
		}
		if valid && net.ParseIP(candidate) != nil {
			return true
		}
	}
	return false
}

func startsInsideTextualLabel(input string, offset int) bool {
	const maximumScan = 64
	for scanned := 0; offset > 0 && scanned < maximumScan; scanned++ {
		previous, width, valid := decodedByteBefore(input, offset)
		if !valid {
			return true
		}
		offset -= width
		if width == 1 && input[offset] == '%' {
			// A malformed escape is still a token delimiter. Do not reject a
			// valid address after input such as "bad=%zz192.0.2.1".
			return false
		}
		switch {
		case previous >= 'A' && previous <= 'Z',
			previous >= 'a' && previous <= 'z',
			previous >= '0' && previous <= '9',
			previous == '_', previous == '-', previous >= 0x80:
			continue
		default:
			if previous == '\\' {
				// A single-letter label preceded by a backslash is a regex or
				// string escape, as in the AWK snippet `/\s192.0.2.1/`. The
				// address is complete even though its first digit follows the
				// letter in the escape sequence.
				return false
			}
			return true
		}
	}
	return true
}

// numericTokenEndsBeforeSeparator distinguishes an alternate-separator numeric
// run, as in "2021_09_13_14_57_42", from a textual label such as
// "node1-10.0.0.1". A dotted run before the separator is address-like: whether
// valid or malformed as a whole, it cannot absorb a separate value joined by
// "_" or "-". The scan is bounded because it only needs enough context to rule
// out a textual prefix.
func numericTokenEndsBeforeSeparator(input string, offset int) bool {
	const maximumScan = 64
	sawDigit := false
	sawDot := false
	colons := 0
	previousWasSeparator := false
	pendingDot := false
	for scanned := 0; offset > 0 && scanned < maximumScan; scanned++ {
		previous, width, valid := decodedByteBefore(input, offset)
		if !valid {
			return false
		}
		offset -= width
		switch {
		case previous >= '0' && previous <= '9':
			sawDigit = true
			sawDot = sawDot || pendingDot
			pendingDot = false
			previousWasSeparator = false
		case previous == '.':
			if previousWasSeparator {
				return sawDigit && !sawDot && colons < 2
			}
			pendingDot = true
			previousWasSeparator = true
		case previous == '_' || previous == '-':
			if previousWasSeparator {
				return sawDigit && !sawDot && colons < 2
			}
			pendingDot = false
			previousWasSeparator = true
		case previous == ':':
			colons++
			pendingDot = false
		case previous >= 'A' && previous <= 'Z' || previous >= 'a' && previous <= 'z' || previous >= 0x80:
			// A letter immediately before the numeric run means this is a
			// textual label ending in digits, not a malformed numeric token.
			// A single colon before the run is a field boundary; two or more
			// colons mean the run is the tail of an IPv6 value.
			return colons == 1
		default:
			return sawDigit && !sawDot && colons < 2
		}
	}
	return sawDigit && !sawDot && colons < 2
}

// readIPv4OctetAt reads one to three digits that must be followed by a
// separator. A four-digit run is rejected so the caller can retry from a later
// digit. Any octet can be zero; two- and three-digit octets cannot have a
// leading zero, matching the old regex boundaries with the necessary exception
// for a valid first octet of zero.
func readIPv4OctetAt(input string, index int, allowZero bool) (int, bool) {
	end := index
	for end < len(input) && end-index < 4 && input[end] >= '0' && input[end] <= '9' {
		end++
	}
	length := end - index
	if length == 0 || length > 3 {
		return 0, false
	}
	if length == 1 {
		return end, allowZero || input[index] != '0'
	}
	if length == 2 {
		return end, input[index] != '0'
	}
	if input[index] == '0' || input[index] > '2' {
		return 0, false
	}
	if input[index] == '1' {
		return end, true
	}
	return end, input[index+1] < '5' || (input[index+1] == '5' && input[index+2] <= '5')
}

// mightContainIPv4 reports whether the IPv4 replacement regexp can possibly
// match. A match needs four numeric groups in a run that contains either three
// dots or three underscore/hyphen separators. It is a prefilter, not an IPv4
// parser: false negatives would change obfuscation behavior, while false
// positives only fall back to the regexp.
func mightContainIPv4(input string) bool {
	groups := 0
	dots := 0
	otherSeparators := 0
	inGroup := false
	for index := 0; index < len(input); index++ {
		character := input[index]
		switch {
		case character >= '0' && character <= '9':
			if !inGroup {
				groups++
				inGroup = true
				if groups >= 4 && (dots >= 3 || otherSeparators >= 3) {
					return true
				}
			}
		case character == '.':
			dots++
			inGroup = false
		case character == '_' || character == '-':
			otherSeparators++
			inGroup = false
		default:
			groups = 0
			dots = 0
			otherSeparators = 0
			inGroup = false
		}
	}
	return false
}

// mightContainIPv6 reports whether the IPv6 replacement regexp can possibly
// produce a value accepted by net.ParseIP. Such a value contains either an
// ellipsis (::), eight hexadecimal groups, or the seven hexadecimal groups
// before the first octet of a full-form IPv6 address with an embedded IPv4
// tail. Like the IPv4 prefilter, this is only a conservative rejection test.
func mightContainIPv6(input string) bool {
	groups := 0
	inGroup := false
	inIPv4Tail := false
	tailSeparators := 0
	for index := 0; index < len(input); index++ {
		character := input[index]
		if isHexDigit(character) {
			if !inGroup {
				if inIPv4Tail {
					inGroup = true
					continue
				}
				groups++
				inGroup = true
				if groups == 8 {
					return true
				}
				if groups == 7 {
					inIPv4Tail = true
					tailSeparators = 0
				}
			}
			continue
		}
		if character == ':' {
			if inGroup {
				inGroup = false
				if groups == 7 {
					// A colon after the seventh group means this is a regular
					// IPv6 group, not the first octet of an embedded IPv4 tail.
					inIPv4Tail = false
				}
				continue
			}
			if index > 0 && input[index-1] == ':' {
				return true
			}
			if groups == 0 {
				continue
			}
		}
		if inIPv4Tail && inGroup &&
			(character == '.' || character == '_' || character == '-') {
			inGroup = false
			tailSeparators++
			if tailSeparators == 3 {
				return true
			}
			continue
		}
		groups = 0
		inGroup = false
		inIPv4Tail = false
		tailSeparators = 0
	}
	return false
}

// mightContainIPv4InIPv6 reports whether a contiguous IP candidate token
// contains both an IPv6 delimiter and an IPv4 separator. An embedded IPv4
// value always satisfies both conditions. False positives only trigger the
// more expensive pass that validates this form.
func mightContainIPv4InIPv6(input string) bool {
	colons := 0
	tailSeparator := false
	for index := 0; index < len(input); index++ {
		character := input[index]
		if !isIPCandidateByte(character) {
			colons = 0
			tailSeparator = false
			continue
		}
		switch character {
		case ':':
			colons++
		case '.', '_', '-':
			tailSeparator = true
		}
		if colons >= 2 && tailSeparator {
			return true
		}
	}
	return false
}

func decodeIPCandidateRun(input string, start int) (string, []int, int, bool) {
	decoded := make([]byte, 0, 45)
	rawStarts := make([]int, 0, cap(decoded)+1)
	index := start
	for index < len(input) {
		character, width, valid := decodedByteAt(input, index)
		if !valid || !isIPCandidateByte(character) {
			break
		}
		decoded = append(decoded, character)
		rawStarts = append(rawStarts, index)
		index += width
	}
	rawStarts = append(rawStarts, index)
	return string(decoded), rawStarts, index, strings.Contains(input[start:index], "%")
}

func findEncodedIPMatches(decoded string, context ipv6Context) []ipMatch {
	// A complete IPv6 value with an embedded IPv4 tail must win over the
	// standalone IPv4 tail. Otherwise replacing the tail leaves an IPv6 prefix
	// in the output.
	ipv4Matches := findIPv4Matches(decoded)
	var matches []ipMatch
	if mightContainIPv4InIPv6(decoded) {
		matches = append(matches, findIPv6Matches(decoded, context, ipv4Matches, true)...)
	}
	matches = append(matches, ipv4Matches...)
	matches = append(matches, findIPv6Matches(decoded, context, nil, false)...)
	return matches
}

func findIPv4Matches(input string) []ipMatch {
	var matches []ipMatch
	for start := 0; start < len(input); start++ {
		if insidePercentEncodingAt(input, start) {
			continue
		}
		end, found := matchIPv4At(input, start)
		if !found {
			continue
		}
		matches = append(matches, ipMatch{start: start, end: end})
	}
	return matches
}

func findRawIPv6Matches(input string) []ipMatch {
	return findIPv6Matches(input, ipv6Context{raw: true}, nil, false)
}

func ipMatchEnds(matches []ipMatch) []int {
	ends := make([]int, 0, len(matches))
	for _, match := range matches {
		ends = append(ends, match.end)
	}
	return ends
}

// findIPv6Matches scans raw input or a decoded percent-encoding run. When
// withIPv4 is true, it only accepts IPv6 values extended by a validated IPv4
// tail. The two modes remain separate because an embedded IPv4 value has to
// win over its standalone tail during match selection.
func findIPv6Matches(input string, context ipv6Context, ipv4Matches []ipMatch, withIPv4 bool) []ipMatch {
	var matches []ipMatch
	scanIPv6RegexMatches(input, func(matchStart, matchEnd int) ([]int, bool) {
		if context.raw && insidePercentEncodingAt(input, matchStart) {
			// Keep a percent triplet atomic when the regexp starts inside it.
			matchStart = percentEncodingEndAfterInterior(input, matchStart)
			if matchStart >= matchEnd {
				return nil, true
			}
		}
		if withIPv4 {
			var consumed []int
			firstIPv4 := sort.Search(len(ipv4Matches), func(index int) bool {
				return ipv4Matches[index].start >= matchStart
			})
			for index := firstIPv4; index < len(ipv4Matches) && ipv4Matches[index].start <= matchEnd; index++ {
				ipv4Match := ipv4Matches[index]
				if !ipv4MatchExtendsIPv6Regex(input, []int{matchStart, matchEnd}, ipv4Match) {
					continue
				}
				candidateStart, candidateEnd, _, ok, retry := ipv6ValueForScan(
					input, matchStart, ipv4Match.end, context,
				)
				if !ok {
					if !retry {
						return nil, false
					}
					continue
				}
				matches = append(matches, ipMatch{start: candidateStart, end: candidateEnd, ipv6: true})
				consumed = append(consumed, candidateEnd)
			}
			return consumed, true
		}

		// A single valid prefix inside a longer colon-delimited candidate is not
		// enough evidence that the suffix is a separate address. For example,
		// "1:2:3:4:5:6:7:8:9" and CRN/CPE fields contain valid-looking prefixes.
		// Only a complete partition into adjacent addresses is safe to recover.
		recovered := recoverAdjacentIPv6Matches(
			input, matchStart, matchEnd, context.trailingStructuredLabel,
			context.structuredIdentifier, context.leadingByte, context.hasLeadingByte,
		)
		if len(recovered) > 0 {
			matches = append(matches, recovered...)
			return ipMatchEnds(recovered), true
		}
		candidateStart, candidateEnd, _, ok, retry := ipv6ValueForScan(input, matchStart, matchEnd, context)
		if !ok {
			return nil, retry
		}
		matches = append(matches, ipMatch{start: candidateStart, end: candidateEnd, ipv6: true})
		return []int{candidateEnd}, true
	})
	return matches
}

func ipv6ValueForScan(input string, matchStart, matchEnd int, context ipv6Context) (int, int, string, bool, bool) {
	if context.raw {
		return ipv6ValueAt(input, matchStart, matchEnd)
	}
	return ipv6ValueInRun(input, matchStart, matchEnd, context)
}

// scanIPv6RegexMatches advances through IPv6 candidates even when a candidate
// is rejected after validation. FindAllStringIndex only returns non-overlapping
// matches, which can hide a valid suffix inside an invalid colon-heavy run.
// The regexp itself bounds each candidate length, so retrying inside a rejected
// span remains linear for ordinary input. A visit can disable retries when a
// validated-looking suffix belongs to a structured identifier.
func scanIPv6RegexMatches(input string, visit func(start, end int) ([]int, bool)) {
	for searchStart := 0; searchStart < len(input); {
		relativeMatch := ipv6Pattern.FindStringIndex(input[searchStart:])
		if relativeMatch == nil {
			return
		}
		start := searchStart + relativeMatch[0]
		end := searchStart + relativeMatch[1]
		consumedEnds, retry := visit(start, end)
		if len(consumedEnds) == 0 {
			if !retry {
				searchStart = end
				continue
			}
			if input[start] == ':' {
				colonRunEnd := start
				for colonRunEnd < end && input[colonRunEnd] == ':' {
					colonRunEnd++
				}
				if colonRunEnd-start > 1 {
					// A run of two or more colons is punctuation or an
					// invalid delimiter sequence. Retrying after its first
					// colon turns values such as ":::9445" into addresses.
					searchStart = end
					continue
				}
			}
			searchStart = start + 1
			continue
		}
		nextStart := end
		for _, consumedEnd := range consumedEnds {
			if consumedEnd > nextStart {
				nextStart = consumedEnd
			}
		}
		searchStart = nextStart
	}
}

func ipv4MatchExtendsIPv6Regex(input string, regexMatch []int, ipv4Match ipMatch) bool {
	if len(regexMatch) != 2 {
		return false
	}
	if ipv4Match.start < regexMatch[0] || ipv4Match.start > regexMatch[1] || ipv4Match.end <= regexMatch[1] {
		return false
	}
	prefix := input[regexMatch[0]:ipv4Match.start]
	// At least two colons are required for an IPv4-embedded IPv6 value. This
	// prevents an arbitrary field delimiter before a plain IPv4 address, as in
	// a CPE suffix, from being treated as an IPv6 prefix.
	return strings.Count(prefix, ":") >= 2
}

func isIPCandidateByte(value byte) bool {
	return isHexDigit(value) || value == ':' || value == '.' || value == '_' || value == '-'
}

func canonicalIP(value string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(value, "_", "."), "-", "."))
}

// parseIPValue applies the same alternate-separator normalization used for
// report keys before validating internally discovered values. This keeps an
// IPv6 value with an underscore or hyphen embedded IPv4 tail from being
// reduced to its IPv4 suffix.
func parseIPValue(value string) net.IP {
	return net.ParseIP(canonicalIP(value))
}

func isExcludedIP(ip net.IP) bool {
	for _, excluded := range excludedIPs {
		if ip.Equal(excluded) {
			return true
		}
	}
	return false
}

func NewIPObfuscator(replacementType schema.ObfuscateReplacementType, tracker ReplacementTracker) (ReportingObfuscator, error) {
	genIPv4, err := newGenerator(consistentIPv4Template, obfuscatedStaticIPv4, maximumSupportedObfuscationsIP, replacementType)
	if err != nil {
		return nil, err
	}
	genIPv6, err := newGenerator(consistentIPv6Template, obfuscatedStaticIPv6, maximumSupportedObfuscationsIP, replacementType)
	if err != nil {
		return nil, err
	}
	return &ipObfuscator{
		ReplacementTracker: tracker,
		ipv4Generator:      genIPv4,
		ipv6Generator:      genIPv6,
	}, nil
}
