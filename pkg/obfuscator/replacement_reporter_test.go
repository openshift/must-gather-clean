package obfuscator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleReportingHappyPath(t *testing.T) {
	r := NewSimpleReporter()
	assert.Equal(t, map[string]string{}, r.ReportingResult())
	r.ReportReplacement("a", "b")
	assert.Equal(t, map[string]string{"a": "b"}, r.ReportingResult())
}

func TestSimpleReporterGetReplacement(t *testing.T) {
	r := NewSimpleReporter()
	r.ReportReplacement("a", "b")
	assert.Equal(t, r.GetReplacement("a"), "b")
	assert.Equal(t, r.GetReplacement("c"), "")
}

func TestReportLeakingBack(t *testing.T) {
	r := NewSimpleReporter()
	r.ReportReplacement("foo", "bar")
	mapping := r.ReportingResult()
	mapping["foo"] = "baz"
	assert.Equal(t, "bar", r.GetReplacement("foo"))
}
