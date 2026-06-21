package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	flagVerbosity  int
	flagWorkingDir string
	flagQuiet      bool
	appVersion     string
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
	rootCmd.PersistentFlags().CountVarP(&flagVerbosity, "verbose", "v", "detailed phase output (-v); debug logging to file (-vv)")
	rootCmd.PersistentFlags().StringVarP(&flagWorkingDir, "working-dir", "d", "", "set working directory")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "suppress all output except errors")
}
