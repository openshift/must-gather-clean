package reporting

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

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

const reportTemporaryFilePrefix = ".mgc-report-"

const reportDirectoryMode = os.FileMode(0700)

var reportPathPreparationMu sync.Mutex

func (s *SimpleReporter) WriteReport(path string) error {
	return s.writeReport(path, nil)
}

type reportWriteHooks struct {
	afterOpenRoot             func()
	afterTemporaryFileCreated func(string, *os.File)
}

func (s *SimpleReporter) writeReport(path string, hooks *reportWriteHooks) error {
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

	targetPath, err := prepareReportPath(path)
	if err != nil {
		return err
	}
	reportingFolder := filepath.Dir(targetPath)

	// Pin the destination directory before staging anything. Directory replacement
	// after this point cannot redirect publication to a different tree.
	reportingRoot, err := os.OpenRoot(reportingFolder)
	if err != nil {
		return fmt.Errorf("failed to open reporting output folder: %w", err)
	}
	defer func() {
		_ = reportingRoot.Close()
	}()
	if hooks != nil && hooks.afterOpenRoot != nil {
		hooks.afterOpenRoot()
	}

	targetName := filepath.Base(targetPath)
	existingMode, existingReport, err := reportFileMode(reportingRoot, targetName)
	if err != nil {
		return fmt.Errorf("failed to inspect report path %s: %w", path, err)
	}

	// Stage one uniquely named file beside the final target, then publish it with
	// an atomic rename. The report uses base mode 0666 filtered by umask for a new
	// file, or the existing file's permission and special mode bits. Replacing the
	// directory entry does not preserve the old inode's owner, ACLs, extended
	// attributes, or hard links.
	temporaryName, reportFile, err := createTemporaryReportFile(reportingRoot)
	if err != nil {
		return fmt.Errorf("failed to open report file %s: %w", path, err)
	}
	defer func() {
		_ = reportingRoot.Remove(temporaryName)
	}()
	defer func() {
		_ = reportFile.Close()
	}()
	if existingReport {
		// Do not expose a temporary report with permissions broader than the target
		// while encoding. The complete mode, including special bits, is restored
		// after writing because some filesystems clear setgid on modification.
		if err := reportFile.Chmod(existingMode.Perm()); err != nil {
			return fmt.Errorf("failed to restrict temporary report mode for %s: %w", path, err)
		}
	}
	if hooks != nil && hooks.afterTemporaryFileCreated != nil {
		hooks.afterTemporaryFileCreated(temporaryName, reportFile)
	}

	rEncoder := yaml.NewEncoder(reportFile)
	err = rEncoder.Encode(Report{
		Replacements: s.replacements,
		Omissions:    s.omissions,
		Config:       s.config.Config,
	})
	if err != nil {
		_ = rEncoder.Close()
		return fmt.Errorf("failed to write report at %s: %w", path, err)
	}
	if err := rEncoder.Close(); err != nil {
		return fmt.Errorf("failed to finish report at %s: %w", path, err)
	}
	if existingReport {
		// Apply special mode bits after writing: some filesystems clear setgid
		// when an unprivileged writer modifies the file.
		if err := reportFile.Chmod(existingMode); err != nil {
			return fmt.Errorf("failed to preserve report mode for %s: %w", path, err)
		}
	}
	if err := reportFile.Close(); err != nil {
		return fmt.Errorf("failed to close report at %s: %w", path, err)
	}
	if err := reportingRoot.Rename(temporaryName, targetName); err != nil {
		return fmt.Errorf("failed to replace report at %s: %w", path, err)
	}

	klog.V(3).Infof("successfully saved obfuscation report in %s", path)

	return nil
}

func prepareReportPath(path string) (string, error) {
	// Path resolution and directory provisioning must be serialized within this
	// process: a restrictive umask briefly publishes a new directory without owner
	// access, before createReportDirectory restores it.
	reportPathPreparationMu.Lock()
	defer reportPathPreparationMu.Unlock()

	targetPath, err := fsutil.ResolveSymlink(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve report path %s: %w", path, err)
	}
	if err := createReportDirectory(filepath.Dir(targetPath)); err != nil {
		return "", fmt.Errorf("failed to create reporting output folder: %w", err)
	}
	return targetPath, nil
}

func createReportDirectory(path string) error {
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", path)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	parent := filepath.Dir(path)
	if parent == path {
		return err
	}
	if err := createReportDirectory(parent); err != nil {
		return err
	}
	if err := os.Mkdir(path, reportDirectoryMode); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err = os.Stat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", path)
		}
		return nil
	}

	// A restrictive umask can remove owner access, preventing creation of nested
	// directories or publication of the report. Restore the complete owner mode on
	// directories created by this call while retaining inherited special bits.
	info, err = os.Stat(path)
	if err != nil {
		return err
	}
	mode := info.Mode() & (os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
	if err := os.Chmod(path, mode|reportDirectoryMode); err != nil {
		return err
	}
	return nil
}

func createTemporaryReportFile(root *os.Root) (string, *os.File, error) {
	for range 100 {
		name := reportTemporaryFilePrefix + rand.Text()
		file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", nil, err
		}
		return name, file, nil
	}
	return "", nil, fmt.Errorf("could not allocate a unique temporary report file")
}

func reportFileMode(root *os.Root, name string) (os.FileMode, bool, error) {
	info, err := root.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if !info.Mode().IsRegular() {
		return 0, false, fmt.Errorf("path is not a regular file: %s", info.Mode().Type())
	}
	return info.Mode() & (os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky), true, nil
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
