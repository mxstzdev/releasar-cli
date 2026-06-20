package main

import (
	"github.com/mxstzdev/releasar-cli/cmd"
)

// version is injected at build time via -ldflags "-X main.version=X.Y.Z".
var version = "dev"

func main() {
	cmd.Execute(version)
}
