package domain_test

import (
	"encoding/json"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// Stricter than scan's SCAN-003 check on purpose: ranges such as 1.x or
// ^1.2.3 are not exact versions, and image tags such as :latest or :0.20 move.
var (
	exactNodeSpec   = regexp.MustCompile(`^(@[a-z0-9][\w.-]*/)?[a-z0-9][\w.-]*@\d+\.\d+\.\d+(-[\w.]+)?$`)
	exactPythonSpec = regexp.MustCompile(`^[A-Za-z0-9][\w.-]*(==|@)\d+(\.\d+)*$`)
	exactImageRef   = regexp.MustCompile(`^[a-z0-9][\w./-]*(:v?\d+\.\d+\.\d+(-[\w.]+)?|@sha256:[0-9a-f]{64})$`)
)

// dockerValueFlags are the `docker run` flags that consume the next argument,
// so that argument is not mistaken for the image.
var dockerValueFlags = map[string]bool{
	"-e": true, "--env": true, "--env-file": true, "-v": true, "--volume": true,
	"-p": true, "--publish": true, "--name": true, "--network": true, "-w": true,
	"--workdir": true, "--entrypoint": true, "-u": true, "--user": true,
	"--mount": true, "-l": true, "--label": true, "--platform": true,
}

func launchedPackage(argv []string) (runner, spec string) {
	if len(argv) == 0 {
		return "", ""
	}
	runner = path.Base(argv[0])
	switch runner {
	case "npx", "bunx", "uvx":
		for _, a := range argv[1:] {
			if a != "" && a[0] != '-' {
				return runner, a
			}
		}
		return runner, ""
	case "docker", "podman":
		if len(argv) < 2 || argv[1] != "run" {
			return "", ""
		}
		for i := 2; i < len(argv); i++ {
			a := argv[i]
			switch {
			case dockerValueFlags[a]:
				i++
			case strings.HasPrefix(a, "-"):
			default:
				return runner, a
			}
		}
		return runner, ""
	}
	return "", ""
}

// unpinnedLaunch returns "<runner> <spec>" when argv launches a package or
// image that is not pinned to an exact version, and "" otherwise.
func unpinnedLaunch(argv []string) string {
	runner, spec := launchedPackage(argv)
	if runner == "" {
		return ""
	}
	re := exactNodeSpec
	switch runner {
	case "uvx":
		re = exactPythonSpec
	case "docker", "podman":
		re = exactImageRef
	}
	if re.MatchString(spec) {
		return ""
	}
	return runner + " " + spec
}

func assertExactVersion(t *testing.T, source string, argv []string) {
	t.Helper()
	if got := unpinnedLaunch(argv); got != "" {
		t.Errorf("%s: %q is not pinned to an exact version", source, got)
	}
}

func TestUnpinnedLaunch_Rules(t *testing.T) {
	const img = "ghcr.io/github/github-mcp-server"
	cases := []struct {
		name   string
		argv   []string
		pinned bool
	}{
		{"npx exact", []string{"npx", "-y", "@upstash/context7-mcp@4.3.0"}, true},
		{"npx latest", []string{"npx", "-y", "@upstash/context7-mcp@latest"}, false},
		{"uvx exact", []string{"uvx", "mcp-server-git==2025.1.14"}, true},
		{"docker exact tag", []string{"docker", "run", "-i", "--rm", "-e", "GITHUB_PERSONAL_ACCESS_TOKEN", img + ":v2.0.2"}, true},
		{"docker digest", []string{"docker", "run", "-i", img + "@sha256:" + strings.Repeat("ab", 32)}, true},
		{"docker untagged", []string{"docker", "run", "-i", "--rm", "-e", "GITHUB_PERSONAL_ACCESS_TOKEN", img}, false},
		{"docker latest", []string{"docker", "run", "-i", "--rm", img + ":latest"}, false},
		{"docker moving minor tag", []string{"docker", "run", "-i", img + ":0.20"}, false},
		{"docker env value is not the image", []string{"docker", "run", "-e", "A=b:1.2.3", img}, false},
		{"not a launcher", []string{"squadai", "mcp-server"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := unpinnedLaunch(tc.argv)
			if tc.pinned && got != "" {
				t.Errorf("flagged as unpinned: %q", got)
			}
			if !tc.pinned && got == "" {
				t.Errorf("%v accepted as pinned", tc.argv)
			}
		})
	}
}

func TestMCPCatalog_PackagesPinnedToExactVersions(t *testing.T) {
	launchers := 0
	for _, s := range domain.DefaultMCPCatalog() {
		argv := append([]string{s.Command}, s.Args...)
		if r, _ := launchedPackage(argv); r != "" {
			launchers++
		}
		assertExactVersion(t, "catalog "+s.Name, argv)
	}
	if launchers == 0 {
		t.Fatal("no npx/bunx/uvx/docker entries in the catalog; the test would pass vacuously")
	}

	files, err := fs.Glob(assets.FS, "mcp/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no embedded MCP assets found: %v", err)
	}
	for _, f := range files {
		var def domain.MCPServerDef
		if err := json.Unmarshal([]byte(assets.MustRead(f)), &def); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		assertExactVersion(t, f, def.Command)
	}
}
