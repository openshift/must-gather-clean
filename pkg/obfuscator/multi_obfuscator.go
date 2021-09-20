package obfuscator

import (
	"github.com/openshift/must-gather-clean/pkg/schema"
)

type MultiObfuscator struct {
	obfuscators []ReportingObfuscator
}

func (m *MultiObfuscator) Path(s string) string {
	for _, obfuscator := range m.obfuscators {
		s = obfuscator.Path(s)
	}

	return s
}

func (m *MultiObfuscator) Contents(s string) string {
	for _, obfuscator := range m.obfuscators {
		s = obfuscator.Contents(s)
	}

	return s
}

func (m *MultiObfuscator) Type() string {
	return ""
}

func (m *MultiObfuscator) Report() map[string]string {
	multiReport := map[string]string{}
	for _, obfuscator := range m.obfuscators {
		report := obfuscator.Report()
		for k, v := range report {
			multiReport[k] = v
		}
	}

	return multiReport
}

func (m *MultiObfuscator) ReportPerObfuscator() []map[string]string {
	var multiReport []map[string]string
	for i := range m.obfuscators {
		multiReport = append(multiReport, m.obfuscators[i].Report())
	}

	return multiReport
}

func (m *MultiObfuscator) UpdateReportPerObfuscator(config *schema.SchemaJson) {
	for _, i := range m.obfuscators {
		for k := range config.Config.Obfuscate {
			if string(config.Config.Obfuscate[k].Type) == i.Type() {
				config.Config.Obfuscate[k].Report = make(map[string]string)
				for key, value := range i.Report() {
					config.Config.Obfuscate[k].Report[key] = value
				}
			}
		}
	}
}

func NewMultiObfuscator(o []ReportingObfuscator) *MultiObfuscator {
	return &MultiObfuscator{obfuscators: o}
}
