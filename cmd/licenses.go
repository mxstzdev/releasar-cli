package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	embeddedLicense  string
	embeddedLicenses string
)

// SetLicenseContent receives the embedded license texts from main.
func SetLicenseContent(license, licenses string) {
	embeddedLicense = license
	embeddedLicenses = licenses
}

func init() {
	rootCmd.AddCommand(licensesCmd)
}

var licensesCmd = &cobra.Command{
	Use:   "licenses",
	Short: "Display the license & all bundled third-party licenses",
	RunE:  runLicenses,
}

func runLicenses(_ *cobra.Command, _ []string) error {
	fmt.Println(embeddedLicense)
	fmt.Println("================================================================================")
	fmt.Println("Third-Party Licenses")
	fmt.Println("================================================================================")
	fmt.Println()
	fmt.Println(embeddedLicenses)
	return nil
}
