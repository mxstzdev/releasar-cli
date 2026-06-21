package main

import (
	_ "embed"

	"github.com/mxstzdev/releasar-cli/cmd"
)

// version is injected at build time via -ldflags "-X main.version=X.Y.Z".
var version = "dev"

//go:embed LICENSE
var license string

//go:embed LICENSES
var licenses string

func main() {
	cmd.SetLicenseContent(license, licenses)
	cmd.Execute(version)
}
