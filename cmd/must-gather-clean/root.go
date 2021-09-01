package main

import (
	goflag "flag"
	"math/rand"
	"os"
	"runtime"
	"time"

	"k8s.io/klog/v2"

	"github.com/openshift/must-gather-clean/pkg/cli"
	"github.com/spf13/cobra"
)

var (
	PipeModeEnabled    bool
	ConfigFile         string
	DeleteOutputFolder bool
	InputFolder        string
	OutputFolder       string
	ReportingFolder    string
	WorkerCount        int
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "must-gather-clean",
	Short: "Obfuscation for must-gather dumps",
	Long:  "This tool obfuscates sensitive information present in must-gather dumps based on input configuration",
	Run: func(cmd *cobra.Command, args []string) {
		defer klog.Flush()

		if PipeModeEnabled {
			err := cli.RunPipe(ConfigFile)
			if err != nil {
				klog.Exitf("%v\n", err)
			}
		} else {
			err := cli.Run(ConfigFile, InputFolder, OutputFolder, DeleteOutputFolder, ReportingFolder, WorkerCount)
			if err != nil {
				klog.Exitf("%v\n", err)
			}
		}
	},
}

func initFlags() {
	flags := rootCmd.Flags()
	flags.StringVarP(&ConfigFile, "config", "c", "", "The path to the obfuscation configuration")
	flags.StringVarP(&InputFolder, "input", "i", "", "The directory of the must-gather dump")
	flags.StringVarP(&OutputFolder, "output", "o", "", "The directory of the obfuscated output")
	flags.BoolVarP(&DeleteOutputFolder, "overwrite", "d", false, "If the output directory exists, setting this flag will delete the folder and all its contents before cleaning.")
	flags.IntVarP(&WorkerCount, "worker-count", "w", runtime.NumCPU(), "The number of workers for processing")
	flags.StringVarP(&ReportingFolder, "report", "r", ".", "The directory of the reporting output folder, default is the current working directory")

	if !PipeModeEnabled {
		_ = rootCmd.MarkFlagRequired("config")
		_ = rootCmd.MarkFlagRequired("input")
		_ = rootCmd.MarkFlagRequired("output")
	}

	fs := goflag.NewFlagSet("", goflag.ExitOnError)
	klog.InitFlags(fs)
	rootCmd.Flags().AddGoFlagSet(fs)
}

func main() {
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		PipeModeEnabled = true
	}

	initFlags()

	rand.Seed(time.Now().UTC().UnixNano())
	cobra.CheckErr(rootCmd.Execute())
}
