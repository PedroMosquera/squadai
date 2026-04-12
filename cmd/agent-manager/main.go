package main

import (
	"fmt"
	"os"

	"github.com/PedroMosquera/agent-manager-pro/internal/app"
)

// version is set via ldflags at build time.
var version = "dev"

func main() {
	app.Version = version

	if err := app.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
