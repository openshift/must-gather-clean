package main

import (
	"os"

	"github.com/openshift/must-gather-clean/pkg/cli"
	"github.com/spf13/cobra"
)

var deobfuscationReportPath string

var deobfuscateCmd = &cobra.Command{
	Use:   "deobfuscate",
	Short: "Restore values in a support response using an obfuscation report",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		if PipeModeEnabled {
			return cli.RunDeobfuscate(deobfuscationReportPath, os.Stdin, os.Stdout)
		}
		return cli.RunDeobfuscateFile(deobfuscationReportPath, InputFolder, OutputFolder)
	},
}

func init() {
	flags := deobfuscateCmd.Flags()
	flags.StringVar(&deobfuscationReportPath, "report", "", "The obfuscation report for the cleaned must-gather")
	flags.StringVarP(&InputFolder, "input", "i", "", "The support response file")
	flags.StringVarP(&OutputFolder, "output", "o", "", "The output file")
	_ = deobfuscateCmd.MarkFlagRequired("report")
	rootCmd.AddCommand(deobfuscateCmd)
}
