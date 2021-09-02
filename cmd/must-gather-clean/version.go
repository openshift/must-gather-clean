package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	VERSION = "v0.0.1"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of the tool",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println(VERSION)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
