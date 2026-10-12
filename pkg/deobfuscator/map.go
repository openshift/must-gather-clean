package deobfuscator

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/openshift/must-gather-clean/pkg/reporting"
	"github.com/openshift/must-gather-clean/pkg/schema"
	"gopkg.in/yaml.v3"
)

// Mapping contains the replacements from an obfuscation report.
type Mapping map[string]string

// LoadReport reads an obfuscation report and builds its reverse lookup.
func LoadReport(path string) (Mapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read report %s: %w", path, err)
	}

	var report reporting.Report
	if err := yaml.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("failed to parse report %s: %w", path, err)
	}
	if len(report.Config.Obfuscate) != len(report.Replacements) {
		return nil, fmt.Errorf("failed to parse report %s: obfuscation config and replacement groups do not match", path)
	}
	unsafeTokens, err := validateReplacementGroupConfiguration(report)
	if err != nil {
		return nil, fmt.Errorf("failed to parse report %s: %w", path, err)
	}
	return newMappingWithUnsafeTokens(report, unsafeTokens), nil
}

// validateReplacementGroupConfiguration checks report metadata that ties each
// replacement group to its configuration. Reversible groups need occurrence
// metadata so a same-count but misaligned report cannot make an unsupported
// replacement look like a supported one. Conflicting outputs for one original
// are returned as unsafe tokens because the config map can store only one.
func validateReplacementGroupConfiguration(report reporting.Report) (map[string]struct{}, error) {
	unsafeTokens := make(map[string]struct{})
	for groupIndex, group := range report.Replacements {
		config := report.Config.Obfuscate[groupIndex]
		outputsByOriginal := make(map[string]map[string]struct{})
		for _, replacement := range group {
			if replacement.ReplacedWith == "" {
				continue
			}
			for _, occurrence := range replacement.Occurrences {
				if outputsByOriginal[occurrence.Original] == nil {
					outputsByOriginal[occurrence.Original] = make(map[string]struct{})
				}
				outputsByOriginal[occurrence.Original][replacement.ReplacedWith] = struct{}{}
			}
		}

		for _, replacement := range group {
			if replacement.ReplacedWith == "" {
				continue
			}
			if isReversibleConfiguration(config) && replacement.Canonical != "" && len(replacement.Occurrences) == 0 {
				return nil, fmt.Errorf("replacement group %d is missing occurrence metadata", groupIndex+1)
			}
			if len(replacement.Occurrences) > 0 {
				for _, occurrence := range replacement.Occurrences {
					replacedWith, exists := config.Replacement[occurrence.Original]
					if !exists {
						return nil, fmt.Errorf("obfuscation config and replacement group %d do not match", groupIndex+1)
					}
					if replacedWith != replacement.ReplacedWith {
						if _, isReportedOutput := outputsByOriginal[occurrence.Original][replacedWith]; isReportedOutput {
							// The config map can retain only one output for an original.
							// Keep the group, but do not restore this conflicting token.
							unsafeTokens[replacement.ReplacedWith] = struct{}{}
							continue
						}
						return nil, fmt.Errorf("obfuscation config and replacement group %d do not match", groupIndex+1)
					}
				}
				continue
			}
			if len(config.Replacement) == 0 {
				continue
			}
			found := false
			for _, configuredReplacement := range config.Replacement {
				if configuredReplacement == replacement.ReplacedWith {
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("obfuscation config and replacement group %d do not match", groupIndex+1)
			}
		}
	}
	return unsafeTokens, nil
}

// newMapping builds a reverse lookup from the replacement groups in a report.
// Only consistent IP, MAC, and Domain replacements are reversible. Tokens with
// more than one canonical value or involved in a replacement chain are omitted
// because they cannot be restored safely.
func newMapping(report reporting.Report) Mapping {
	return newMappingWithUnsafeTokens(report, nil)
}

func newMappingWithUnsafeTokens(report reporting.Report, unsafeTokens map[string]struct{}) Mapping {
	mapping := make(Mapping)
	knownUnsafeTokens := make(map[string]struct{}, len(unsafeTokens))
	for token := range unsafeTokens {
		knownUnsafeTokens[token] = struct{}{}
	}
	unsafeTokens = knownUnsafeTokens
	for token := range findChainedReplacementTokens(report) {
		unsafeTokens[token] = struct{}{}
	}
	for token := range findTokensInUnsupportedOutputs(report) {
		unsafeTokens[token] = struct{}{}
	}
	canonicalValues := make(map[string]struct{})
	replacementTokens := make(map[string]struct{})

	for groupIndex, group := range report.Replacements {
		reversible := isReversibleConfiguration(report.Config.Obfuscate[groupIndex])
		for _, replacement := range group {
			if replacement.Canonical == "" || replacement.ReplacedWith == "" {
				continue
			}
			token := replacement.ReplacedWith
			canonicalValues[replacement.Canonical] = struct{}{}
			replacementTokens[token] = struct{}{}

			if !reversible {
				delete(mapping, token)
				unsafeTokens[token] = struct{}{}
				continue
			}
			if _, unsafe := unsafeTokens[token]; unsafe {
				delete(mapping, token)
				continue
			}
			if previous, exists := mapping[token]; exists && previous != replacement.Canonical {
				delete(mapping, token)
				unsafeTokens[token] = struct{}{}
				continue
			}
			mapping[token] = replacement.Canonical
		}
	}

	for token, canonical := range mapping {
		_, tokenIsInput := canonicalValues[token]
		_, canonicalIsOutput := replacementTokens[canonical]
		if tokenIsInput || canonicalIsOutput || canonical == token {
			delete(mapping, token)
			unsafeTokens[token] = struct{}{}
		}
	}
	// If an unsafe token contains a shorter supported token, restoring the
	// shorter token would still partially change the ambiguous or chained value.
	for unsafeToken := range unsafeTokens {
		for token := range mapping {
			if strings.Contains(unsafeToken, token) {
				delete(mapping, token)
			}
		}
	}

	return mapping
}

// findTokensInUnsupportedOutputs prevents a supported token from being restored
// when an unsupported obfuscator can emit it as part of a literal replacement.
// The response is processed as plain text, so it cannot distinguish that value
// from a token produced by the supported obfuscator.
func findTokensInUnsupportedOutputs(report reporting.Report) map[string]struct{} {
	supportedTokens := make(map[string]struct{})
	var unsupportedOutputs []string

	for groupIndex, group := range report.Replacements {
		config := report.Config.Obfuscate[groupIndex]
		reversible := isReversibleConfiguration(config)
		for _, replacement := range group {
			if replacement.ReplacedWith == "" {
				continue
			}
			if reversible {
				supportedTokens[replacement.ReplacedWith] = struct{}{}
			} else {
				unsupportedOutputs = append(unsupportedOutputs, replacement.ReplacedWith)
			}
		}

		if !reversible {
			for _, replacement := range config.Replacement {
				unsupportedOutputs = append(unsupportedOutputs, replacement)
			}
		}
		if config.Type == schema.ObfuscateTypeExact {
			for _, replacement := range config.ExactReplacements {
				unsupportedOutputs = append(unsupportedOutputs, replacement.Replacement)
			}
		}
	}

	unsafeTokens := make(map[string]struct{})
	for token := range supportedTokens {
		for _, output := range unsupportedOutputs {
			if strings.Contains(output, token) {
				unsafeTokens[token] = struct{}{}
				break
			}
		}
	}

	return unsafeTokens
}

func findChainedReplacementTokens(report reporting.Report) map[string]struct{} {
	chained := make(map[string]struct{})
	pathTokens := make(Mapping)
	contentTokens := make(Mapping)

	// Include unsupported groups because their outputs can feed later obfuscators.
	for groupIndex, group := range report.Replacements {
		config := report.Config.Obfuscate[groupIndex]
		target := config.Target
		if target == "" {
			target = schema.ObfuscateTargetFileContents
		}
		pathTarget := target == schema.ObfuscateTargetAll || target == schema.ObfuscateTargetFilePath
		contentTarget := target == schema.ObfuscateTargetAll || target == schema.ObfuscateTargetFileContents

		var pathReplacer, contentReplacer *strings.Replacer
		if len(pathTokens) > 0 && pathTarget {
			pathReplacer = pathTokens.newReplacer()
		}
		if len(contentTokens) > 0 && contentTarget {
			contentReplacer = contentTokens.newReplacer()
		}
		if config.Type == schema.ObfuscateTypeExact {
			for _, exact := range config.ExactReplacements {
				if pathReplacer != nil && pathReplacer.Replace(exact.Original) != exact.Original {
					markContainedTokens(chained, pathTokens, exact.Original)
				}
				if contentReplacer != nil && contentReplacer.Replace(exact.Original) != exact.Original {
					markContainedTokens(chained, contentTokens, exact.Original)
				}
			}
		}
		for _, replacement := range group {
			if replacement.ReplacedWith == "" {
				continue
			}
			inputs := []string{replacement.Canonical}
			// Canonicalization can change case or formatting. Occurrences retain
			// the spelling that actually consumed an earlier output.
			for _, occurrence := range replacement.Occurrences {
				inputs = append(inputs, occurrence.Original)
			}
			for _, input := range inputs {
				pathChained := pathReplacer != nil && pathReplacer.Replace(input) != input
				contentChained := contentReplacer != nil && contentReplacer.Replace(input) != input
				if pathChained || contentChained {
					chained[replacement.ReplacedWith] = struct{}{}
					if pathChained {
						markContainedTokens(chained, pathTokens, input)
					}
					if contentChained {
						markContainedTokens(chained, contentTokens, input)
					}
				}
			}
		}

		for _, replacement := range group {
			if replacement.ReplacedWith == "" {
				continue
			}
			if pathTarget {
				pathTokens[replacement.ReplacedWith] = ""
			}
			if contentTarget {
				contentTokens[replacement.ReplacedWith] = ""
			}
		}
		// Exact replacements do not populate the report group. Their configured
		// outputs can still feed a later obfuscator in the same target.
		if config.Type == schema.ObfuscateTypeExact {
			for _, replacement := range config.ExactReplacements {
				if replacement.Replacement == "" {
					continue
				}
				if pathTarget {
					pathTokens[replacement.Replacement] = ""
				}
				if contentTarget {
					contentTokens[replacement.Replacement] = ""
				}
			}
		}
	}

	return chained
}

// markContainedTokens prevents a token from being restored when it occurs
// anywhere inside the canonical value of a later replacement in the same
// target. All such tokens must stay intact, including overlapping token forms
// that could otherwise be partially restored after a longer token is omitted.
func markContainedTokens(chained map[string]struct{}, tokens Mapping, input string) {
	for token := range tokens {
		if strings.Contains(input, token) {
			chained[token] = struct{}{}
		}
	}
}

func isReversibleConfiguration(config schema.Obfuscate) bool {
	if config.ReplacementType != schema.ObfuscateReplacementTypeConsistent {
		return false
	}
	if config.Target != "" && config.Target != schema.ObfuscateTargetAll &&
		config.Target != schema.ObfuscateTargetFileContents && config.Target != schema.ObfuscateTargetFilePath {
		return false
	}

	switch config.Type {
	case schema.ObfuscateTypeIP,
		schema.ObfuscateTypeMAC,
		schema.ObfuscateTypeDomain:
		return true
	default:
		return false
	}
}

// Replace restores tokens in input. Unknown tokens are left unchanged.
func (m Mapping) Replace(input string) string {
	if len(m) == 0 {
		return input
	}

	return m.newReplacer().Replace(input)
}

// ReplaceReader restores tokens line by line.
func (m Mapping) ReplaceReader(input io.Reader, output io.Writer) error {
	if len(m) == 0 {
		_, err := io.Copy(output, input)
		return err
	}

	replacer := m.newReplacer()
	reader := bufio.NewReader(input)
	writer := bufio.NewWriter(output)
	for {
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			if _, err := writer.WriteString(replacer.Replace(line)); err != nil {
				return err
			}
		}
		if readErr != nil {
			if err := writer.Flush(); err != nil {
				return err
			}
			if readErr == io.EOF {
				return nil
			}
			return readErr
		}
	}
}

func (m Mapping) newReplacer() *strings.Replacer {
	keys := make([]string, 0, len(m))
	for token := range m {
		keys = append(keys, token)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})

	args := make([]string, 0, len(keys)*2)
	for _, token := range keys {
		args = append(args, token, m[token])
	}
	return strings.NewReplacer(args...)
}
