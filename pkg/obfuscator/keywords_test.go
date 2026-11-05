package obfuscator

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewKeywordsObfuscator(t *testing.T) {
	for _, tc := range []struct {
		name           string
		replacements   map[string]string
		input          string
		expectedOutput string
		expectLegend   ReplacementReport
	}{
		{
			name: "basic",
			replacements: map[string]string{
				"unique-word": "replacement",
			},
			input:          "input with unique-word",
			expectedOutput: "input with replacement",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "unique-word", ReplacedWith: "replacement",
					Counter: map[string]uint{
						"unique-word": 1,
					}},
			}},
		},
		{
			name: "no replacement",
			replacements: map[string]string{
				"unique-word": "replacement",
			},
			input:          "input with common words",
			expectedOutput: "input with common words",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "unique-word", ReplacedWith: "replacement",
					Counter: map[string]uint{
						"unique-word": 0,
					}},
			}},
		},
		{
			name: "partial replacement",
			replacements: map[string]string{
				"first-unique":  "first-replacement",
				"second-unique": "second-replacement",
			},
			input:          "input with first-unique word",
			expectedOutput: "input with first-replacement word",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "first-unique", ReplacedWith: "first-replacement",
					Counter: map[string]uint{
						"first-unique": 1,
					}},
				{Canonical: "second-unique", ReplacedWith: "second-replacement",
					Counter: map[string]uint{
						"second-unique": 0,
					}},
			}},
		},
		{
			name: "partial replacement with repetition",
			replacements: map[string]string{
				"foo": "four",
				"bar": "zero",
			},
			input:          "input with foo foo foo times foo",
			expectedOutput: "input with four four four times four",
			expectLegend: ReplacementReport{[]Replacement{
				{Canonical: "foo", ReplacedWith: "four",
					Counter: map[string]uint{
						"foo": 4,
					}},
				{Canonical: "bar", ReplacedWith: "zero",
					Counter: map[string]uint{
						"bar": 0,
					}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := NewKeywordsObfuscator(tc.replacements)
			require.Equal(t, tc.expectedOutput, o.Contents(tc.input))
			replacementReportsMatch(t, tc.expectLegend, o.Report())
		})
	}
}

func TestKeywordsObfuscatorDoesNotRewriteGeneratedReplacements(t *testing.T) {
	for iteration := 0; iteration < 200; iteration++ {
		obfuscator := NewKeywordsObfuscator(map[string]string{
			"foo": "bar",
			"bar": "baz",
		})

		require.Equal(t, "bar", obfuscator.Contents("foo"))
		replacementReportsMatch(t, ReplacementReport{Replacements: []Replacement{
			{Canonical: "bar", ReplacedWith: "baz", Counter: map[string]uint{"bar": 0}},
			{Canonical: "foo", ReplacedWith: "bar", Counter: map[string]uint{"foo": 1}},
		}}, obfuscator.Report())
	}
}

func TestKeywordsObfuscatorPrefersLongestKeywordWithoutOverlappingCounts(t *testing.T) {
	for iteration := 0; iteration < 200; iteration++ {
		obfuscator := NewKeywordsObfuscator(map[string]string{
			"foo":    "short",
			"foobar": "long",
		})

		require.Equal(t, "longshort", obfuscator.Contents("foobarfoo"))
		replacementReportsMatch(t, ReplacementReport{Replacements: []Replacement{
			{Canonical: "foo", ReplacedWith: "short", Counter: map[string]uint{"foo": 1}},
			{Canonical: "foobar", ReplacedWith: "long", Counter: map[string]uint{"foobar": 1}},
		}}, obfuscator.Report())
	}
}

func TestKeywordsObfuscatorMatchingEdgeCases(t *testing.T) {
	for _, tc := range []struct {
		name         string
		replacements map[string]string
		input        string
		want         string
		counts       map[string]uint
	}{
		{
			name:         "adjacent non-overlapping matches",
			replacements: map[string]string{"aa": "X"},
			input:        "aaaaa",
			want:         "XXa",
			counts:       map[string]uint{"aa": 2},
		},
		{
			name:         "earlier match consumes a later candidate",
			replacements: map[string]string{"ab": "A", "bc": "B"},
			input:        "abc",
			want:         "Ac",
			counts:       map[string]uint{"ab": 1, "bc": 0},
		},
		{
			name:         "longer keyword does not match a shorter input",
			replacements: map[string]string{"ab": "A", "abc": "ABC"},
			input:        "ab",
			want:         "A",
			counts:       map[string]uint{"ab": 1, "abc": 0},
		},
		{
			name:         "arbitrary bytes",
			replacements: map[string]string{string([]byte{0xff, 0x00}): "masked"},
			input:        "a" + string([]byte{0xff, 0x00}) + "b",
			want:         "amaskedb",
			counts:       map[string]uint{string([]byte{0xff, 0x00}): 1},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obfuscator := NewKeywordsObfuscator(tc.replacements)
			require.Equal(t, tc.want, obfuscator.Contents(tc.input))

			report := obfuscator.Report()
			require.Len(t, report.Replacements, len(tc.replacements))
			for keyword, count := range tc.counts {
				found := false
				for _, replacement := range report.Replacements {
					if replacement.Canonical == keyword {
						found = true
						require.Equal(t, count, replacement.Counter[keyword])
					}
				}
				require.Truef(t, found, "report is missing keyword %q", keyword)
			}
		})
	}
}

func TestKeywordsObfuscatorIsCaseSensitiveAndIgnoresEmptyKeyword(t *testing.T) {
	obfuscator := NewKeywordsObfuscator(map[string]string{
		"":    "ignored",
		"Foo": "upper",
		"foo": "lower",
	})

	require.Equal(t, "lower upper FOO lower", obfuscator.Contents("foo Foo FOO foo"))
	replacementReportsMatch(t, ReplacementReport{Replacements: []Replacement{
		{Canonical: "", ReplacedWith: "ignored", Counter: map[string]uint{"": 0}},
		{Canonical: "Foo", ReplacedWith: "upper", Counter: map[string]uint{"Foo": 1}},
		{Canonical: "foo", ReplacedWith: "lower", Counter: map[string]uint{"foo": 2}},
	}}, obfuscator.Report())
}

func TestKeywordsObfuscatorPathAndContentsAccumulateCounts(t *testing.T) {
	obfuscator := NewKeywordsObfuscator(map[string]string{"foo": "replacement"})

	require.Equal(t, "namespaces/replacement/Foo", obfuscator.Path("namespaces/foo/Foo"))
	require.Equal(t, "replacement replacement", obfuscator.Contents("foo foo"))
	require.Equal(t, "replacement", obfuscator.Contents("foo"))
	replacementReportsMatch(t, ReplacementReport{Replacements: []Replacement{
		{Canonical: "foo", ReplacedWith: "replacement", Counter: map[string]uint{"foo": 4}},
	}}, obfuscator.Report())
}

func TestKeywordsObfuscatorSnapshotsReplacements(t *testing.T) {
	replacements := map[string]string{"foo": "bar"}
	obfuscator := NewKeywordsObfuscator(replacements)
	replacements["foo"] = "changed"
	replacements["new"] = "value"

	require.Equal(t, "bar new", obfuscator.Contents("foo new"))
	replacementReportsMatch(t, ReplacementReport{Replacements: []Replacement{
		{Canonical: "foo", ReplacedWith: "bar", Counter: map[string]uint{"foo": 1}},
	}}, obfuscator.Report())
}

func TestKeywordsObfuscatorConcurrentCalls(t *testing.T) {
	const (
		workers = 16
		calls   = 100
	)
	obfuscator := NewKeywordsObfuscator(map[string]string{
		"foo": "replacement",
		"bar": "other-replacement",
	})

	mismatch := make(chan string, 1)
	var waitGroup sync.WaitGroup
	waitGroup.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func() {
			defer waitGroup.Done()
			for call := 0; call < calls; call++ {
				if got := obfuscator.Contents("foo bar foo"); got != "replacement other-replacement replacement" {
					select {
					case mismatch <- got:
					default:
					}
					return
				}
			}
		}()
	}
	waitGroup.Wait()
	select {
	case got := <-mismatch:
		t.Fatalf("unexpected concurrent output %q", got)
	default:
	}

	replacementReportsMatch(t, ReplacementReport{Replacements: []Replacement{
		{Canonical: "bar", ReplacedWith: "other-replacement", Counter: map[string]uint{"bar": workers * calls}},
		{Canonical: "foo", ReplacedWith: "replacement", Counter: map[string]uint{"foo": 2 * workers * calls}},
	}}, obfuscator.Report())
}

func TestKeywordsObfuscatorCountsWithMoreThanInlineKeywords(t *testing.T) {
	replacements := make(map[string]string, 9)
	for index := 0; index < 9; index++ {
		replacements[fmt.Sprintf("keyword-%d", index)] = fmt.Sprintf("replacement-%d", index)
	}
	obfuscator := NewKeywordsObfuscator(replacements)

	require.Equal(t, "replacement-8", obfuscator.Contents("keyword-8"))

	report := obfuscator.Report()
	require.Len(t, report.Replacements, len(replacements))
	for _, replacement := range report.Replacements {
		want := uint(0)
		if replacement.Canonical == "keyword-8" {
			want = 1
		}
		require.Equal(t, want, replacement.Counter[replacement.Canonical])
	}
}

func TestKeywordsObfuscatorNoMatchDoesNotAllocate(t *testing.T) {
	obfuscator := NewKeywordsObfuscator(map[string]string{
		"cluster-name": "obfuscated-cluster",
		"namespace":    "obfuscated-namespace",
		"token":        "obfuscated-token",
	})
	input := strings.Repeat("ordinary text without configured keywords\n", 1000)
	var output string

	allocations := testing.AllocsPerRun(100, func() {
		output = obfuscator.Contents(input)
	})
	require.Zero(t, allocations)
	require.Equal(t, input, output)
	require.Len(t, obfuscator.Report().Replacements, 3)
}

func BenchmarkKeywordsObfuscator(b *testing.B) {
	replacements := map[string]string{
		"cluster-name": "obfuscated-cluster",
		"namespace":    "obfuscated-namespace",
		"token":        "obfuscated-token",
	}
	input := strings.Repeat("namespace cluster-name token ordinary text\n", 10000)
	obfuscator := NewKeywordsObfuscator(replacements)

	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		_ = obfuscator.Contents(input)
	}
}

func BenchmarkKeywordsObfuscatorNoMatch(b *testing.B) {
	replacements := map[string]string{
		"cluster-name": "obfuscated-cluster",
		"namespace":    "obfuscated-namespace",
		"token":        "obfuscated-token",
	}
	input := strings.Repeat("ordinary text without configured keywords\n", 10000)
	obfuscator := NewKeywordsObfuscator(replacements)

	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		_ = obfuscator.Contents(input)
	}
}

func BenchmarkKeywordsObfuscatorSharedPrefixMiss(b *testing.B) {
	replacements := make(map[string]string, 4096)
	for index := 0; index < 4096; index++ {
		keyword := fmt.Sprintf("shared-prefix-%08d", index)
		replacements[keyword] = "replacement"
	}
	input := strings.Repeat("shared-prefix-not-configured ordinary text\n", 64)
	obfuscator := NewKeywordsObfuscator(replacements)

	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		_ = obfuscator.Contents(input)
	}
}
