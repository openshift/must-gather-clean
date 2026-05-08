package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openshift/must-gather-clean/pkg/cleaner"
	"github.com/openshift/must-gather-clean/pkg/fsutil"
	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/omitter"
	"github.com/openshift/must-gather-clean/pkg/reporting"
	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/openshift/must-gather-clean/pkg/traversal"
	watermarking "github.com/openshift/must-gather-clean/pkg/watermarker"
	"k8s.io/klog/v2"
)

const (
	reportFileName = "report.yaml"
)

func RunPipe(configPath string, stdin io.Reader, stdout io.Writer) error {
	var multiObfuscator *obfuscator.MultiObfuscator
	if configPath != "" {
		config, err := schema.ReadConfigFromPath(configPath)
		if err != nil {
			return fmt.Errorf("failed to read config at %s: %w", configPath, err)
		}
		// we cannot logically prescan because the end of input isn't clear
		multiObfuscator, _, err = createObfuscatorsFromConfig(config)
		if err != nil {
			return fmt.Errorf("failed to create obfuscators via config at %s: %w", configPath, err)
		}
	} else {
		ipObfuscator, err := obfuscator.NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, obfuscator.NewSimpleTracker())
		if err != nil {
			return fmt.Errorf("failed to create IP obfuscator: %w", err)
		}

		macObfuscator, err := obfuscator.NewMacAddressObfuscator(schema.ObfuscateReplacementTypeConsistent, obfuscator.NewSimpleTracker())
		if err != nil {
			return fmt.Errorf("failed to create MAC obfuscator: %w", err)
		}

		multiObfuscator = obfuscator.NewMultiObfuscator([]obfuscator.ReportingObfuscator{
			ipObfuscator,
			macObfuscator,
		})
	}

	contentObfuscator := cleaner.ContentObfuscator{Obfuscator: multiObfuscator}
	err := contentObfuscator.ObfuscateReader(stdin, stdout)
	if err != nil {
		return fmt.Errorf("failed to obfuscate via pipe: %w", err)
	}

	return nil
}

func Run(configPath string, inputPath string, outputPath string, deleteOutputFolder bool, reportingFolder string, workerCount int) error {
	if workerCount < 1 {
		return fmt.Errorf("invalid number of workers specified %d", workerCount)
	}

	err := fsutil.EnsureInputOutputPath(inputPath, outputPath, deleteOutputFolder)
	if err != nil {
		return err
	}

	config, err := schema.ReadConfigFromPath(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config at %s: %w", configPath, err)
	}

	obfuscator, prescanObfuscator, err := createObfuscatorsFromConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create obfuscators via config at %s: %w", configPath, err)
	}

	// Seed Azure Resource Obfuscators with cluster token extracted from
	// input file names. This enables compound discovery during prescan
	// even when the dataset contains no ARM paths.
	seedObfuscatorsFromInputDir(inputPath, obfuscator, prescanObfuscator)

	// this pass allows obfuscators that first need to scan the input to determine what needs to be obfuscated to run before
	// redactor actually happens. The empty input path signals a dry-run.
	prescanCleaner := cleaner.NewFileCleaner(inputPath, "", prescanObfuscator, &omitter.NoopOmitter{})
	prescanWorkerFactory := func(id int) traversal.QueueProcessor {
		return traversal.NewWorker(id, prescanCleaner)
	}
	traversal.NewParallelFileWalker(inputPath, workerCount, prescanWorkerFactory).Traverse()

	mro, err := createOmittersFromConfig(config, inputPath)
	if err != nil {
		return fmt.Errorf("failed to create omitters via config at %s: %w", configPath, err)
	}
	fileCleaner := cleaner.NewFileCleaner(inputPath, outputPath, obfuscator, mro)

	workerFactory := func(id int) traversal.QueueProcessor {
		return traversal.NewWorker(id, fileCleaner)
	}
	traversal.NewParallelFileWalker(inputPath, workerCount, workerFactory).Traverse()

	reporter := reporting.NewSimpleReporter(config)
	reporter.CollectOmitterReport(mro.Report())
	reporter.CollectObfuscatorReport(obfuscator.ReportPerObfuscator())
	reporterErr := reporter.WriteReport(filepath.Join(reportingFolder, reportFileName))
	if reporterErr != nil {
		return reporterErr
	}

	watermarker := watermarking.NewSimpleWaterMarker()
	return watermarker.WriteWaterMarkFile(outputPath)
}

func createOmittersFromConfig(config *schema.SchemaJson, inputPath string) (omitter.ReportingOmitter, error) {
	var fileOmitters []omitter.FileOmitter
	var k8sOmitters []omitter.KubernetesResourceOmitter
	for _, o := range config.Config.Omit {
		switch o.Type {
		case schema.OmitTypeSymbolicLink:
			fileOmitters = append(fileOmitters, omitter.NewSymlinkOmitter(inputPath))
		case schema.OmitTypeFile:
			om, err := omitter.NewFilenamePatternOmitter(*o.Pattern)
			if err != nil {
				return nil, err
			}
			fileOmitters = append(fileOmitters, om)
		case schema.OmitTypeKubernetes:
			if o.KubernetesResource == nil {
				klog.Exitf("type Kubernetes must also include a 'kubernetesResource'. Given: %v", o)
			}
			kr := *o.KubernetesResource
			om, err := omitter.NewKubernetesResourceOmitter(kr.ApiVersion, kr.Kind, kr.Namespaces)
			if err != nil {
				return nil, err
			}
			k8sOmitters = append(k8sOmitters, om)
		}
	}

	return omitter.NewMultiReportingOmitter(fileOmitters, k8sOmitters), nil
}

// finalObfuscator is the obfuscator to use to actually clean a directory.
// prescanObfuscator is an obfuscator that shares some instances of individual obfuscators with the finalObfuscator, but is run in
// a dryRun mode (no output directory) to pre-scan the input and determine the full set of strings to elide.  This allows for
// usage patterns like:
//
//	file/B (exact name unknown) may contain strings like /subscription/ID, where ID needs to be redacted in all files,
//	but file/A contains only ID.  We won't recognize ID as needing redaction until we read file/B.  This means we need to first
//	scan all files, then redact.
func createObfuscatorsFromConfig(config *schema.SchemaJson) (finalObfuscator *obfuscator.MultiObfuscator, prescanObfuscator *obfuscator.MultiObfuscator, finalErr error) {
	var obfuscators []obfuscator.ReportingObfuscator
	var prescanObfuscators []obfuscator.ReportingObfuscator
	for _, o := range config.Config.Obfuscate {
		var (
			k   obfuscator.ReportingObfuscator
			err error
		)
		tracker := obfuscator.NewSimpleTrackerMap(o.Replacement)
		switch o.Type {
		case schema.ObfuscateTypeKeywords:
			k = obfuscator.NewKeywordsObfuscator(o.Replacement)
		case schema.ObfuscateTypeMAC:
			k, err = obfuscator.NewMacAddressObfuscator(o.ReplacementType, tracker)
			if err != nil {
				return nil, nil, err
			}
		case schema.ObfuscateTypeRegex:
			k, err = obfuscator.NewRegexObfuscator(*o.Regex, tracker)
			if err != nil {
				return nil, nil, err
			}
		case schema.ObfuscateTypeDomain:
			k, err = obfuscator.NewDomainObfuscator(o.DomainNames, o.ReplacementType, tracker)
			if err != nil {
				return nil, nil, err
			}
		case schema.ObfuscateTypeAzureResources:
			k, err = obfuscator.NewAzureResourceObfuscator(o.ReplacementType, tracker, config.Config.RandSeed)
			if err != nil {
				return nil, nil, err
			}
			prescanObfuscators = append(prescanObfuscators, k)
		case schema.ObfuscateTypeExact:
			k = obfuscator.NewExactReplacementObfuscator(o.ExactReplacements, tracker)
		case schema.ObfuscateTypeIP:
			k, err = obfuscator.NewIPObfuscator(o.ReplacementType, tracker)
			if err != nil {
				return nil, nil, err
			}
		default:
			return nil, nil, fmt.Errorf("unknown obfuscator type %s", o.Type)
		}
		k = obfuscator.NewTargetObfuscator(o.Target, k)
		obfuscators = append(obfuscators, k)
	}
	return obfuscator.NewMultiObfuscator(obfuscators), obfuscator.NewMultiObfuscator(prescanObfuscators), nil
}

// preferredSeedDirs are the must-gather filesystem subdirectory names most likely
// to contain files following the <env>-<clusterID>-<type>-<namespace>-<container>
// naming convention used for cluster token extraction. These are directory names,
// not semantic keywords — the overlap with genericSkipWords is incidental.
//
// "mgmt" is included because ARO-HCP must-gathers contain a separate management
// cluster log directory whose prefix (e.g. "hcp-underlay-cd-mgmt-1") is entirely
// different from the service cluster prefix and must be seeded independently.
var preferredSeedDirs = map[string]struct{}{
	"service": {},
	"cluster": {},
	"mgmt":    {},
}

// collectFileNames reads non-directory entries from each subdirectory in dirs
// and returns their base names.
func collectFileNames(inputPath string, dirs []string) []string {
	var fileNames []string
	for _, dir := range dirs {
		dirPath := filepath.Join(inputPath, dir)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				fileNames = append(fileNames, e.Name())
			}
		}
	}
	return fileNames
}

// seedObfuscatorsFromInputDir extracts cluster prefixes from must-gather file
// names and seeds them into all SeedableObfuscators. Each preferred directory
// is processed independently so that a must-gather containing both a "service"
// cluster and a "mgmt" cluster seeds both prefixes even when they share no
// common prefix. Falls back to all other subdirectories only if no preferred
// directory yields a valid prefix.
func seedObfuscatorsFromInputDir(inputPath string, obfuscators ...*obfuscator.MultiObfuscator) {
	primaryDirs := make([]string, 0, len(preferredSeedDirs))
	for dir := range preferredSeedDirs {
		primaryDirs = append(primaryDirs, dir)
	}
	sort.Strings(primaryDirs)

	seededAny := false
	for _, dir := range primaryDirs {
		fileNames := collectFileNames(inputPath, []string{dir})
		prefix, token := obfuscator.ExtractClusterInfo(fileNames)
		if prefix == "" {
			continue
		}
		klog.Infof("discovered cluster prefix %q from input directory %q (token %q)", prefix, dir, token)
		seedOnePrefix(prefix, token, obfuscators...)
		seededAny = true
	}

	if seededAny {
		return
	}

	// Fallback: no preferred dir yielded a prefix — try all other subdirs pooled together.
	entries, err := os.ReadDir(inputPath)
	if err != nil {
		return
	}
	var fallbackDirs []string
	for _, e := range entries {
		if e.IsDir() {
			if _, preferred := preferredSeedDirs[e.Name()]; !preferred {
				fallbackDirs = append(fallbackDirs, e.Name())
			}
		}
	}
	fileNames := collectFileNames(inputPath, fallbackDirs)
	prefix, token := obfuscator.ExtractClusterInfo(fileNames)
	if prefix == "" {
		return
	}
	klog.Infof("discovered cluster prefix %q from fallback directories (token %q)", prefix, token)
	seedOnePrefix(prefix, token, obfuscators...)
}

// seedOnePrefix seeds a single cluster prefix (and optional token) into all
// SeedableObfuscators, deduplicating by pointer identity so that an obfuscator
// shared between the final and prescan multi-obfuscators is only seeded once
// per prefix.
func seedOnePrefix(prefix, token string, obfuscators ...*obfuscator.MultiObfuscator) {
	concatenated := strings.ReplaceAll(prefix, "-", "")
	seen := map[obfuscator.SeedableObfuscator]struct{}{}
	for _, mo := range obfuscators {
		for _, o := range mo.SeedableObfuscators() {
			if _, dup := seen[o]; dup {
				continue
			}
			seen[o] = struct{}{}
			o.SeedCanonical(prefix)
			if concatenated != prefix {
				o.SeedCanonical(concatenated)
			}
			if token != "" {
				o.SetClusterToken(token)
			}
		}
	}
}
