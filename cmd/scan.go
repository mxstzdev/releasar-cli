package cmd

import (
	"github.com/mxstzdev/releasar-cli/internal/tasks"
	"github.com/spf13/cobra"
)

var flagScanSource string

func init() {
	scanCmd.Flags().StringVarP(&flagScanSource, "source", "s", ".", "directory to scan")
	rootCmd.AddCommand(scanCmd)
}

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan the working tree for secrets",
	Long:  "Scans source files for secrets using built-in gitleaks rules. Exits non-zero if any are found — suitable for use as a pre-commit hook.",
	RunE:  runScan,
}

func runScan(cmd *cobra.Command, _ []string) error {
	return tasks.RunSecretScan(flagScanSource, flagVerbosity > 0)
}
