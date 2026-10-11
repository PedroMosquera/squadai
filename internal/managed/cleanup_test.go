package managed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInspectStale_RemovesOwnedArrayEntriesAndKeepsUserEntriesInOrder(t *testing.T) {
	project := t.TempDir()
	path := filepath.Join(project, "opencode.json")
	content := `{"mcp": {"docs": {"type": "remote"}}, "plugin": ["first@1.0.0", "squadai-owned@1.2.3", ["last@2.0.0", {}]]}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteManagedEntries(project, "opencode.json", "plugin", []string{"squadai-owned@1.2.3"}); err != nil {
		t.Fatal(err)
	}

	stale, _, err := InspectStale(project, path)
	if err != nil {
		t.Fatal(err)
	}
	if !stale.Found {
		t.Fatal("the owned array entry should be recognized as squadai content")
	}
	var doc map[string]any
	if err := json.Unmarshal(stale.Stripped, &doc); err != nil {
		t.Fatal(err)
	}
	want := []any{"first@1.0.0", []any{"last@2.0.0", map[string]any{}}}
	if !reflect.DeepEqual(doc["plugin"], want) {
		t.Errorf("plugin = %v, want %v", doc["plugin"], want)
	}
	if doc["mcp"] == nil {
		t.Errorf("other keys must stay: %v", doc)
	}
}
