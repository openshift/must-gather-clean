package obfuscator

type ReplacementReport struct {
	Replacements []Replacement
}

type Replacement struct {
	Canonical    string       `yaml:"canonical,omitempty"`
	ReplacedWith string       `yaml:"replacedWith,omitempty"`
	Occurrences  []Occurrence `yaml:"occurrences,omitempty"`
}

type Occurrence struct {
	Original string `yaml:"original,omitempty"`
	Count    uint   `yaml:"count,omitempty"`
}

func (r *Replacement) Increment(original string, count uint) Replacement {
	if r.Occurrences == nil {
		r.Occurrences = []Occurrence{}
	}
	for i, o := range r.Occurrences {
		if original == o.Original {
			r.Occurrences[i].Count += count
			return *r
		}
	}
	o := Occurrence{Original: original, Count: count}
	r.Occurrences = append(r.Occurrences, o)
	return *r
}
