package obfuscator

import (
	cryptorand "crypto/rand"
	_ "embed"
	"math/big"
	"strings"

	"github.com/openshift/must-gather-clean/pkg/schema"
)

// petNameReplacementGenerator generates replacements using petnames (adjective-noun combinations)
type petNameReplacementGenerator struct {
	prefix          string
	static          string
	petNameGen      *PetNameGenerator
	replacementType schema.ObfuscateReplacementType
}

func newPetNameReplacementGenerator(prefix, static string, petNameGen *PetNameGenerator, replacementType schema.ObfuscateReplacementType) *petNameReplacementGenerator {
	return &petNameReplacementGenerator{
		prefix:          prefix,
		static:          static,
		petNameGen:      petNameGen,
		replacementType: replacementType,
	}
}

func (g *petNameReplacementGenerator) generateConsistentReplacement() string {
	return g.prefix + "-" + g.petNameGen.Generate(2)
}

func (g *petNameReplacementGenerator) generateStaticReplacement() string {
	return g.static
}

func (g *petNameReplacementGenerator) generateReplacement(key string, original string, count uint, tracker ReplacementTracker) string {
	var replacement string
	switch g.replacementType {
	case schema.ObfuscateReplacementTypeStatic:
		replacement = tracker.GenerateIfAbsent(key, original, count, g.generateStaticReplacement)
	case schema.ObfuscateReplacementTypeConsistent:
		replacement = tracker.GenerateIfAbsent(key, original, count, g.generateConsistentReplacement)
	}
	return replacement
}

var (
	//go:embed artifacts/adjectives.txt
	adjectives string
	//go:embed artifacts/adverbs.txt
	adverbs string
	//go:embed artifacts/names.txt
	names string
)

type RandomSource interface {
	Intn(n int) int
}

type cryptoRandSource struct{}

func (cryptoRandSource) Intn(n int) int {
	ret, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(err)
	}
	return int(ret.Int64())
}

type PetNameGenerator struct {
	separator string

	adverbs    []string
	adjectives []string
	names      []string

	rand RandomSource
}

func NewPetNameGenerator(separator string, rand RandomSource) *PetNameGenerator {
	return &PetNameGenerator{
		separator:  separator,
		adjectives: strings.Split(strings.TrimSpace(adjectives), "\n"),
		adverbs:    strings.Split(strings.TrimSpace(adverbs), "\n"),
		names:      strings.Split(strings.TrimSpace(names), "\n"),

		rand: rand,
	}
}

// Adverb returns a random adverb from a list of petname adverbs.
func (p *PetNameGenerator) adverb() string {
	return p.adverbs[p.rand.Intn(len(p.adverbs))]
}

// Adjective returns a random adjective from a list of petname adjectives.
func (p *PetNameGenerator) adjective() string {
	return p.adjectives[p.rand.Intn(len(p.adjectives))]
}

// Name returns a random name from a list of petname names.
func (p *PetNameGenerator) name() string {
	return p.names[p.rand.Intn(len(p.names))]
}

// Generate generates and returns a random pet name.
// It takes two parameters:  the number of words in the name, and a separator token.
// If a single word is requested, simply a Name() is returned.
// If two words are requested, a Adjective() and a Name() are returned.
// If three or more words are requested, a variable number of Adverb() and a Adjective and a Name() is returned.
// The separator can be any character, string, or the empty string.
func (p *PetNameGenerator) Generate(words int) string {
	if words == 1 {
		return p.name()
	} else if words == 2 {
		return p.adjective() + p.separator + p.name()
	}
	var petnameParts []string
	for i := 0; i < words-2; i++ {
		petnameParts = append(petnameParts, p.adverb())
	}
	petnameParts = append(petnameParts, p.adjective(), p.name())
	return strings.Join(petnameParts, p.separator)
}
