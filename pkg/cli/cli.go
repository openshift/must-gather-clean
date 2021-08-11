package cli

import (
	"fmt"

	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/omitter"
	"github.com/openshift/must-gather-clean/pkg/output"
	"github.com/openshift/must-gather-clean/pkg/schema"
	"github.com/openshift/must-gather-clean/pkg/traversal"
)

func Run(configPath string, inputPath string, outputPath string) error {

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
			obfuscators = append(obfuscators, obfuscator.NewDomainObfuscator(config.Config.TopLevelDomains))
		case schema.ObfuscateTypeIP:
			obfuscators = append(obfuscators, obfuscator.NewIPObfuscator())
		}
	}

	var omitters []omitter.Omitter
	for _, o := range config.Config.Omit {
		switch om := o.(type) {
		case schema.FileOmission:
			omitters = append(omitters, omitter.NewFilenamePatternOmitter(om.Pattern))
		}
	}

	writer, err := output.NewFSWriter(outputPath)
	if err != nil {
		return err
	}
	walker, err := traversal.NewFileWalker(inputPath, writer, obfuscators, omitters)
	if err != nil {
		return err
	}

	err = walker.Traverse()
	if err != nil {
		return fmt.Errorf("failed to generate obfuscated output: %w", err)
	}

	return nil
}
