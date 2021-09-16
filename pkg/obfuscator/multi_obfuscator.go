package obfuscator

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

func (m *MultiObfuscator) Report() ReplacementReport {
	var replacements []Replacement
	for _, obfuscator := range m.obfuscators {
		report := obfuscator.Report()
		replacements = append(replacements, report.Replacements...)
	}

	return ReplacementReport{Replacements: replacements}
}

func (m *MultiObfuscator) ReportPerObfuscator() []ReplacementReport {
	var multiReport []ReplacementReport
	for i := range m.obfuscators {
		multiReport = append(multiReport, m.obfuscators[i].Report())
	}

	return multiReport
}

func NewMultiObfuscator(o []ReportingObfuscator) *MultiObfuscator {
	return &MultiObfuscator{obfuscators: o}
}
