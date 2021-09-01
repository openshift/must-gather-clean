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

func NewMultiObfuscator(o []ReportingObfuscator) *MultiObfuscator {
	return &MultiObfuscator{obfuscators: o}
}
