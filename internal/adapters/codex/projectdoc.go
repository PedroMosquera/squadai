package codex

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DefaultProjectDocMaxBytes is Codex's built-in project_doc_max_bytes: the
// byte budget shared by the whole AGENTS.md chain, past which Codex truncates.
const DefaultProjectDocMaxBytes = 32768

// ProjectDocMaxBytes returns the project_doc_max_bytes Codex applies for
// projectDir: the top-level key from <projectDir>/.codex/config.toml, else from
// userConfigPath, else DefaultProjectDocMaxBytes. Missing files and malformed
// values fall through to the next source. Profile tables are not consulted.
func ProjectDocMaxBytes(userConfigPath, projectDir string) int {
	for _, path := range []string{filepath.Join(projectDir, ".codex", "config.toml"), userConfigPath} {
		if v, ok := readTopLevelInt(path, "project_doc_max_bytes"); ok {
			return v
		}
	}
	return DefaultProjectDocMaxBytes
}

// readTopLevelInt line-scans a TOML file for a non-negative integer key before
// the first table header. It is not a TOML parser; anything unusual is a miss.
func readTopLevelInt(path, key string) (int, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			return 0, false
		}
		name, value, found := strings.Cut(line, "=")
		if !found || strings.TrimSpace(name) != key {
			continue
		}
		value, _, _ = strings.Cut(value, "#")
		n, err := strconv.Atoi(strings.ReplaceAll(strings.TrimSpace(value), "_", ""))
		if err != nil || n < 0 {
			return 0, false
		}
		return n, true
	}
	return 0, false
}
