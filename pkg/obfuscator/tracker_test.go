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

	// the following testcase generates an already existing replacement
	replacement, isPresent := tracker.GenerateIfAbsent("a", nil)
	assert.Equal(t, replacement, "b")
	assert.Equal(t, isPresent, true)
	// the following testcase doesn't generate a replacement as the generator function is nil
	replacement, isPresent = tracker.GenerateIfAbsent("c", nil)
	assert.Equal(t, replacement, "")
	assert.Equal(t, isPresent, false)
	// the following testcase generates a replacement in LowerCase according to the generator function
	replacement, isPresent = tracker.GenerateIfAbsent("D", func() string { return strings.ToLower("D") })
	assert.Equal(t, replacement, "d")
	assert.Equal(t, isPresent, false)
	// The following report would not contain the above testcase as the same is not added using AddReplacement method.
	assert.Equal(t, map[string]string{"a": "b"}, tracker.Report())
}

func TestReportLeakingBack(t *testing.T) {
	tracker := NewSimpleTracker()
	tracker.AddReplacement("foo", "bar")
	mapping := tracker.Report()
	mapping["foo"] = "baz"
	replacement, _ := tracker.GenerateIfAbsent("foo", nil)
	assert.Equal(t, "bar", replacement)
}

func TestSimpleReporterInitialize(t *testing.T) {
	tracker := NewSimpleTracker()
	tracker.Initialize(map[string]string{"a": "b"})
	// the following testcase generates an already existing replacement
	replacement, isPresent := tracker.GenerateIfAbsent("a", nil)
	assert.Equal(t, replacement, "b")
	assert.Equal(t, isPresent, true)
	// the following testcase generates the known replacement irrespective of the generator
	replacement, isPresent = tracker.GenerateIfAbsent("a", func() string { return strings.ToUpper("a") })
	assert.Equal(t, replacement, "b")
	assert.Equal(t, isPresent, true)
	// the following testcase doesn't generate a replacement as the generator function is nil
	replacement, isPresent = tracker.GenerateIfAbsent("c", nil)
	assert.Equal(t, replacement, "")
	assert.Equal(t, isPresent, false)
	// the following testcase generates a replacement in LowerCase according to the generator function
	replacement, isPresent = tracker.GenerateIfAbsent("c", func() string { return strings.ToUpper("c") })
	assert.Equal(t, replacement, "C")
	assert.Equal(t, isPresent, false)
}
