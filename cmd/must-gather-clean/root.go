package main

import (
	"github.com/openshift/must-gather-clean/pkg/cli"
	"github.com/spf13/cobra"
	"math/rand"
	"os"
	"runtime"
	"time"
)

var (
	ConfigFile   string
	InputFolder  string
	OutputFolder string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "must-gather-clean",
	Short: "Obfuscation for must-gather dumps",
	Long:  "This tool obfuscates sensitive information present in must-gather dumps based on input configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cli.Run(ConfigFile, InputFolder, OutputFolder)
	},
}

func init() {
	rootCmd.Flags().StringVarP(&ConfigFile, "config", "c", "", "The path to the obfuscation configuration")
	_ = rootCmd.MarkFlagRequired("config")

	rootCmd.Flags().StringVarP(&InputFolder, "input", "i", "", "The directory of the must-gather dump")
	_ = rootCmd.MarkFlagRequired("input")

	rootCmd.Flags().StringVarP(&OutputFolder, "output", "o", "", "The directory of the obfuscated output")
	_ = rootCmd.MarkFlagRequired("output")
}

func main() {
	// set some basic consistency with OC
	rand.Seed(time.Now().UTC().UnixNano())
	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	cobra.CheckErr(rootCmd.Execute())
}
