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

// SeedableObfuscators returns all obfuscators that implement SeedableObfuscator.
func (m *MultiObfuscator) SeedableObfuscators() []SeedableObfuscator {
	var result []SeedableObfuscator
	for _, o := range m.obfuscators {
		if s, ok := unwrapSeedable(o); ok {
			result = append(result, s)
		}
	}
	return result
}

// unwrapSeedable checks if o implements SeedableObfuscator, unwrapping
// any number of targetObfuscator layers to find it.
func unwrapSeedable(o ReportingObfuscator) (SeedableObfuscator, bool) {
	for {
		if s, ok := o.(SeedableObfuscator); ok {
			return s, true
		}
		t, ok := o.(*targetObfuscator)
		if !ok {
			return nil, false
		}
		o = t.obfuscator
	}
}
