package cli

import "github.com/openshift/must-gather-clean/pkg/schema"

func Run(configPath string, inputPath string, outputPath string) error {

	_, err := schema.ReadConfigFromPath(configPath)
	if err != nil {
		return err
	}

	return nil
}
