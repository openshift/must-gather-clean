package obfuscator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type splitObfuscator struct {
	tracker ReplacementTracker
}

func (d *splitObfuscator) Path(input string) string {
	s := strings.SplitN(input, " ", 3)[2]
	if d.tracker != nil {
		d.tracker.AddReplacement(input, s)
	}
	return s
}

func (d *splitObfuscator) Contents(input string) string {
	s := strings.SplitN(input, " ", 2)[1]
	if d.tracker != nil {
		d.tracker.AddReplacement(input, s)
	}
	return s
}

func (d *splitObfuscator) Report() ReplacementReport {
	return d.tracker.Report()
}

func TestMultiObfuscationContents(t *testing.T) {
	mo := NewMultiObfuscator(
		[]ReportingObfuscator{
			&splitObfuscator{},
			&splitObfuscator{},
		})

	contents := mo.Contents("this must be split twice")
	assert.Equal(t, "be split twice", contents)
}

func TestMultiObfuscationPaths(t *testing.T) {
	mo := NewMultiObfuscator(
		[]ReportingObfuscator{
			&splitObfuscator{},
			&splitObfuscator{},
		})

	contents := mo.Path("this must be split twice or more?")
	assert.Equal(t, "twice or more?", contents)
}

func TestMultiObfuscationReport(t *testing.T) {
	mo := NewMultiObfuscator(
		[]ReportingObfuscator{
			&splitObfuscator{tracker: NewSimpleTracker()},
		})

	contents := mo.Contents("this must be split once")
	assert.Equal(t, "must be split once", contents)
	assert.Equal(t, map[string]string{"this must be split once": "must be split once"}, mo.Report().AsMap())
}

func TestMultiObfuscationReportShouldOverride(t *testing.T) {
	mo := NewMultiObfuscator(
		[]ReportingObfuscator{
			&NoopObfuscator{map[string]string{"a": "b"}},
			&NoopObfuscator{map[string]string{"a": "c"}},
		})

	assert.Equal(t, map[string]string{"a": "c"}, mo.Report().AsMap())
}

func TestMultiObfuscationReportMulti(t *testing.T) {
	mo := NewMultiObfuscator(
		[]ReportingObfuscator{
			&splitObfuscator{tracker: NewSimpleTracker()},
			&splitObfuscator{tracker: NewSimpleTracker()},
			&splitObfuscator{tracker: NewSimpleTracker()},
		})

	contents := mo.Contents("this must be split thrice")
	assert.Equal(t, "split thrice", contents)
	assert.Equal(t, map[string]string{
		"be split thrice":           "split thrice",
		"must be split thrice":      "be split thrice",
		"this must be split thrice": "must be split thrice"}, mo.Report().AsMap())

	perObfuscator := mo.ReportPerObfuscator()
	var reportsAsMap []map[string]string
	for _, val := range perObfuscator {
		reportsAsMap = append(reportsAsMap, val.AsMap())
	}
	assert.Equal(t, []map[string]string{
		{"this must be split thrice": "must be split thrice"},
		{"must be split thrice": "be split thrice"},
		{"be split thrice": "split thrice"}}, reportsAsMap)
}
