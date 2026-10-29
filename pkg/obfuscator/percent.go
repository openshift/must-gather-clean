package obfuscator

import "strings"

// decodePercentEncoded decodes exactly one layer of valid percent-encoded
// bytes. Malformed escapes are preserved verbatim. A second layer is
// intentionally left encoded so callers can reason about the value that was
// actually represented by the input.
func decodePercentEncoded(input string) string {
	if !strings.Contains(input, "%") {
		return input
	}

	var decoded strings.Builder
	decoded.Grow(len(input))
	for index := 0; index < len(input); {
		if isPercentEncodingAt(input, index) {
			decoded.WriteByte(percentEncodedByteAt(input, index))
			index += 3
			continue
		}
		decoded.WriteByte(input[index])
		index++
	}
	return decoded.String()
}

func decodedByteAt(input string, index int) (byte, int, bool) {
	if index < 0 || index >= len(input) {
		return 0, 0, false
	}
	if input[index] != '%' {
		return input[index], 1, true
	}
	if !isPercentEncodingAt(input, index) {
		return 0, 0, false
	}
	return percentEncodedByteAt(input, index), 3, true
}

func decodedByteBefore(input string, index int) (byte, int, bool) {
	if index <= 0 || index > len(input) {
		return 0, 0, false
	}
	encodingIndex := index - 3
	if encodingIndex < 0 || !isPercentEncodingAt(input, encodingIndex) {
		return input[index-1], 1, true
	}
	return percentEncodedByteAt(input, encodingIndex), 3, true
}

func percentEncodedByteAt(input string, index int) byte {
	return byte(hexDigitValue(input[index+1])*16 + hexDigitValue(input[index+2]))
}

func isPercentEncodingAt(input string, index int) bool {
	return index >= 0 && index+2 < len(input) &&
		input[index] == '%' &&
		isHexDigit(input[index+1]) &&
		isHexDigit(input[index+2])
}

func isHexDigit(value byte) bool {
	return value >= '0' && value <= '9' ||
		value >= 'a' && value <= 'f' ||
		value >= 'A' && value <= 'F'
}

func hexDigitValue(value byte) int {
	switch {
	case value >= '0' && value <= '9':
		return int(value - '0')
	case value >= 'a' && value <= 'f':
		return int(value-'a') + 10
	case value >= 'A' && value <= 'F':
		return int(value-'A') + 10
	default:
		return 0
	}
}

// insidePercentEncodingAt reports whether index is one of the two hexadecimal
// digits inside a valid percent encoding. The encoding itself is atomic: a
// matcher may start at the percent sign, but not inside either hex digit.
func insidePercentEncodingAt(input string, index int) bool {
	if index <= 0 {
		return false
	}
	if isPercentEncodingAt(input, index-1) {
		return true
	}
	return index >= 2 && isPercentEncodingAt(input, index-2)
}

// percentEncodingEndAfterInterior returns the index immediately after the
// valid percent encoding containing index. If index is not inside one, it is
// returned unchanged.
func percentEncodingEndAfterInterior(input string, index int) int {
	if index > 0 && isPercentEncodingAt(input, index-1) {
		return index + 2
	}
	if index >= 2 && isPercentEncodingAt(input, index-2) {
		return index + 1
	}
	return index
}
