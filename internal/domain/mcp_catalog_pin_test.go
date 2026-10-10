package domain_test

import (
	"encoding/json"
	"io/fs"
	"path"
	"regexp"
	"testing"

	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// Stricter than scan's SCAN-003 check on purpose: ranges such as 1.x or
// ^1.2.3 are not exact versions.
var (
	exactNodeSpec   = regexp.MustCompile(`^(@[a-z0-9][\w.-]*/)?[a-z0-9][\w.-]*@\d+\.\d+\.\d+(-[\w.]+)?$`)
	exactPythonSpec = regexp.MustCompile(`^[A-Za-z0-9][\w.-]*(==|@)\d+(\.\d+)*$`)
)

func launchedPackage(argv []string) (runner, spec string) {
	if len(argv) == 0 {
		return "", ""
	}
	runner = path.Base(argv[0])
	if runner != "npx" && runner != "bunx" && runner != "uvx" {
		return "", ""
	}
	for _, a := range argv[1:] {
		if a != "" && a[0] != '-' {
			return runner, a
		}
	}
	return runner, ""
}

func assertExactVersion(t *testing.T, source string, argv []string) {
	t.Helper()
	runner, spec := launchedPackage(argv)
	if runner == "" {
		return
	}
	re := exactNodeSpec
	if runner == "uvx" {
		re = exactPythonSpec
	}
	if !re.MatchString(spec) {
		t.Errorf("%s: %s package %q is not pinned to an exact version", source, runner, spec)
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
		t.Fatal("no npx/bunx/uvx entries in the catalog; the test would pass vacuously")
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
