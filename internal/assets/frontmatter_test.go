package assets

import (
	"io/fs"
	"path"
	"strings"
	"testing"
)

// Pi and other harnesses parse frontmatter with a strict YAML parser, which
// rejects a plain scalar containing ": " ("Nested mappings are not allowed in
// compact mappings") and truncates one at " #". Such a skill fails to load.
func TestEmbeddedAssets_FrontmatterIsStrictYAML(t *testing.T) {
	err := fs.WalkDir(FS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return err
		}
		data, err := fs.ReadFile(FS, p)
		if err != nil {
			return err
		}
		fm, ok := frontmatter(string(data))
		if !ok {
			return nil
		}
		for _, line := range strings.Split(fm, "\n") {
			key, value, found := strings.Cut(line, ":")
			if !found || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "#") {
				continue
			}
			if problem := plainScalarProblem(strings.TrimSpace(value)); problem != "" {
				t.Errorf("%s: frontmatter %q %s", p, key, problem)
			}
		}
		if path.Base(p) == "SKILL.md" && !strings.Contains("\n"+fm, "\nname: "+path.Base(path.Dir(p))+"\n") {
			t.Errorf("%s: frontmatter name must equal the directory name %q", p, path.Base(path.Dir(p)))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking embedded assets: %v", err)
	}
}

func frontmatter(s string) (string, bool) {
	rest, ok := strings.CutPrefix(s, "---\n")
	if !ok {
		return "", false
	}
	fm, _, ok := strings.Cut(rest, "\n---")
	return fm + "\n", ok
}

func plainScalarProblem(v string) string {
	if v == "" || strings.HasPrefix(v, `"`) || strings.HasPrefix(v, "'") || v == "|" || v == ">" {
		return ""
	}
	switch {
	case strings.Contains(v, ": "):
		return "is an unquoted value containing \": \""
	case strings.Contains(v, " #"):
		return "is an unquoted value containing \" #\""
	case strings.ContainsAny(v[:1], "[{&*!|>%@`"):
		return "is an unquoted value starting with a YAML indicator"
	}
	return ""
}
