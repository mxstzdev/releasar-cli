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
	Short:        "Local-first release automation for Git-based projects — no pipeline, no cloud runner",
	Version:      appVersion,
	SilenceUsage: true,
}

// Execute wires version and runs the root command.
func Execute(version string) {
	appVersion = version
	rootCmd.Version = version

	// Pre-initialize completion so it is registered before Execute adds the help
	// command — InitDefaultCompletionCmd is a no-op if already registered, so this
	// locks in its position ahead of help in the Additional Commands section.
	rootCmd.InitDefaultCompletionCmd()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.EnableCommandSorting = false
	rootCmd.AddGroup(&cobra.Group{ID: "primary", Title: "Commands:"})
	rootCmd.PersistentFlags().CountVarP(&flagVerbosity, "verbose", "v", "detailed phase output (-v); debug logging to file (-vv)")
	rootCmd.PersistentFlags().StringVarP(&flagWorkingDir, "working-dir", "d", "", "set working directory")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "suppress all output except errors")
}
