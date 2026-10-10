package assets

import (
	"io/fs"
	"strings"
	"testing"
)

// Shipped content lands in users' repos under their name, so it must not use
// em or en dashes.
func TestEmbeddedAssets_NoEmOrEnDashes(t *testing.T) {
	err := fs.WalkDir(FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(FS, path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.ContainsAny(line, "\u2014\u2013") {
				t.Errorf("%s:%d contains an em or en dash: %s", path, i+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking embedded assets: %v", err)
	}
}
