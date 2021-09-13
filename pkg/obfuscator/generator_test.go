package obfuscator

import (
	"fmt"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratorHappyPath(t *testing.T) {
	g, err := newGenerator("%d", "x", 10, schema.ObfuscateReplacementTypeStatic)
	require.NoError(t, err)
	assert.Equal(t, "1", g.generateConsistentReplacement())
	assert.Equal(t, "2", g.generateConsistentReplacement())
	assert.Equal(t, "x", g.generateStaticReplacement())
}

func TestInvalidGenerator(t *testing.T) {
	_, err := newGenerator("%d", "x", 10, schema.ObfuscateReplacementType("customType"))
	assert.Equal(t, err, fmt.Errorf("unsupported replacement type: %s", schema.ObfuscateReplacementType("customType")))
}

func TestGeneratorOverLimit(t *testing.T) {
	exitCalled := false
	g := generator{
		template:        "%d",
		static:          "x",
		count:           0,
		replacementType: schema.ObfuscateReplacementTypeStatic,
		max:             1,
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
