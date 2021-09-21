package obfuscator

// Obfuscator is the interface which all obfuscators should implement
type Obfuscator interface {
	// Path takes a relative path (from the must-gather input root) as input and returns the obfuscated name
	Path(string) string
	// Contents takes string as input and return the obfuscated string
	Contents(string) string
}

type ReportingObfuscator interface {
	Obfuscator
	// Report returns a map of words and their Replacements
	Report() ReplacementReport
}
