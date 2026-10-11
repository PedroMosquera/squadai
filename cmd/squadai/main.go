package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/PedroMosquera/squadai/internal/app"
	"github.com/PedroMosquera/squadai/internal/exitcode"
)

var version = "dev"

var pseudoVersion = regexp.MustCompile(`\d{14}-[0-9a-f]{12}$`)

// resolveVersion prefers the goreleaser ldflags value and falls back to the
// module version Go records for `go install pkg@vX`, which the action uses.
// Local builds get a VCS pseudo-version or +dirty from the go tool; those
// stay "dev" so they are never mistaken for a release.
func resolveVersion(ldflags string, info *debug.BuildInfo) string {
	if ldflags != "dev" || info == nil {
		return ldflags
	}
	v := info.Main.Version
	if v == "" || v == "(devel)" || strings.Contains(v, "+") || pseudoVersion.MatchString(v) {
		return ldflags
	}
	return strings.TrimPrefix(v, "v")
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
