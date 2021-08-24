package obfuscator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleTrackerHappyPath(t *testing.T) {
	tracker := NewSimpleTracker()
	assert.Equal(t, map[string]string{}, tracker.Report())
	tracker.AddReplacement("a", "b")
	assert.Equal(t, map[string]string{"a": "b"}, tracker.Report())
}

func TestSimpleTrackerGetReplacement(t *testing.T) {
	tracker := NewSimpleTracker()
	tracker.AddReplacement("a", "b")
	assert.Equal(t, tracker.GenerateIfAbsent("a", "a", nil), "b")
	assert.Equal(t, tracker.GenerateIfAbsent("c", "c", nil), "")
	assert.Equal(t, tracker.GenerateIfAbsent("D", "D", strings.ToLower), "d")
	assert.Equal(t, tracker.GenerateIfAbsent("E", "F", strings.ToLower), "f")
	assert.Equal(t, map[string]string{"D": "d", "a": "b", "E": "f"}, tracker.Report())
}

func TestReportLeakingBack(t *testing.T) {
	tracker := NewSimpleTracker()
	tracker.AddReplacement("foo", "bar")
	mapping := tracker.Report()
	mapping["foo"] = "baz"
	assert.Equal(t, "bar", tracker.GenerateIfAbsent("foo", "foo", nil))
}

func TestSimpleReporterInitialize(t *testing.T) {
	tracker := NewSimpleTracker()
	tracker.Initialize(map[string]string{"a": "b"})
	assert.Equal(t, "b", tracker.GenerateIfAbsent("a", "a", nil))
	assert.Equal(t, "b", tracker.GenerateIfAbsent("a", "a", strings.ToUpper))
	assert.Equal(t, "", tracker.GenerateIfAbsent("c", "c", nil))
	assert.Equal(t, "C", tracker.GenerateIfAbsent("c", "c", strings.ToUpper))
}
