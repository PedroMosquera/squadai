package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// modelsDoctor builds a Doctor over temp dirs with a fixed clock.
func modelsDoctor(t *testing.T, homeDir, projectDir string, now time.Time) *Doctor {
	t.Helper()
	return NewWithDeps(homeDir, projectDir, nil, nil, fakeLooker{}, fakeRunner{}).
		WithClock(func() time.Time { return now })
}

// writeModelsOverride writes a models.json override under dir/.squadai.
func writeModelsOverride(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".squadai"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squadai", "models.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckModelsCatalogFreshness_FakeClock(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	// Pin the catalog date via a user override so the test is independent
	// of the embedded catalog's updated field.
	writeModelsOverride(t, home, `{"schema_version": 1, "updated": "2026-01-01"}`)
	updated := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		now  time.Time
		want CheckStatus
	}{
		{"fresh at 30 days", updated.AddDate(0, 0, 30), CheckPass},
		{"fresh at 119 days", updated.AddDate(0, 0, 119), CheckPass},
		{"stale at 121 days", updated.AddDate(0, 0, 121), CheckWarn},
		{"stale at 2 years", updated.AddDate(2, 0, 0), CheckWarn},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := modelsDoctor(t, home, project, tc.now)
			r := d.checkModelsCatalogFreshness()
			if r.Status != tc.want {
				t.Errorf("status = %s, want %s (msg: %s)", r.Status, tc.want, r.Message)
			}
			if tc.want == CheckWarn && !strings.Contains(r.FixHint, "squadai models update") {
				t.Errorf("stale warning FixHint = %q, want 'squadai models update'", r.FixHint)
			}
		})
	}
}

func TestCheckModelsCatalogFreshness_InvalidOverrideWarns(t *testing.T) {
	home := t.TempDir()
	writeModelsOverride(t, home, `{"schema_version": 99}`)
	d := modelsDoctor(t, home, t.TempDir(), time.Now())
	r := d.checkModelsCatalogFreshness()
	if r.Status != CheckWarn {
		t.Errorf("invalid override status = %s, want warn", r.Status)
	}
}

func TestCheckModelsKnown(t *testing.T) {
	cases := []struct {
		name        string
		projectJSON string
		want        CheckStatus
		wantMsg     string
	}{
		{
			name:        "no project config skips",
			projectJSON: "",
			want:        CheckSkip,
		},
		{
			name:        "no concrete overrides passes",
			projectJSON: `{"version": 1}`,
			want:        CheckPass,
		},
		{
			name: "known concrete override passes",
			projectJSON: `{"version": 1, "models": {"profiles": {
				"premium": {"tier": "premium", "adapters": {"claude-code": "claude-fable-5"}}
			}}}`,
			want: CheckPass,
		},
		{
			name: "provider-qualified known model passes",
			projectJSON: `{"version": 1, "models": {"profiles": {
				"premium": {"tier": "premium", "adapters": {"opencode": "anthropic/claude-sonnet-4-6"}}
			}}}`,
			want: CheckPass,
		},
		{
			name: "unknown concrete override warns",
			projectJSON: `{"version": 1, "models": {"profiles": {
				"premium": {"tier": "premium", "adapters": {"claude-code": "claude-imaginary-99"}}
			}}}`,
			want:    CheckWarn,
			wantMsg: "claude-imaginary-99",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			project := t.TempDir()
			if tc.projectJSON != "" {
				if err := os.MkdirAll(filepath.Join(project, ".squadai"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(project, ".squadai", "project.json"), []byte(tc.projectJSON), 0644); err != nil {
					t.Fatal(err)
				}
			}
			d := modelsDoctor(t, t.TempDir(), project, time.Now())
			r := d.checkModelsKnown()
			if r.Status != tc.want {
				t.Errorf("status = %s, want %s (msg: %s)", r.Status, tc.want, r.Message)
			}
			if tc.wantMsg != "" && !strings.Contains(r.Message, tc.wantMsg) {
				t.Errorf("message %q missing %q", r.Message, tc.wantMsg)
			}
		})
	}
}
