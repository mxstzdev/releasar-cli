package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	flagDryRun      bool
	flagNoInteraction bool
	flagVerbosity   int
	appVersion   string
)

var rootCmd = &cobra.Command{
	Use:          "releasar",
	Short:        "Local release workflow automation for Git-based projects",
	Version:      appVersion,
	SilenceUsage: true,
}

// Execute wires version and runs the root command.
func Execute(version string) {
	appVersion = version
	rootCmd.Version = version

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false, "walk through the flow without writing anything")
	rootCmd.PersistentFlags().BoolVarP(&flagNoInteraction, "no-interaction", "n", false, "skip interactive confirmations")
	rootCmd.PersistentFlags().CountVarP(&flagVerbosity, "verbose", "v", "detailed phase output (-v); debug logging to file (-vv)")
}
