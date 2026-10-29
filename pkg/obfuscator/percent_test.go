package obfuscator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInsidePercentEncodingAt(t *testing.T) {
	input := "%41example.com"

	assert.False(t, insidePercentEncodingAt(input, 0))
	assert.True(t, insidePercentEncodingAt(input, 1))
	assert.True(t, insidePercentEncodingAt(input, 2))
	assert.False(t, insidePercentEncodingAt(input, 3))
	assert.False(t, insidePercentEncodingAt(input, 4))
}
