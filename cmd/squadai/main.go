package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/PedroMosquera/squadai/internal/app"
	"github.com/PedroMosquera/squadai/internal/exitcode"
)

var version = "dev"

// resolveVersion prefers the goreleaser ldflags value and falls back to the
// module version Go records for `go install pkg@vX`, which the action uses.
func resolveVersion(ldflags string, info *debug.BuildInfo) string {
	if ldflags != "dev" || info == nil {
		return ldflags
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return strings.TrimPrefix(v, "v")
	}
	return ldflags
}

func main() {
	info, _ := debug.ReadBuildInfo()
	app.Version = resolveVersion(version, info)
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
