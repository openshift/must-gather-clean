package obfuscator

// Obfuscator is the interface which all obfuscators should implement
type Obfuscator interface {
	// FileName takes a filename as input and return the obfuscated name
	FileName(string) string
	// Contents takes string as input and return the obfuscated string
	Contents(string) string
	// ReportingResult returns a map of words and their replacements
	ReportingResult() map[string]string
}
