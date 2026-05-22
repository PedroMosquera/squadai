package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/squadrefine"
)

func hookTestSetup(t *testing.T, projectDir string) (*bytes.Buffer, func()) {
	t.Helper()
	buf := &bytes.Buffer{}
	prevOut := hookStdout
	prevWd := hookGetwd
	hookStdout = buf
	hookGetwd = func() (string, error) { return projectDir, nil }
	return buf, func() {
		hookStdout = prevOut
		hookGetwd = prevWd
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestSquadNudge_HonorsKillSwitch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example\n")

	buf, restore := hookTestSetup(t, dir)
	defer restore()

	t.Setenv("SQUADAI_NO_NUDGE", "1")

	if err := RunHookCommand([]string{"squad-nudge"}); err != nil {
		t.Fatalf("hook returned error: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no output under SQUADAI_NO_NUDGE, got: %q", buf.String())
	}
	if _, err := os.Stat(squadrefine.FilePath(dir)); !os.IsNotExist(err) {
		t.Fatalf("state file was created under kill switch: err=%v", err)
	}
}

func TestSquadNudge_NoStateNoSignals_FiresOnceNoStateCreated(t *testing.T) {
	dir := t.TempDir()

	buf, restore := hookTestSetup(t, dir)
	defer restore()
	t.Setenv("SQUADAI_NO_NUDGE", "")

	if err := RunHookCommand([]string{"squad-nudge"}); err != nil {
		t.Fatalf("hook returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "this squad hasn't been tuned") {
		t.Fatalf("expected never-refined nudge, got: %q", out)
	}
	if _, err := os.Stat(squadrefine.FilePath(dir)); !os.IsNotExist(err) {
		t.Fatalf("hook created state file (it must not); err=%v", err)
	}
}

func TestSquadNudge_NeverRefinedWithGoMod(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n")

	buf, restore := hookTestSetup(t, dir)
	defer restore()
	t.Setenv("SQUADAI_NO_NUDGE", "")

	if err := RunHookCommand([]string{"squad-nudge"}); err != nil {
		t.Fatalf("hook returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "this squad hasn't been tuned") {
		t.Fatalf("expected never-refined wording, got: %q", out)
	}
	if !strings.Contains(out, "/squadai-init") {
		t.Fatalf("nudge missing /squadai-init reference: %q", out)
	}
	if !strings.Contains(out, "tokens") {
		t.Fatalf("nudge must disclose token cost: %q", out)
	}
}

func TestSquadNudge_DriftWhenManifestMismatches(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module new\n")

	st := &squadrefine.State{Version: squadrefine.CurrentVersion}
	if err := squadrefine.Save(dir, st); err != nil {
		t.Fatalf("seed state (pre): %v", err)
	}
	st.SignalHashes = sampleDriftSignals(dir)
	st.SignalHashes["go.mod"] = squadrefine.HashContent([]byte("module old\n"))
	if err := squadrefine.Save(dir, st); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	buf, restore := hookTestSetup(t, dir)
	defer restore()
	t.Setenv("SQUADAI_NO_NUDGE", "")

	if err := RunHookCommand([]string{"squad-nudge"}); err != nil {
		t.Fatalf("hook returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "codebase has changed") {
		t.Fatalf("expected drift wording, got: %q", out)
	}
	if !strings.Contains(out, "go.mod") {
		t.Fatalf("expected drift reason to include go.mod: %q", out)
	}

	loaded, _, err := squadrefine.Load(dir)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if loaded.Nudges.UnactionedCount != 1 {
		t.Fatalf("expected unactioned_count=1, got %d", loaded.Nudges.UnactionedCount)
	}
	if loaded.Nudges.LastSignature == "" {
		t.Fatalf("expected last_signature to be recorded")
	}
}

func TestSquadNudge_FreshIsSilent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module fresh\n")

	st := &squadrefine.State{Version: squadrefine.CurrentVersion}
	if err := squadrefine.Save(dir, st); err != nil {
		t.Fatalf("seed state (pre): %v", err)
	}
	st.SignalHashes = sampleDriftSignals(dir)
	if err := squadrefine.Save(dir, st); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	buf, restore := hookTestSetup(t, dir)
	defer restore()
	t.Setenv("SQUADAI_NO_NUDGE", "")

	if err := RunHookCommand([]string{"squad-nudge"}); err != nil {
		t.Fatalf("hook returned error: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected silent fresh path, got output: %q", buf.String())
	}
	loaded, _, err := squadrefine.Load(dir)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if loaded.Nudges.UnactionedCount != 0 {
		t.Fatalf("fresh path must not bump counter, got %d", loaded.Nudges.UnactionedCount)
	}
}

func TestSquadNudge_ThrottlesAfterThreshold(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module new\n")

	st := &squadrefine.State{Version: squadrefine.CurrentVersion}
	if err := squadrefine.Save(dir, st); err != nil {
		t.Fatalf("seed state (pre): %v", err)
	}
	st.SignalHashes = sampleDriftSignals(dir)
	st.SignalHashes["go.mod"] = squadrefine.HashContent([]byte("module old\n"))
	if err := squadrefine.Save(dir, st); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	for i := 0; i < squadrefine.NudgeThrottleAt; i++ {
		buf, restore := hookTestSetup(t, dir)
		t.Setenv("SQUADAI_NO_NUDGE", "")
		if err := RunHookCommand([]string{"squad-nudge"}); err != nil {
			restore()
			t.Fatalf("hook iter %d: %v", i, err)
		}
		if buf.Len() == 0 {
			restore()
			t.Fatalf("iter %d: expected nudge before throttle, got silence", i)
		}
		restore()
	}

	buf, restore := hookTestSetup(t, dir)
	defer restore()
	t.Setenv("SQUADAI_NO_NUDGE", "")
	if err := RunHookCommand([]string{"squad-nudge"}); err != nil {
		t.Fatalf("post-throttle hook: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected throttled silence, got: %q", buf.String())
	}

	loaded, _, err := squadrefine.Load(dir)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if !loaded.Nudges.Throttled {
		t.Fatalf("expected throttled=true, got %+v", loaded.Nudges)
	}
}

func TestSquadNudge_ResetsThrottleOnSignatureChange(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module new\n")

	st := &squadrefine.State{Version: squadrefine.CurrentVersion}
	if err := squadrefine.Save(dir, st); err != nil {
		t.Fatalf("seed state (pre): %v", err)
	}
	st.SignalHashes = sampleDriftSignals(dir)
	st.SignalHashes["go.mod"] = squadrefine.HashContent([]byte("module old\n"))
	st.Nudges.UnactionedCount = squadrefine.NudgeThrottleAt + 5
	st.Nudges.Throttled = true
	st.Nudges.LastSignature = "some-other-prior-signature"
	if err := squadrefine.Save(dir, st); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	buf, restore := hookTestSetup(t, dir)
	defer restore()
	t.Setenv("SQUADAI_NO_NUDGE", "")

	if err := RunHookCommand([]string{"squad-nudge"}); err != nil {
		t.Fatalf("hook: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatalf("expected nudge after signature change, got silence")
	}

	loaded, _, err := squadrefine.Load(dir)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if loaded.Nudges.UnactionedCount != 1 {
		t.Fatalf("expected counter reset to 1 on signature change, got %d", loaded.Nudges.UnactionedCount)
	}
	if loaded.Nudges.Throttled {
		t.Fatalf("expected throttled=false after signature change, got true")
	}
}
