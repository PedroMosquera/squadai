package scan

import "testing"

// Pi package entries are strings or {"source": ...} objects
// (https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/packages.md);
// OpenCode plugin entries are npm specs or [spec, options] tuples
// (https://opencode.ai/docs/plugins/). Local paths load files already in the
// checkout and are not package fetches.
const handAddedPiPackages = `{
  "packages": [
    "npm:@vetted-org/tools@1.2.3",
    "npm:loose-tools",
    {"source": "git:github.com/someone/pi-ext"},
    "./local-pkg"
  ]
}`

const handAddedOpenCodePlugins = `{
  "plugin": [
    "@vetted-org/oc-tools@2.0.0",
    "stray-plugin@^1.0.0",
    ["tuple-plugin", {"verbose": true}],
    "./plugins/mine.ts"
  ]
}`

func TestScan_FlagsHandAddedPackagesThatAreUnpinnedOrOutsideAllowList(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".pi/settings.json", handAddedPiPackages)
	writeFile(t, dir, "opencode.json", handAddedOpenCodePlugins)
	writeFile(t, dir, ".squadai/policy.json", `{"version": 1, "mode": "team", "allowed": {"plugin_sources": ["npm:@vetted-org/*", "github.com/vetted-org/*"]}}`)

	r, err := Scan(dir, AllAdapters(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	assertHits(t, r, []hit{
		{"SCAN-003", ".pi/settings.json", 4},
		{"SCAN-007", ".pi/settings.json", 4},
		{"SCAN-003", ".pi/settings.json", 5},
		{"SCAN-007", ".pi/settings.json", 5},
		{"SCAN-003", "opencode.json", 4},
		{"SCAN-007", "opencode.json", 4},
		{"SCAN-003", "opencode.json", 5},
		{"SCAN-007", "opencode.json", 5},
	})
}

func TestScan_UnpinnedPackageIsFlaggedWithoutAPolicy(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".pi/settings.json", handAddedPiPackages)
	writeFile(t, dir, "opencode.json", handAddedOpenCodePlugins)

	r, err := Scan(dir, AllAdapters(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	assertHits(t, r, []hit{
		{"SCAN-003", ".pi/settings.json", 4},
		{"SCAN-003", ".pi/settings.json", 5},
		{"SCAN-003", "opencode.json", 4},
		{"SCAN-003", "opencode.json", 5},
	})
}
