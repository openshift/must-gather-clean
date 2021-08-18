package traversal

type Report struct {
	Replacements []map[string]string `yaml:"replacements,omitempty"`
	Omissions    []string            `yaml:"omissions,omitempty"`
}
