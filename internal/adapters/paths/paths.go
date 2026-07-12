// Package paths provides shared per-user config directory resolution for
// editor adapters, so OS-specific rules (notably the Windows APPDATA
// fallback) live in exactly one place.
package paths

import (
	"os"
	"path/filepath"
)

// UserConfigDir returns the per-user config directory for an Electron-style
// editor identified by appName, following the VS Code convention:
//
//   - windows: %APPDATA%\<appName>\User, falling back to
//     homeDir\AppData\Roaming\<appName>\User when APPDATA is unset
//   - linux:   homeDir/.config/<appName>/User
//   - all other platforms (darwin, ...): homeDir/Library/Application Support/<appName>/User
//
// goos is passed explicitly (callers use runtime.GOOS) so resolution stays
// testable across platforms. Adapters whose editor keeps its global config
// in a dot-directory instead (e.g. Cursor's ~/.cursor) should call this only
// for the Windows branch and keep their own non-Windows path.
func UserConfigDir(homeDir, goos, appName string) string {
	switch goos {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, appName, "User")
		}
		return filepath.Join(homeDir, "AppData", "Roaming", appName, "User")
	case "linux":
		return filepath.Join(homeDir, ".config", appName, "User")
	default:
		return filepath.Join(homeDir, "Library", "Application Support", appName, "User")
	}
}
