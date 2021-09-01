package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift/must-gather-clean/pkg/cleaner"
	"github.com/openshift/must-gather-clean/pkg/reporting"
	"k8s.io/klog/v2"

	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/omitter"
	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/openshift/must-gather-clean/pkg/traversal"
)

const (
	reportFileName = "report.yaml"
)

func RunPipe(configPath string) error {
	if configPath != "" {
		// TODO(thomas): read some config and do the same as below
		return fmt.Errorf("supplying config is not supported yet")
	} else {
		ipObfuscator, err := obfuscator.NewIPObfuscator(schema.ObfuscateReplacementTypeConsistent)
		if err != nil {
			return fmt.Errorf("failed to create IP obfuscator: %w", err)
		}

		multiObfuscator := obfuscator.NewMultiObfuscator([]obfuscator.ReportingObfuscator{
			ipObfuscator,
			obfuscator.NewMacAddressObfuscator(),
		})

		contentObfuscator := cleaner.ContentObfuscator{Obfuscator: multiObfuscator}
		err = contentObfuscator.ObfuscateReader(os.Stdin, os.Stdout)
		if err != nil {
			return fmt.Errorf("failed to obfuscate via pipe: %w", err)
		}
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

	var obfuscators []obfuscator.ReportingObfuscator
	for _, o := range config.Config.Obfuscate {
		switch o.Type {
		case schema.ObfuscateTypeKeywords:
			k := obfuscator.NewKeywordsObfuscator(o.Replacement)
			k = obfuscator.NewTargetObfuscator(o.Target, k)
			obfuscators = append(obfuscators, k)
		case schema.ObfuscateTypeMAC:
			k := obfuscator.NewTargetObfuscator(o.Target, obfuscator.NewMacAddressObfuscator())
			obfuscators = append(obfuscators, k)
		case schema.ObfuscateTypeRegex:
			k, err := obfuscator.NewRegexObfuscator(*o.Regex)
			if err != nil {
				return err
			}
			k = obfuscator.NewTargetObfuscator(o.Target, k)
			obfuscators = append(obfuscators, k)
		case schema.ObfuscateTypeDomain:
			k, err := obfuscator.NewDomainObfuscator(o.Domains)
			if err != nil {
				return err
			}
			k = obfuscator.NewTargetObfuscator(o.Target, k)
			obfuscators = append(obfuscators, k)
		case schema.ObfuscateTypeIP:
			k, err := obfuscator.NewIPObfuscator(o.ReplacementType)
			if err != nil {
				return err
			}
			k = obfuscator.NewTargetObfuscator(o.Target, k)
			obfuscators = append(obfuscators, k)
		}
	}

	var fileOmitters []omitter.FileOmitter
	var k8sOmitters []omitter.KubernetesResourceOmitter
	for _, o := range config.Config.Omit {
		switch o.Type {
		case schema.OmitTypeFile:
			om, err := omitter.NewFilenamePatternOmitter(*o.Pattern)
			if err != nil {
				return err
			}
			fileOmitters = append(fileOmitters, om)
		case schema.OmitTypeKubernetes:
			if o.KubernetesResource == nil {
				klog.Exitf("type Kubernetes must also include a 'kubernetesResource'. Given: %v", o)
			}
			kr := *o.KubernetesResource
			om, err := omitter.NewKubernetesResourceOmitter(kr.ApiVersion, kr.Kind, kr.Namespaces)
			if err != nil {
				return err
			}
			k8sOmitters = append(k8sOmitters, om)
		}
	}

	multiReportingOmitter := omitter.NewMultiReportingOmitter(fileOmitters, k8sOmitters)
	multiObfuscator := obfuscator.NewMultiObfuscator(obfuscators)
	fileCleaner := cleaner.NewFileCleaner(inputPath, outputPath, multiObfuscator, multiReportingOmitter)

	workerFactory := func(id int) traversal.QueueProcessor {
		return traversal.NewWorker(id, fileCleaner)
	}

	traversal.NewParallelFileWalker(inputPath, workerCount, workerFactory).Traverse()

	reporter := reporting.NewSimpleReporter()
	reporter.CollectOmitterReport(multiReportingOmitter.Report())
	reporter.CollectObfuscatorReport(multiObfuscator.ReportPerObfuscator())
	return reporter.WriteReport(filepath.Join(reportingFolder, reportFileName))
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
