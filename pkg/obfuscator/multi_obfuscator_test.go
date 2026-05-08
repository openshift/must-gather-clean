package obfuscator

import (
	"strings"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

type splitObfuscator struct {
	tracker ReplacementTracker
}

func (d *splitObfuscator) Path(input string) string {
	s := strings.SplitN(input, " ", 3)[2]
	if d.tracker != nil {
		d.tracker.GenerateIfAbsent(input, input, 1, func() string {
			return s
		})
	}
	return s
}

func (d *splitObfuscator) Contents(input string) string {
	s := strings.SplitN(input, " ", 2)[1]
	if d.tracker != nil {
		d.tracker.GenerateIfAbsent(input, input, 1, func() string {
			return s
		})
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

func TestSeedableObfuscators_DirectSeedable(t *testing.T) {
	azureObs, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
	require.NoError(t, err)
	mo := NewMultiObfuscator([]ReportingObfuscator{azureObs})
	seedable := mo.SeedableObfuscators()
	require.Len(t, seedable, 1)
}

func TestSeedableObfuscators_SingleWrapped(t *testing.T) {
	azureObs, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
	require.NoError(t, err)
	wrapped := NewTargetObfuscator(schema.ObfuscateTargetFileContents, azureObs)
	mo := NewMultiObfuscator([]ReportingObfuscator{wrapped})
	seedable := mo.SeedableObfuscators()
	require.Len(t, seedable, 1)
}

func TestSeedableObfuscators_DoubleWrapped(t *testing.T) {
	azureObs, err := NewAzureResourceObfuscator(schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker(), ptr.To(1))
	require.NoError(t, err)
	wrapped := NewTargetObfuscator(schema.ObfuscateTargetFileContents, azureObs)
	doubleWrapped := NewTargetObfuscator(schema.ObfuscateTargetAll, wrapped)
	mo := NewMultiObfuscator([]ReportingObfuscator{doubleWrapped})
	seedable := mo.SeedableObfuscators()
	require.Len(t, seedable, 1)
}

func TestSeedableObfuscators_NonSeedableSkipped(t *testing.T) {
	mo := NewMultiObfuscator([]ReportingObfuscator{
		&splitObfuscator{tracker: NewSimpleTracker()},
	})
	seedable := mo.SeedableObfuscators()
	require.Len(t, seedable, 0)
}
