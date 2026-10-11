package plugins

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/adapters/pi"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/managed"
)

// Pi reads "packages" from .pi/settings.json as npm:, git:, URL or local
// path sources, as strings or {"source": ...} objects
// (https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/packages.md).
// OpenCode reads "plugin" from opencode.json as npm package names
// (https://opencode.ai/docs/plugins/); its loader parses them with
// npm-package-arg, so "name@1.2.3" pins a version and "./x.ts" is a local file.

func packagePlugins(toolsSpec string) map[string]domain.PluginDef {
	return map[string]domain.PluginDef{
		"tools": {
			Enabled:         true,
			SupportedAgents: []string{"pi", "opencode"},
			InstallMethod:   "package",
			Package:         toolsSpec,
		},
		"gitpkg": {
			Enabled:         true,
			SupportedAgents: []string{"pi"},
			InstallMethod:   "package",
			Package:         "git:github.com/example-org/pi-git@v1.0.0",
		},
	}
}

func seedPackageFiles(t *testing.T, project string) (piPath, ocPath string) {
	t.Helper()
	piPath = filepath.Join(project, ".pi", "settings.json")
	ocPath = filepath.Join(project, "opencode.json")
	writeTestJSON(t, piPath, map[string]interface{}{
		"theme": "dark",
		"packages": []interface{}{
			"npm:user-a@1.0.0",
			map[string]interface{}{"source": "git:github.com/user/b@v2", "skills": []interface{}{}},
			"./local-pkg",
		},
	})
	writeTestJSON(t, ocPath, map[string]interface{}{
		"mcp":    map[string]interface{}{"docs": map[string]interface{}{"type": "remote", "url": "https://example.invalid/mcp"}},
		"plugin": []interface{}{"user-plugin@0.1.0", "./plugins/mine.ts"},
	})
	return piPath, ocPath
}

func applyPackages(t *testing.T, plugins map[string]domain.PluginDef, home, project string) {
	t.Helper()
	inst := New(plugins, &domain.MergedConfig{})
	planAndApply(t, inst, pi.New(), home, project)
	planAndApply(t, inst, opencode.New(), home, project)
}

func TestApply_PackageDeclarationsGoToPiAndOpenCodeAfterUserEntries(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	piPath, ocPath := seedPackageFiles(t, project)

	applyPackages(t, packagePlugins("npm:@example-org/pi-tools@1.2.3"), home, project)

	piDoc := readTestJSONHelper(t, piPath)
	wantPi := []interface{}{
		"npm:user-a@1.0.0",
		map[string]interface{}{"source": "git:github.com/user/b@v2", "skills": []interface{}{}},
		"./local-pkg",
		"git:github.com/example-org/pi-git@v1.0.0",
		"npm:@example-org/pi-tools@1.2.3",
	}
	if !reflect.DeepEqual(piDoc["packages"], wantPi) {
		t.Errorf("pi packages = %v\nwant %v", piDoc["packages"], wantPi)
	}
	if piDoc["theme"] != "dark" {
		t.Errorf("pi user key theme was not preserved: %v", piDoc)
	}

	ocDoc := readTestJSONHelper(t, ocPath)
	wantOC := []interface{}{"user-plugin@0.1.0", "./plugins/mine.ts", "@example-org/pi-tools@1.2.3"}
	if !reflect.DeepEqual(ocDoc["plugin"], wantOC) {
		t.Errorf("opencode plugin = %v\nwant %v", ocDoc["plugin"], wantOC)
	}
	if _, ok := ocDoc["mcp"].(map[string]interface{})["docs"]; !ok {
		t.Errorf("opencode mcp servers must be kept: %v", ocDoc)
	}
}

func TestApply_ChangedPackageVersionIsUpdatedInPlace(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	_, ocPath := seedPackageFiles(t, project)
	applyPackages(t, packagePlugins("npm:@example-org/pi-tools@1.2.3"), home, project)

	doc := readTestJSONHelper(t, ocPath)
	doc["plugin"] = append(doc["plugin"].([]interface{}), "added-later@2.0.0")
	writeTestJSON(t, ocPath, doc)

	applyPackages(t, packagePlugins("npm:@example-org/pi-tools@1.3.0"), home, project)

	want := []interface{}{"user-plugin@0.1.0", "./plugins/mine.ts", "@example-org/pi-tools@1.3.0", "added-later@2.0.0"}
	if got := readTestJSONHelper(t, ocPath)["plugin"]; !reflect.DeepEqual(got, want) {
		t.Errorf("opencode plugin = %v\nwant %v", got, want)
	}
	names, _, err := managed.ReadManagedEntries(project, "opencode.json", "plugin")
	if err != nil || !reflect.DeepEqual(names, []string{"@example-org/pi-tools@1.3.0"}) {
		t.Errorf("sidecar should own only the new version, got %v (%v)", names, err)
	}
}

func TestApply_UndeclaredPackageIsRemovedAndUserEntriesStay(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	piPath, ocPath := seedPackageFiles(t, project)
	applyPackages(t, packagePlugins("npm:@example-org/pi-tools@1.2.3"), home, project)

	applyPackages(t, map[string]domain.PluginDef{}, home, project)

	wantPi := []interface{}{
		"npm:user-a@1.0.0",
		map[string]interface{}{"source": "git:github.com/user/b@v2", "skills": []interface{}{}},
		"./local-pkg",
	}
	if got := readTestJSONHelper(t, piPath)["packages"]; !reflect.DeepEqual(got, wantPi) {
		t.Errorf("pi packages = %v\nwant only the user's %v", got, wantPi)
	}
	wantOC := []interface{}{"user-plugin@0.1.0", "./plugins/mine.ts"}
	if got := readTestJSONHelper(t, ocPath)["plugin"]; !reflect.DeepEqual(got, wantOC) {
		t.Errorf("opencode plugin = %v\nwant only the user's %v", got, wantOC)
	}
	if keys, _ := managed.ReadManagedKeys(project, filepath.Join(".pi", "settings.json")); len(keys) != 0 {
		t.Errorf("sidecar should own nothing in .pi/settings.json after removal, got %v", keys)
	}
}

func TestApply_RemovingTheOnlyPackageDropsTheKeySquadaiAdded(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	only := map[string]domain.PluginDef{"tools": packagePlugins("npm:@example-org/pi-tools@1.2.3")["tools"]}
	applyPackages(t, only, home, project)
	piPath := filepath.Join(project, ".pi", "settings.json")
	if got := readTestJSONHelper(t, piPath)["packages"]; !reflect.DeepEqual(got, []interface{}{"npm:@example-org/pi-tools@1.2.3"}) {
		t.Fatalf("pi packages before removal = %v", got)
	}

	applyPackages(t, map[string]domain.PluginDef{}, home, project)

	if doc := readTestJSONHelper(t, piPath); doc["packages"] != nil {
		t.Errorf("packages should be gone once squadai's only entry is removed: %v", doc)
	}
}

func TestApply_UserPackageAtAnotherVersionIsAConflict(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	ocPath := filepath.Join(project, "opencode.json")
	writeTestJSON(t, ocPath, map[string]interface{}{"plugin": []interface{}{"@example-org/pi-tools@0.9.0"}})

	inst := New(packagePlugins("npm:@example-org/pi-tools@1.2.3"), &domain.MergedConfig{})
	actions, err := inst.Plan(opencode.New(), home, project)
	if err != nil || len(actions) != 1 {
		t.Fatalf("plan: %v %+v", err, actions)
	}
	var conflict *domain.ConflictError
	if err := inst.Apply(actions[0]); !errors.As(err, &conflict) {
		t.Fatalf("want a conflict on the user's entry, got %v", err)
	}
	if got := readTestJSONHelper(t, ocPath)["plugin"]; !reflect.DeepEqual(got, []interface{}{"@example-org/pi-tools@0.9.0"}) {
		t.Errorf("the user's entry must stay, got %v", got)
	}
}

func TestPlan_PackageDeclarationsAreCurrentAfterApply(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	inst := New(packagePlugins("npm:@example-org/pi-tools@1.2.3"), &domain.MergedConfig{})
	for _, adapter := range []domain.Adapter{pi.New(), opencode.New()} {
		applied := planAndApply(t, inst, adapter, home, project)
		if len(applied) != 1 {
			t.Fatalf("%s: want one package action, got %+v", adapter.ID(), applied)
		}
		actions, err := inst.Plan(adapter, home, project)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range actions {
			if a.Action != domain.ActionSkip {
				t.Errorf("%s: a second plan should skip, got %s %s", adapter.ID(), a.Action, a.TargetPath)
			}
		}
		results, err := inst.Verify(adapter, home, project)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) == 0 {
			t.Fatalf("%s: verify should report the declared packages", adapter.ID())
		}
		for _, r := range results {
			if !r.Passed {
				t.Errorf("%s: verify %s failed: %s", adapter.ID(), r.Check, r.Message)
			}
		}
	}
}
