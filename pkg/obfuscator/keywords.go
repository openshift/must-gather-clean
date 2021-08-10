package obfuscator

import "strings"

type keywordsObfuscator struct {
	ReplacementReporter
	replacements map[string]string
}

func (o *keywordsObfuscator) Report() map[string]string {
	return o.ReplacementReporter.Report()
}

func (o *keywordsObfuscator) FileName(name string) string {
	return replace(name, o.replacements, o.ReplacementReporter)
}

func replace(name string, replacements map[string]string, reporter ReplacementReporter) string {
	for keyword, replacement := range replacements {
		if strings.Contains(name, keyword) {
			name = strings.Replace(name, keyword, replacement, -1)
			reporter.UpsertReplacement(keyword, replacement)
		}
	}
	return name
}

func (o *keywordsObfuscator) Contents(contents string) string {
	return replace(contents, o.replacements, o.ReplacementReporter)
}

// NewKeywordsObfuscator returns an Obfuscator which replace all occurrences of keys in the map
// passed to it with the value of the key.
func NewKeywordsObfuscator(replacements map[string]string) Obfuscator {
	return &keywordsObfuscator{
		ReplacementReporter: NewSimpleReporter(),
		replacements:        replacements,
	}
}
