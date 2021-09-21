package obfuscator

// NoopObfuscator is only used for testing purposes across packages. It does obfuscate nothing.
// It also can't be defined in a test file, since it's won't be exported and needed by other packages.
type NoopObfuscator struct {
	Replacements map[string]string
}

func (d NoopObfuscator) GetReplacement(original string) string {
	return original
}

func (d NoopObfuscator) Path(input string) string {
	return input
}

func (d NoopObfuscator) Contents(input string) string {
	return input
}

func (d NoopObfuscator) Report() ReplacementReport {
	var r []Replacement
	for k, v := range d.Replacements {
		r = append(r, Replacement{
			Canonical:    k,
			ReplacedWith: v,
			// hard-coded 1 because NoopObfuscator doesn't track occurrences
			Counter: map[string]uint{
				k: 1,
			},
		})
	}
	return ReplacementReport{r}
}

func (d NoopObfuscator) ReportReplacement(a string, b string) {
	d.Replacements[a] = b
}
