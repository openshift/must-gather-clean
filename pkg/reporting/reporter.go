package reporting

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

type Replacement struct {
	Canonical    string       `yaml:"canonical,omitempty"`
	ReplacedWith string       `yaml:"replacedWith,omitempty"`
	Occurrences  []Occurrence `yaml:"occurrences,omitempty"`
}

type Occurrence struct {
	Original string `yaml:"original,omitempty"`
	Count    uint   `yaml:"count,omitempty"`
}

type Report struct {
	Replacements [][]Replacement `yaml:"replacements,omitempty"`
	Omissions    []string        `yaml:"omissions,omitempty"`
}

type Reporter interface {
	// WriteReport writes the final report into the given path, will create folders if necessary.
	WriteReport(path string) error

	// CollectOmitterReport collects the omitter's omission results.
	CollectOmitterReport(omitter []string)

	// CollectObfuscatorReport will call the Report method on the obfuscator and collect the individual obfuscation results.
	CollectObfuscatorReport(obfuscatorReport []obfuscator.ReplacementReport)
}

type SimpleReporter struct {
	replacements [][]Replacement
	omissions    []string
}

var _ Reporter = (*SimpleReporter)(nil)

func (s *SimpleReporter) WriteReport(path string) error {

	reportingFolder := filepath.Dir(path)
	err := os.MkdirAll(reportingFolder, 0700)
	if err != nil {
		return fmt.Errorf("failed to create reporting output folder: %w", err)
	}

	reportFile, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to open report file %s: %w", path, err)
	}
	rEncoder := yaml.NewEncoder(reportFile)
	err = rEncoder.Encode(Report{
		Replacements: s.replacements,
		Omissions:    s.omissions,
	})
	if err != nil {
		return fmt.Errorf("failed to write report at %s: %w", path, err)
	}

	klog.V(2).Infof("successfully saved obfuscation report in %s", path)

	return nil
}

func (s *SimpleReporter) CollectOmitterReport(report []string) {
	s.omissions = append(s.omissions, report...)
}

func (s *SimpleReporter) CollectObfuscatorReport(obfuscatorReport []obfuscator.ReplacementReport) {
	for _, report := range obfuscatorReport {
		var replacements []Replacement
		for _, r := range report.Replacements {
			var occurrences []Occurrence
			for original, cnt := range r.Counter {
				occurrences = append(occurrences, Occurrence{
					Original: original,
					Count:    cnt,
				})
			}
			replacements = append(replacements, Replacement{
				Canonical:    r.Canonical,
				ReplacedWith: r.ReplacedWith,
				Occurrences:  occurrences,
			})
		}
		s.replacements = append(s.replacements, replacements)
	}
}

func NewSimpleReporter() Reporter {
	return &SimpleReporter{
		replacements: [][]Replacement{},
		omissions:    []string{},
	}
}
