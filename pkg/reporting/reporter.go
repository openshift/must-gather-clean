package reporting

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift/must-gather-clean/pkg/omitter"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

type Report struct {
	Replacements []map[string]string `yaml:"replacements,omitempty"`
	Omissions    []string            `yaml:"omissions,omitempty"`
}

type Reporter interface {
	// WriteReport writes the final report into the given path, will create folders if necessary.
	WriteReport(path string) error

	// CollectOmitterReport will call the Report method on the omitter and collect its omissions.
	CollectOmitterReport(omitter omitter.ReportingOmitter)

	// CollectObfuscatorReport will call the Report method on the obfuscator and collect the individual obfuscation results.
	CollectObfuscatorReport(obfuscatorReport []map[string]string)
}

type SimpleReporter struct {
	replacements []map[string]string
	omissions    []string
}

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

func (s *SimpleReporter) CollectObfuscatorReport(obfuscatorReport []map[string]string) {
	s.replacements = append(s.replacements, obfuscatorReport...)
}

func NewSimpleReporter() *SimpleReporter {
	return &SimpleReporter{
		replacements: []map[string]string{},
		omissions:    []string{},
	}
}
