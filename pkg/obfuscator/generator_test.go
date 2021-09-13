package obfuscator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGeneratorHappyPath(t *testing.T) {
	g := newGenerator("%d", "x", 10)
	assert.Equal(t, "1", g.generateConsistentReplacement())
	assert.Equal(t, "2", g.generateConsistentReplacement())
	assert.Equal(t, "x", g.generateStaticReplacement())
}

func TestGeneratorOverLimit(t *testing.T) {
	exitCalled := false
	g := generator{
		template: "%d",
		static:   "x",
		count:    0,
		max:      1,
		exitFunc: func(s string, i int) {
			assert.Equal(t, "%d", s)
			assert.Equal(t, 1, i)
			exitCalled = true
		},
	}

	assert.Equal(t, "1", g.generateConsistentReplacement())
	assert.Equal(t, "", g.generateConsistentReplacement())
	assert.True(t, exitCalled, "should have called exit function")
}
