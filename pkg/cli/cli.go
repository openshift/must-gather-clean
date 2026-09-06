package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	"sigs.k8s.io/yaml"
)

const (
	reportFileName = "report.yaml"
)

// detectPlatform reads the infrastructure.yaml from must-gather to detect the platform type
func detectPlatform(inputPath string) string {
	// Search for infrastructure.yaml - must-gather structure has subdirectories
	pattern := filepath.Join(inputPath, "*/cluster-scoped-resources/config.openshift.io/infrastructures/cluster.yaml")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		// Try direct path as fallback
		infraPath := filepath.Join(inputPath, "cluster-scoped-resources", "config.openshift.io", "infrastructures", "cluster.yaml")
		matches = []string{infraPath}
	}

	var data []byte
	for _, infraPath := range matches {
		data, err = os.ReadFile(infraPath)
		if err == nil {
			break
		}
	}

	if data == nil {
		klog.V(2).Info("Could not find infrastructure.yaml in must-gather (will run Azure obfuscation)")
		return ""
	}

	// Parse YAML
	var infra struct {
		Status struct {
			Platform string `json:"platform"`
		} `json:"status"`
	}

	err = yaml.Unmarshal(data, &infra)
	if err != nil {
		klog.V(2).Infof("Could not parse infrastructure YAML: %v (will run Azure obfuscation)", err)
		return ""
	}

	platform := strings.ToLower(infra.Status.Platform)
	klog.Infof("Detected platform: %s", infra.Status.Platform)
	return platform
}

func RunPipe(configPath string, stdin io.Reader, stdout io.Writer) error {
	var multiObfuscator *obfuscator.MultiObfuscator
	if configPath != "" {
		config, err := schema.ReadConfigFromPath(configPath)
		if err != nil {
			return fmt.Errorf("failed to read config at %s: %w", configPath, err)
		}
		// we cannot logically prescan because the end of input isn't clear
		// For pipe mode, we cannot auto-detect platform, so we include all obfuscators
		multiObfuscator, _, err = createObfuscatorsFromConfig(config, false)
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

func Run(configPath string, inputPath string, outputPath string, deleteOutputFolder bool, reportingFolder string, workerCount int, skipAzure bool) error {
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

	// Auto-detect platform if skipAzure not explicitly set
	if !skipAzure {
		platform := detectPlatform(inputPath)
		// Only run Azure obfuscation for Azure clusters
		if platform != "" && platform != "azure" {
			klog.Infof("Platform %s detected, skipping Azure resource obfuscation prescan", platform)
			skipAzure = true
		}
	} else {
		klog.Info("--skip-azure flag set, skipping Azure resource obfuscation prescan")
	}

	obfuscator, prescanObfuscator, err := createObfuscatorsFromConfig(config, skipAzure)
	if err != nil {
		return fmt.Errorf("failed to create obfuscators via config at %s: %w", configPath, err)
	}

	// Only run prescan if not skipping Azure (currently only Azure requires prescan)
	if !skipAzure {
		// this pass allows obfuscators that first need to scan the input to determine what needs to be obfuscated to run before
		// redactor actually happens. The empty input path signals a dry-run.
		klog.Info("Running prescan phase for Azure resource obfuscation")
		prescanCleaner := cleaner.NewFileCleaner(inputPath, "", prescanObfuscator, &omitter.NoopOmitter{})
		prescanWorkerFactory := func(id int) traversal.QueueProcessor {
			return traversal.NewWorker(id, prescanCleaner)
		}
		traversal.NewParallelFileWalker(inputPath, workerCount, prescanWorkerFactory).Traverse()
	} else {
		klog.Info("Skipping prescan phase entirely (no Azure obfuscation needed)")
	}

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
func createObfuscatorsFromConfig(config *schema.SchemaJson, skipAzure bool) (finalObfuscator *obfuscator.MultiObfuscator, prescanObfuscator *obfuscator.MultiObfuscator, finalErr error) {
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
			if skipAzure {
				// Use no-op obfuscator for non-Azure platforms
				klog.Info("Using no-op Azure obfuscator (non-Azure platform detected)")
				k = &obfuscator.NoopObfuscator{Replacements: make(map[string]string)}
			} else {
				// Create real Azure obfuscator for Azure platforms
				k, err = obfuscator.NewAzureResourceObfuscator(o.ReplacementType, tracker, config.Config.RandSeed)
				if err != nil {
					return nil, nil, err
				}
				prescanObfuscators = append(prescanObfuscators, k)
			}
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
