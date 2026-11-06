package obfuscator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsidePercentEncodingAt(t *testing.T) {
	input := "%41example.com"

	assert.False(t, insidePercentEncodingAt(input, 0))
	assert.True(t, insidePercentEncodingAt(input, 1))
	assert.True(t, insidePercentEncodingAt(input, 2))
	assert.False(t, insidePercentEncodingAt(input, 3))
	assert.False(t, insidePercentEncodingAt(input, 4))
}

func TestDecodedByteBefore(t *testing.T) {
	value, width, ok := decodedByteBefore("prefix%41suffix", 9)
	require.True(t, ok)
	assert.Equal(t, byte('A'), value)
	assert.Equal(t, 3, width)

	value, width, ok = decodedByteBefore("prefix%41suffix", 6)
	require.True(t, ok)
	assert.Equal(t, byte('x'), value)
	assert.Equal(t, 1, width)

	_, _, ok = decodedByteBefore("prefix", 0)
	assert.False(t, ok)
}

func TestPercentEncodingEndAfterInterior(t *testing.T) {
	assert.Equal(t, 3, percentEncodingEndAfterInterior("%41suffix", 1))
	assert.Equal(t, 3, percentEncodingEndAfterInterior("%41suffix", 2))
	assert.Equal(t, 4, percentEncodingEndAfterInterior("%41suffix", 4))
}

func TestDecodePercentEncodedSingleLayer(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "valid escape", input: "%41", want: "A"},
		{name: "encoded percent", input: "%25", want: "%"},
		{name: "second layer preserved", input: "%2531", want: "%31"},
		{name: "malformed hex preserved", input: "%zz", want: "%zz"},
		{name: "bare percent preserved", input: "%", want: "%"},
		{name: "incomplete escape preserved", input: "%4", want: "%4"},
		{name: "UTF-8 bytes", input: "%C3%A9", want: string([]byte{0xC3, 0xA9})},
		{name: "non-UTF-8 byte", input: "%FF", want: string([]byte{0xFF})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, decodePercentEncoded(tc.input))
		})
	}
}
