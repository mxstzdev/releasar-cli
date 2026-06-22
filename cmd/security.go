package cmd

import (
	"github.com/mxstzdev/releasar-cli/internal/tasks"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(auditCmd)
}

var auditCmd = &cobra.Command{
	Use:     "audit",
	Short:   "Run an audit to detect security vulnerabilities in your project",
	Long:    "Scans source files for secrets using built-in gitleaks rules. Exits non-zero if any are found — suitable for use as a pre-commit hook.",
	GroupID: "primary",
	RunE:    runAudit,
}

func runAudit(_ *cobra.Command, _ []string) error {
	source := flagWorkingDir
	if source == "" {
		source = "."
	}
	return tasks.RunSecretScan(source, flagVerbosity > 0)
}
