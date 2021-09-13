package main

import (
	version "github.com/openshift/must-gather-clean/pkg/version"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of the tool",
	Run: func(_ *cobra.Command, _ []string) {
		version.GetVersion().Print()
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
