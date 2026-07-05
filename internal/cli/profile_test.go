package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// chdirTemp switches the working directory to a fresh temp dir for the test.
func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	return dir
}

func writeDefaultProject(t *testing.T, dir string) {
	t.Helper()
	if err := config.WriteJSON(config.ProjectConfigPath(dir), domain.DefaultProjectConfig()); err != nil {
		t.Fatalf("write project.json: %v", err)
	}
}

func TestRunProfile_Help(t *testing.T) {
	var buf bytes.Buffer
	if err := RunProfile([]string{"--help"}, &buf); err != nil {
		t.Fatalf("help should not error: %v", err)
	}
	for _, want := range []string{"Usage: squadai profile", "default_profile", "not enforced yet"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("help missing %q", want)
		}
	}
}

func TestRunProfile_List_ShowsActiveAndAll(t *testing.T) {
	dir := chdirTemp(t)
	writeDefaultProject(t, dir)

	var buf bytes.Buffer
	if err := RunProfile(nil, &buf); err != nil {
		t.Fatalf("profile list: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Active profile: default") {
		t.Errorf("expected active default profile, got:\n%s", out)
	}
	for _, name := range []string{"cheap", "debug", "review"} {
		if !strings.Contains(out, name) {
			t.Errorf("profile list missing %q:\n%s", name, out)
		}
	}
	if !strings.Contains(out, "6000") {
		t.Errorf("expected cheap profile cap 6000 in listing:\n%s", out)
	}
}

func TestRunProfile_JSON(t *testing.T) {
	dir := chdirTemp(t)
	writeDefaultProject(t, dir)

	var buf bytes.Buffer
	if err := RunProfile([]string{"--json"}, &buf); err != nil {
		t.Fatalf("profile --json: %v", err)
	}
	var out profileJSON
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if out.Active != "default" {
		t.Errorf("active = %q, want default", out.Active)
	}
	if _, ok := out.Profiles["cheap"]; !ok {
		t.Error("profiles JSON missing cheap")
	}
}

func TestRunProfile_Set_PersistsDefaultProfile(t *testing.T) {
	dir := chdirTemp(t)
	writeDefaultProject(t, dir)

	var buf bytes.Buffer
	if err := RunProfile([]string{"cheap"}, &buf); err != nil {
		t.Fatalf("profile cheap: %v", err)
	}
	if !strings.Contains(buf.String(), "squadai apply") {
		t.Errorf("expected apply hint, got:\n%s", buf.String())
	}

	proj, err := config.LoadProject(dir)
	if err != nil {
		t.Fatalf("reload project: %v", err)
	}
	if proj.Context.DefaultProfile != "cheap" {
		t.Errorf("persisted default_profile = %q, want cheap", proj.Context.DefaultProfile)
	}
	// Profile definitions untouched.
	if len(proj.Context.Profiles) == 0 {
		t.Error("profiles must survive the save")
	}
}

func TestRunProfile_Set_UnknownName_ErrorsWithAvailable(t *testing.T) {
	dir := chdirTemp(t)
	writeDefaultProject(t, dir)

	var buf bytes.Buffer
	err := RunProfile([]string{"bogus"}, &buf)
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
	if !strings.Contains(err.Error(), "unknown context profile") || !strings.Contains(err.Error(), "cheap") {
		t.Errorf("error should name the profile and list available ones, got: %v", err)
	}
}

func TestRunProfile_Set_NoProject_Errors(t *testing.T) {
	chdirTemp(t)
	var buf bytes.Buffer
	err := RunProfile([]string{"cheap"}, &buf)
	if err == nil {
		t.Fatal("expected error without project.json")
	}
}

func TestRunProfile_UnknownFlag(t *testing.T) {
	var buf bytes.Buffer
	if err := RunProfile([]string{"--bogus"}, &buf); err == nil {
		t.Fatal("expected error for unknown flag")
	}
}
