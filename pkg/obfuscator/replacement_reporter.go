package obfuscator

// this struct mainly exists in case we later want to make it thread-safe, so we don't have to individually go through
// dozens of obfuscators.

type ReplacementReporter interface {
	// Report returns a mapping of strings which were replaced by this obfuscator
	Report() map[string]string

	// UpsertReplacement will add a new replacement along with its original string to the report.
	// Overwrites existing values for the same key, thus "upsert" for update+insert.
	UpsertReplacement(original string, replacement string)
}

type SimpleReporter struct {
	mapping map[string]string
}

func (s *SimpleReporter) Report() map[string]string {
	return s.mapping
}

func (s *SimpleReporter) UpsertReplacement(original string, replacement string) {
	s.mapping[original] = replacement
}

func NewSimpleReporter() ReplacementReporter {
	return &SimpleReporter{map[string]string{}}
}
