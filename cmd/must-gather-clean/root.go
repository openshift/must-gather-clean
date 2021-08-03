package main

import (
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "must-gather-clean",
	Short: "Obfuscation for must-gather dumps",
	Long:  "This tool obfuscates sensitive information present in must-gather dumps based on input configuration",
	Run:   func(cmd *cobra.Command, args []string) {},
}

func init() {
	rootCmd.Flags().StringP("config", "c", "", "The path to the obfuscation configuration")
	rootCmd.Flags().StringP("input", "i", "", "The directory of the must-gather dump")
	rootCmd.Flags().StringP("output", "o", "", "The directory of the obfuscated output")
}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}
