package cmd

import (
	"github.com/mxstzdev/releasar-cli/internal/tasks"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(scanCmd)
}

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan the working tree for secret leaks",
	Long:  "Scans source files for secrets using built-in gitleaks rules. Exits non-zero if any are found — suitable for use as a pre-commit hook.",
	RunE:  runScan,
}

func runScan(cmd *cobra.Command, _ []string) error {
	source := flagWorkingDir
	if source == "" {
		source = "."
	}
	return tasks.RunSecretScan(source, flagVerbosity > 0)
}
