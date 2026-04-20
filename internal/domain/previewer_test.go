package domain

import "testing"

// fakePreviewer exists solely to assert Previewer is implementable outside the
// package. Compile-time check catches interface-signature drift.
type fakePreviewer struct{}

func (fakePreviewer) Preview(_ Adapter, _, _ string) ([]PreviewEntry, error) {
	return []PreviewEntry{{
		Component:  ComponentMCP,
		Action:     ActionCreate,
		TargetPath: "/tmp/.mcp.json",
		Diff:       "+++ b/.mcp.json\n",
	}}, nil
}

var _ Previewer = (*fakePreviewer)(nil)

func TestTruncate(t *testing.T) {
	cases := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"empty", "", 5, ""},
		{"shorter", "abc", 5, "abc"},
		{"exact", "abcde", 5, "abcde"},
		{"longer", "abcdef", 5, "abcd…"},
		{"n_zero", "abc", 0, ""},
		{"n_one", "abc", 1, "…"},
		{"unicode_longer", "héllo world", 6, "héllo…"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := truncate(tc.in, tc.n)
			if got != tc.want {
				t.Fatalf("truncate(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
			}
			if tc.n > 0 && len([]rune(got)) > tc.n {
				t.Fatalf("truncate(%q, %d) returned %d runes, exceeds cap", tc.in, tc.n, len([]rune(got)))
			}
		})
	}
}
