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

func (d NoopObfuscator) Report() map[string]string {
	return d.Replacements
}

func (d NoopObfuscator) ReportReplacement(a string, b string) {
	d.Replacements[a] = b
}
