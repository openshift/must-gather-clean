package reporting

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/openshift/must-gather-clean/pkg/fsutil"
	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/schema"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

type Replacement struct {
	Canonical    string       `yaml:"canonical,omitempty"`
	ReplacedWith string       `yaml:"replacedWith,omitempty"`
	Occurrences  []Occurrence `yaml:"occurrences,omitempty"`
}

type Occurrence struct {
	Original string `yaml:"original,omitempty"`
	Count    uint   `yaml:"count,omitempty"`
}

type Report struct {
	Replacements [][]Replacement         `yaml:"replacements,omitempty"`
	Omissions    []string                `yaml:"omissions,omitempty"`
	Config       schema.SchemaJsonConfig `yaml:"config,omitempty"`
}

type Reporter interface {
	// WriteReport writes the final report into the given path, will create folders if necessary.
	WriteReport(path string) error

	// CollectOmitterReport collects the omitter's omission results.
	CollectOmitterReport(omitter []string)

	// CollectObfuscatorReport will call the Report method on the obfuscator and collect the individual obfuscation results.
	CollectObfuscatorReport(obfuscatorReport []obfuscator.ReplacementReport)
}

type SimpleReporter struct {
	replacements  [][]Replacement
	omissions     []string
	config        *schema.SchemaJson
	collectionErr error
}

var _ Reporter = (*SimpleReporter)(nil)

func (s *SimpleReporter) WriteReport(path string) error {
	if s.collectionErr != nil {
		return s.collectionErr
	}
	if s.config == nil {
		return fmt.Errorf("cannot write report without a configuration")
	}
	if len(s.replacements) != len(s.config.Config.Obfuscate) {
		return fmt.Errorf(
			"cannot write report: %d replacement groups for %d configured obfuscators",
			len(s.replacements), len(s.config.Config.Obfuscate),
		)
	}

	targetPath, err := fsutil.ResolveSymlink(path)
	if err != nil {
		return fmt.Errorf("failed to resolve report path %s: %w", path, err)
	}

	reportingFolder := filepath.Dir(targetPath)
	err = os.MkdirAll(reportingFolder, 0700)
	if err != nil {
		return fmt.Errorf("failed to create reporting output folder: %w", err)
	}

	// Stage under a private directory, then publish atomically beside the final
	// target. The report itself follows ordinary open semantics: base mode 0666
	// filtered by umask for a new file, or the existing file's permission bits.
	temporaryDir, err := os.MkdirTemp(reportingFolder, "."+filepath.Base(targetPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary report directory for %s: %w", path, err)
	}
	defer func() {
		_ = os.RemoveAll(temporaryDir)
	}()

	existingMode, existingReport, err := reportFileMode(targetPath)
	if err != nil {
		return fmt.Errorf("failed to inspect report path %s: %w", path, err)
	}

	temporaryPath := filepath.Join(temporaryDir, filepath.Base(targetPath))
	reportFile, err := os.OpenFile(temporaryPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if err != nil {
		return fmt.Errorf("failed to open report file %s: %w", path, err)
	}
	if existingReport {
		if err := reportFile.Chmod(existingMode); err != nil {
			_ = reportFile.Close()
			return fmt.Errorf("failed to preserve report mode for %s: %w", path, err)
		}
	}
	committed := false
	defer func() {
		if !committed {
			_ = reportFile.Close()
		}
	}()

	rEncoder := yaml.NewEncoder(reportFile)
	err = rEncoder.Encode(Report{
		Replacements: s.replacements,
		Omissions:    s.omissions,
		Config:       s.config.Config,
	})
	if err != nil {
		_ = rEncoder.Close()
		_ = reportFile.Close()
		return fmt.Errorf("failed to write report at %s: %w", path, err)
	}
	if err := rEncoder.Close(); err != nil {
		_ = reportFile.Close()
		return fmt.Errorf("failed to finish report at %s: %w", path, err)
	}
	if err := reportFile.Close(); err != nil {
		return fmt.Errorf("failed to close report at %s: %w", path, err)
	}
	if err := os.Rename(temporaryPath, targetPath); err != nil {
		return fmt.Errorf("failed to replace report at %s: %w", path, err)
	}
	committed = true

	klog.V(3).Infof("successfully saved obfuscation report in %s", path)

	return nil
}

func reportFileMode(path string) (os.FileMode, bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return info.Mode().Perm(), true, nil
}

func (s *SimpleReporter) CollectOmitterReport(report []string) {
	s.omissions = append(s.omissions, report...)
	sort.Strings(s.omissions)
}

func (s *SimpleReporter) CollectObfuscatorReport(obfuscatorReport []obfuscator.ReplacementReport) {
	for _, report := range obfuscatorReport {
		var replacements []Replacement
		for _, r := range report.Replacements {
			var occurrences []Occurrence
			for original, cnt := range r.Counter {
				occurrences = append(occurrences, Occurrence{
					Original: original,
					Count:    cnt,
				})
			}
			sortOccurrences(occurrences)
			replacements = append(replacements, Replacement{
				Canonical:    r.Canonical,
				ReplacedWith: r.ReplacedWith,
				Occurrences:  occurrences,
			})
		}
		sortReplacements(replacements)
		s.replacements = append(s.replacements, replacements)
	}

	if s.config == nil {
		s.setCollectionError(fmt.Errorf("cannot collect obfuscator replacements without a configuration"))
		return
	}
	if len(s.replacements) != len(s.config.Config.Obfuscate) {
		s.setCollectionError(fmt.Errorf(
			"collected %d replacement groups for %d configured obfuscators",
			len(s.replacements), len(s.config.Config.Obfuscate),
		))
		return
	}
	for i := range s.config.Config.Obfuscate {
		if s.config.Config.Obfuscate[i].Replacement == nil {
			s.config.Config.Obfuscate[i].Replacement = map[string]string{}
		}
		doneReplacements := s.replacements[i]
		for _, replacement := range doneReplacements {
			for _, oc := range replacement.Occurrences {
				s.config.Config.Obfuscate[i].Replacement[oc.Original] = replacement.ReplacedWith
			}
		}
	}
}

func (s *SimpleReporter) setCollectionError(err error) {
	if s.collectionErr == nil {
		s.collectionErr = err
	}
}

func NewSimpleReporter(config *schema.SchemaJson) Reporter {
	return &SimpleReporter{
		replacements: [][]Replacement{},
		omissions:    []string{},
		config:       config,
	}
}

func sortReplacements(replacements []Replacement) {
	sort.SliceStable(replacements, func(first, second int) bool {
		if replacements[first].Canonical != replacements[second].Canonical {
			return replacements[first].Canonical < replacements[second].Canonical
		}
		return replacements[first].ReplacedWith < replacements[second].ReplacedWith
	})
}
func sortOccurrences(occurrences []Occurrence) {
	sort.SliceStable(occurrences, func(first, second int) bool {
		if occurrences[first].Original != occurrences[second].Original {
			return occurrences[first].Original < occurrences[second].Original
		}
		return occurrences[first].Count < occurrences[second].Count
	})
}
