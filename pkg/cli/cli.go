package cli

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/openshift/must-gather-clean/pkg/cleaner"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"

	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/omitter"
	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/openshift/must-gather-clean/pkg/traversal"
	watermarking "github.com/openshift/must-gather-clean/pkg/watermarker"
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
		multiObfuscator, err = createObfuscatorsFromConfig(config)
		if err != nil {
			return fmt.Errorf("failed to create obfuscators via config at %s: %w", configPath, err)
		}
	} else {
		ipObfuscator, err := obfuscator.NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent, map[string]string{})
		if err != nil {
			return fmt.Errorf("failed to create IP obfuscator: %w", err)
		}

		macObfuscator, err := obfuscator.NewMacAddressObfuscator(schema.ObfuscateReplacementTypeConsistent, map[string]string{})
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

	err := ensureOutputPath(outputPath, deleteOutputFolder)
	if err != nil {
		return fmt.Errorf("failed to ensure output folder: %w", err)
	}

	config, err := schema.ReadConfigFromPath(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config at %s: %w", configPath, err)
	}

	mo, err := createObfuscatorsFromConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create obfuscators via config at %s: %w", configPath, err)
	}

	mro, err := createOmittersFromConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create omitters via config at %s: %w", configPath, err)
	}
	fileCleaner := cleaner.NewFileCleaner(inputPath, outputPath, mo, mro)

	workerFactory := func(id int) traversal.QueueProcessor {
		return traversal.NewWorker(id, fileCleaner)
	}

	traversal.NewParallelFileWalker(inputPath, workerCount, workerFactory).Traverse()

	mo.UpdateReportPerObfuscator(config)
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to encode the report at %s: %w", reportFileName, err)
	}

	err = ioutil.WriteFile(filepath.Join(reportingFolder, reportFileName), data, 0644)
	if err != nil {
		return fmt.Errorf("failed to generate the report at %s: %w", reportFileName, err)
	}

	/*
		reporter := reporting.NewSimpleReporter()
		reporter.CollectOmitterReport(mro.Report())
		reporter.CollectObfuscatorReport(mo.ReportPerObfuscator())
		reporterErr := reporter.WriteReport(filepath.Join(reportingFolder, reportFileName))
		if reporterErr != nil {
			return reporterErr
		}
	*/
	watermarker := watermarking.NewSimpleWaterMarker()
	return watermarker.WriteWaterMarkFile(outputPath)
}

func createOmittersFromConfig(config *schema.SchemaJson) (omitter.ReportingOmitter, error) {
	var fileOmitters []omitter.FileOmitter
	var k8sOmitters []omitter.KubernetesResourceOmitter
	for _, o := range config.Config.Omit {
		switch o.Type {
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

func createObfuscatorsFromConfig(config *schema.SchemaJson) (*obfuscator.MultiObfuscator, error) {
	var obfuscators []obfuscator.ReportingObfuscator
	for _, o := range config.Config.Obfuscate {
		var (
			k   obfuscator.ReportingObfuscator
			err error
		)
		switch o.Type {
		case schema.ObfuscateTypeKeywords:
			k = obfuscator.NewKeywordsObfuscator(o.Replacement, o.Report)
		case schema.ObfuscateTypeMAC:
			k, err = obfuscator.NewMacAddressObfuscator(o.ReplacementType, o.Report)
			if err != nil {
				return nil, err
			}
		case schema.ObfuscateTypeRegex:
			k, err = obfuscator.NewRegexObfuscator(*o.Regex, o.Report)
			if err != nil {
				return nil, err
			}
		case schema.ObfuscateTypeDomain:
			k, err = obfuscator.NewDomainObfuscator(o.DomainNames, o.ReplacementType, o.Report)
			if err != nil {
				return nil, err
			}
		case schema.ObfuscateTypeIP:
			k, err = obfuscator.NewIPObfuscator(o.ReplacementType, o.Report)
			if err != nil {
				return nil, err
			}
		}
		k = obfuscator.NewTargetObfuscator(o.Target, k)
		obfuscators = append(obfuscators, k)
	}
	return obfuscator.NewMultiObfuscator(obfuscators), nil
}

func ensureOutputPath(path string, deleteIfExists bool) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return os.Mkdir(path, 0700)
		}
		return err
	}

	if !info.IsDir() {
		return fmt.Errorf("output destination must be a directory: '%s'", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to get contents of output directory '%s': %w", path, err)
	}

	if len(entries) != 0 {
		if deleteIfExists {
			err = os.RemoveAll(path)
			if err != nil {
				return fmt.Errorf("error while deleting the output path '%s': %w", path, err)
			}
		} else {
			return fmt.Errorf("output directory %s is not empty", path)
		}
	}

	err = os.MkdirAll(path, 0700)
	if err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", path, err)
	}

	return nil
}
