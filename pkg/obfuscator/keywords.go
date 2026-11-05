package obfuscator

import (
	"sort"
	"strings"
)

type keywordsObfuscator struct {
	ReplacementTracker
	matcher *keywordMatcher
}

type keywordEntry struct {
	text        string
	replacement string
	// generator is separate so the hot path can write replacement directly.
	generator GenerateReplacement
}

type keywordEdge struct {
	first byte
	child int32
}

type keywordNode struct {
	terminal int32
	edges    []keywordEdge
}

type keywordMatcher struct {
	entries []keywordEntry
	nodes   []keywordNode
	root    [256]int32
}

func (o *keywordsObfuscator) Path(name string) string {
	return o.matcher.replace(name, o.ReplacementTracker)
}

func (o *keywordsObfuscator) Contents(contents string) string {
	return o.matcher.replace(contents, o.ReplacementTracker)
}

func newKeywordMatcher(replacements map[string]string) *keywordMatcher {
	matcher := &keywordMatcher{
		entries: make([]keywordEntry, 0, len(replacements)),
		nodes:   []keywordNode{{terminal: -1}},
	}
	for first := range matcher.root {
		matcher.root[first] = -1
	}
	for keyword, replacement := range replacements {
		if keyword == "" {
			continue
		}
		entryIndex := len(matcher.entries)
		matcher.entries = append(matcher.entries, keywordEntry{
			text:        keyword,
			replacement: replacement,
			generator: func() string {
				return replacement
			},
		})
		matcher.add(entryIndex, keyword)
	}
	return matcher
}

func (m *keywordMatcher) add(entryIndex int, keyword string) {
	node := int32(0)
	for index := 0; index < len(keyword); index++ {
		first := keyword[index]
		child := m.next(node, first)
		if child < 0 {
			child = int32(len(m.nodes))
			m.nodes = append(m.nodes, keywordNode{terminal: -1})
			if node == 0 {
				m.root[first] = child
			} else {
				edges := m.nodes[node].edges
				position := sort.Search(len(edges), func(index int) bool {
					return edges[index].first >= first
				})
				edges = append(edges, keywordEdge{})
				copy(edges[position+1:], edges[position:])
				edges[position] = keywordEdge{first: first, child: child}
				m.nodes[node].edges = edges
			}
		}
		node = child
	}
	m.nodes[node].terminal = int32(entryIndex)
}

func (m *keywordMatcher) next(node int32, first byte) int32 {
	if node == 0 {
		return m.root[first]
	}
	edges := m.nodes[node].edges
	index := sort.Search(len(edges), func(index int) bool {
		return edges[index].first >= first
	})
	if index < len(edges) && edges[index].first == first {
		return edges[index].child
	}
	return -1
}

// matchAt returns the entry index and byte length of the longest keyword
// beginning at start, or -1 if no keyword begins there.
func (m *keywordMatcher) matchAt(input string, start int) (int, int) {
	node := m.root[input[start]]
	if node < 0 {
		return -1, 0
	}

	matched := m.nodes[node].terminal
	matchedLength := 1
	for offset := 1; start+offset < len(input); offset++ {
		node = m.next(node, input[start+offset])
		if node < 0 {
			break
		}
		if terminal := m.nodes[node].terminal; terminal >= 0 {
			matched = terminal
			matchedLength = offset + 1
		}
	}
	return int(matched), matchedLength
}

// replace performs one literal, byte-oriented, case-sensitive, non-overlapping
// pass over the input. At each position the longest keyword wins. Replacement
// text is not scanned again during this pass.
func (m *keywordMatcher) replace(input string, reporter ReplacementTracker) string {
	if len(m.entries) == 0 {
		return input
	}

	const inlineCounts = 8
	var countStorage [inlineCounts]uint
	var counts []uint
	var output strings.Builder

	for index := 0; index < len(input); {
		entryIndex, matchedLength := m.matchAt(input, index)
		if entryIndex < 0 {
			if counts != nil {
				output.WriteByte(input[index])
			}
			index++
			continue
		}

		if counts == nil {
			if len(m.entries) <= inlineCounts {
				counts = countStorage[:len(m.entries)]
			} else {
				counts = make([]uint, len(m.entries))
			}
			output.Grow(len(input))
			output.WriteString(input[:index])
		}

		entry := &m.entries[entryIndex]
		output.WriteString(entry.replacement)
		counts[entryIndex]++
		index += matchedLength
	}

	if counts == nil {
		return input
	}
	for index, count := range counts {
		if count == 0 {
			continue
		}
		entry := &m.entries[index]
		_ = reporter.GenerateIfAbsent(entry.text, entry.text, count, entry.generator)
	}
	return output.String()
}

// NewKeywordsObfuscator returns an Obfuscator which replaces all occurrences
// of keys in the map passed to it with the corresponding values.
func NewKeywordsObfuscator(replacements map[string]string) ReportingObfuscator {
	return &keywordsObfuscator{
		ReplacementTracker: NewSimpleTrackerMap(replacements),
		matcher:            newKeywordMatcher(replacements),
	}
}
