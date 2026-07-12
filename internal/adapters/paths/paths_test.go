package paths

import (
	"path/filepath"
	"testing"
)

func TestUserConfigDir(t *testing.T) {
	home := "/Users/test"

	tests := []struct {
		name    string
		goos    string
		appData string // value for APPDATA; "" means unset
		appName string
		want    string
	}{
		{
			name:    "windows with APPDATA set",
			goos:    "windows",
			appData: `C:\Users\test\AppData\Roaming`,
			appName: "Code",
			want:    filepath.Join(`C:\Users\test\AppData\Roaming`, "Code", "User"),
		},
		{
			name:    "windows falls back to homeDir when APPDATA unset",
			goos:    "windows",
			appName: "Code",
			want:    filepath.Join(home, "AppData", "Roaming", "Code", "User"),
		},
		{
			name:    "linux uses XDG-style .config",
			goos:    "linux",
			appName: "Code",
			want:    filepath.Join(home, ".config", "Code", "User"),
		},
		{
			name:    "darwin uses Library/Application Support",
			goos:    "darwin",
			appName: "Code",
			want:    filepath.Join(home, "Library", "Application Support", "Code", "User"),
		},
		{
			name:    "unknown GOOS falls back to darwin layout",
			goos:    "freebsd",
			appName: "Code",
			want:    filepath.Join(home, "Library", "Application Support", "Code", "User"),
		},
		{
			name:    "APPDATA ignored on non-windows",
			goos:    "linux",
			appData: `C:\Users\test\AppData\Roaming`,
			appName: "Windsurf",
			want:    filepath.Join(home, ".config", "Windsurf", "User"),
		},
		{
			name:    "app name is interpolated on windows",
			goos:    "windows",
			appData: `C:\AppData`,
			appName: "Cursor",
			want:    filepath.Join(`C:\AppData`, "Cursor", "User"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APPDATA", tt.appData)
			if got := UserConfigDir(home, tt.goos, tt.appName); got != tt.want {
				t.Errorf("UserConfigDir(%q, %q, %q) = %q, want %q",
					home, tt.goos, tt.appName, got, tt.want)
			}
		})
	}
}
