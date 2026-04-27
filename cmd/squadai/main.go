package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/PedroMosquera/squadai/internal/app"
	"github.com/PedroMosquera/squadai/internal/exitcode"
)

var version = "dev"

func main() {
	app.Version = version
	if err := app.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		var ae *exitcode.AppError
		if errors.As(err, &ae) {
			fmt.Fprintf(os.Stderr, "Error %s: %s\n", ae.ErrCode, ae.Msg)
			if ae.Hint != "" {
				fmt.Fprintf(os.Stderr, "  → %s\n", ae.Hint)
			}
			os.Exit(ae.Code)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitcode.Unexpected)
	}
}
