package main

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	cases := []struct {
		name    string
		ldflags string
		info    *debug.BuildInfo
		want    string
	}{
		{"ldflags win", "0.9.0", &debug.BuildInfo{Main: debug.Module{Version: "v0.9.0"}}, "0.9.0"},
		{"go install module version", "dev", &debug.BuildInfo{Main: debug.Module{Version: "v0.9.0"}}, "0.9.0"},
		{"local build", "dev", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, "dev"},
		{"no build info", "dev", nil, "dev"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveVersion(c.ldflags, c.info); got != c.want {
				t.Errorf("resolveVersion(%q) = %q, want %q", c.ldflags, got, c.want)
			}
		})
	}
}
