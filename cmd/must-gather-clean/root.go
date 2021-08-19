package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/openshift/must-gather-clean/pkg/cli"
	"github.com/spf13/cobra"
)

var (
	ConfigFile         string
	InputFolder        string
	OutputFolder       string
	DeleteOutputFolder bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "must-gather-clean",
	Short: "Obfuscation for must-gather dumps",
	Long:  "This tool obfuscates sensitive information present in must-gather dumps based on input configuration",
	Run: func(cmd *cobra.Command, args []string) {
		err := cli.Run(ConfigFile, InputFolder, OutputFolder, DeleteOutputFolder)
		if err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.Flags().StringVarP(&ConfigFile, "config", "c", "", "The path to the obfuscation configuration")
	_ = rootCmd.MarkFlagRequired("config")

	rootCmd.Flags().StringVarP(&InputFolder, "input", "i", "", "The directory of the must-gather dump")
	_ = rootCmd.MarkFlagRequired("input")

	rootCmd.Flags().StringVarP(&OutputFolder, "output", "o", "", "The directory of the obfuscated output")
	_ = rootCmd.MarkFlagRequired("output")

	rootCmd.Flags().BoolVarP(&DeleteOutputFolder, "overwrite", "d", false, "If the output directory exists, setting this flag will delete the folder and all its contents before cleaning.")
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	cobra.CheckErr(rootCmd.Execute())
}
