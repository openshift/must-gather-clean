package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"

	"github.com/openshift/must-gather-clean/pkg/input"
	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/omitter"
	"github.com/openshift/must-gather-clean/pkg/output"
	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/openshift/must-gather-clean/pkg/traversal"
)

const (
	reportFileName = "report.yaml"
)

func Run(configPath string, inputPath string, outputPath string, deleteOutputFolder bool, reportingFolder string) error {

	err := output.EnsureOutputPath(outputPath, deleteOutputFolder)
	if err != nil {
		return fmt.Errorf("failed to ensure output folder: %w", err)
	}

	config, err := schema.ReadConfigFromPath(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config at %s: %w", configPath, err)
	}

	var obfuscators []obfuscator.Obfuscator
	for _, o := range config.Config.Obfuscate {
		switch o.Type {
		case schema.ObfuscateTypeKeywords:
			obfuscators = append(obfuscators, obfuscator.NewKeywordsObfuscator(o.Replacement))
		case schema.ObfuscateTypeMAC:
			obfuscators = append(obfuscators, obfuscator.NewMacAddressObfuscator())
		case schema.ObfuscateTypeRegex:
			o, err := obfuscator.NewRegexObfuscator(*o.Regex, o.Target)
			if err != nil {
				return err
			}
			obfuscators = append(obfuscators, o)
		case schema.ObfuscateTypeDomain:
			o, err := obfuscator.NewDomainObfuscator(o.Domains)
			if err != nil {
				return err
			}
			obfuscators = append(obfuscators, o)
		case schema.ObfuscateTypeIP:
			o, err := obfuscator.NewIPObfuscator(o.ReplacementType)
			if err != nil {
				return err
			}
			obfuscators = append(obfuscators, o)
		}
	}

	var omitters []omitter.Omitter
	for _, o := range config.Config.Omit {
		switch o.Type {
		case schema.OmitTypeFile:
			om, err := omitter.NewFilenamePatternOmitter(*o.Pattern)
			if err != nil {
				return err
			}
			omitters = append(omitters, om)
		case schema.OmitTypeKubernetes:
			if o.KubernetesResource == nil {
				klog.Exitf("type Kubernetes must also include a 'kubernetesResource'. Given: %v", o)
			}
			kr := *o.KubernetesResource
			om, err := omitter.NewKubernetesResourceOmitter(kr.ApiVersion, kr.Kind, kr.Namespaces)
			if err != nil {
				return err
			}
			omitters = append(omitters, om)
		}
	}

	reader, err := input.NewFSInput(inputPath)
	if err != nil {
		return err
	}
	writer, err := output.NewFSWriter(outputPath)
	if err != nil {
		return err
	}
	walker, err := traversal.NewFileWalker(reader, writer, obfuscators, omitters)
	if err != nil {
		return err
	}

	err = walker.Traverse()
	if err != nil {
		return fmt.Errorf("failed to generate obfuscated output: %w", err)
	}

	report := walker.GenerateReport()

	err = os.MkdirAll(reportingFolder, 0700)
	if err != nil {
		return fmt.Errorf("failed to create reporting output folder: %w", err)
	}

	reportingFile := filepath.Join(reportingFolder, reportFileName)
	reportFile, err := os.Create(reportingFile)
	if err != nil {
		return fmt.Errorf("failed to open report file %s: %w", reportingFile, err)
	}
	rEncoder := yaml.NewEncoder(reportFile)
	err = rEncoder.Encode(report)
	if err != nil {
		return fmt.Errorf("failed to write report at %s: %w", reportingFile, err)
	}

	klog.V(2).Infof("successfully saved obfuscation report in %s", reportingFile)

	return nil
}
