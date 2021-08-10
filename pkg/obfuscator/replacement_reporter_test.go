package obfuscator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleReportingHappyPath(t *testing.T) {
	r := NewSimpleReporter()
	assert.Equal(t, map[string]string{}, r.Report())
	r.UpsertReplacement("a", "b")
	assert.Equal(t, map[string]string{"a": "b"}, r.Report())
}

func TestSimpleReportingOverwritesExistingReplacements(t *testing.T) {
	r := NewSimpleReporter()
	r.UpsertReplacement("a", "b")
	assert.Equal(t, map[string]string{"a": "b"}, r.Report())
	r.UpsertReplacement("a", "c")
	assert.Equal(t, map[string]string{"a": "c"}, r.Report())
}
