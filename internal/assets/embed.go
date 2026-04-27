package assets

import (
	"embed"
	"strings"
)

//go:embed all:memory all:standards all:copilot all:skills all:teams all:mcp all:workflows all:commands all:agents
var FS embed.FS

// MustRead returns the content of an embedded file or panics.
// Trailing whitespace is trimmed to match Go string literal conventions.
func MustRead(path string) string {
	data, err := FS.ReadFile(path)
	if err != nil {
		panic("assets: " + err.Error())
	}
	return strings.TrimRight(string(data), "\n")
}

// Read returns the content of an embedded file.
// Trailing whitespace is trimmed to match Go string literal conventions.
func Read(path string) (string, error) {
	data, err := FS.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\n"), nil
}
