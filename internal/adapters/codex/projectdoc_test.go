package codex

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTOML(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProjectDocMaxBytes(t *testing.T) {
	tests := []struct {
		name    string
		user    string
		project string
		want    int
	}{
		{name: "no config files", want: DefaultProjectDocMaxBytes},
		{name: "user config sets key", user: "model = \"o3\"\nproject_doc_max_bytes = 65536\n", want: 65536},
		{name: "project config wins over user", user: "project_doc_max_bytes = 65536\n", project: "project_doc_max_bytes = 8192 # tighter\n", want: 8192},
		{name: "underscore separators", user: "project_doc_max_bytes = 65_536\n", want: 65536},
		{name: "key inside a table is ignored", user: "[profiles.big]\nproject_doc_max_bytes = 65536\n", want: DefaultProjectDocMaxBytes},
		{name: "malformed value falls through", user: "project_doc_max_bytes = 65536\n", project: "project_doc_max_bytes = \"lots\"\n", want: 65536},
		{name: "zero is honored", user: "project_doc_max_bytes = 0\n", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			project := t.TempDir()
			userPath := filepath.Join(home, ".codex", "config.toml")
			if tt.user != "" {
				writeTOML(t, userPath, tt.user)
			}
			if tt.project != "" {
				writeTOML(t, filepath.Join(project, ".codex", "config.toml"), tt.project)
			}
			if got := ProjectDocMaxBytes(userPath, project); got != tt.want {
				t.Errorf("ProjectDocMaxBytes() = %d, want %d", got, tt.want)
			}
		})
	}
}
