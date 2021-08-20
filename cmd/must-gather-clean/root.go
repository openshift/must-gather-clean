package main

import (
	goflag "flag"
	"math/rand"
	"time"

	"k8s.io/klog/v2"

	"github.com/openshift/must-gather-clean/pkg/cli"
	"github.com/spf13/cobra"
)

var (
	ConfigFile         string
	InputFolder        string
	OutputFolder       string
	DeleteOutputFolder bool
	ReportingFolder    string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "must-gather-clean",
	Short: "Obfuscation for must-gather dumps",
	Long:  "This tool obfuscates sensitive information present in must-gather dumps based on input configuration",
	Run: func(cmd *cobra.Command, args []string) {
		defer klog.Flush()

		err := cli.Run(ConfigFile, InputFolder, OutputFolder, DeleteOutputFolder, ReportingFolder)
		if err != nil {
			klog.Exitf("%v\n", err)
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

	rootCmd.Flags().StringVarP(&ReportingFolder, "report", "r", ".", "The directory of the reporting output folder, default is the current working directory")

	fs := goflag.NewFlagSet("", goflag.ExitOnError)
	klog.InitFlags(fs)
	rootCmd.Flags().AddGoFlagSet(fs)
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	cobra.CheckErr(rootCmd.Execute())
}
