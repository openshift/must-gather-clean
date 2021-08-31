package traversal

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/openshift/must-gather-clean/pkg/obfuscator"
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

	// ReportOmission saves the given path as omitted to later report as such.
	ReportOmission(path string)

	// ReportObfuscators will call the Report method on all obfuscators and collect the individual obfuscation results for final reporting.
	ReportObfuscators(obfuscators []obfuscator.Obfuscator)
}

type SimpleReporter struct {
	lock         *sync.RWMutex
	replacements []map[string]string
	omissions    []string
}

func (s *SimpleReporter) WriteReport(path string) error {
	s.lock.RLock()
	defer s.lock.RUnlock()

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

func (s *SimpleReporter) ReportOmission(path string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.omissions = append(s.omissions, path)
}

func (s *SimpleReporter) ReportObfuscators(obfuscators []obfuscator.Obfuscator) {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, o := range obfuscators {
		s.replacements = append(s.replacements, o.Report())
	}
}

func NewSimpleReporter() *SimpleReporter {
	return &SimpleReporter{
		lock:         &sync.RWMutex{},
		replacements: []map[string]string{},
		omissions:    []string{},
	}
}
