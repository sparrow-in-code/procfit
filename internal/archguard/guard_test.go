// Package archguard enforces the ports-and-adapters boundary (DEVELOPMENT.md
// §2.4, PM-9003): the OS-agnostic core must not import syscalls, x/sys, or any
// outer-ring adapter package. This test fails the build if the boundary is
// breached, so portability stays free.
package archguard

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// coreDirs are the OS-agnostic core packages (relative to internal/).
var coreDirs = []string{
	"model", "expr", "query", "metrics", "queryspec", "config",
	"control", "collect", "ports", "resolve", "history", "state",
	"render", "render/csv", "render/json", "render/ndjson",
}

// forbidden import substrings for core packages.
var forbidden = []string{
	"syscall",
	"golang.org/x/sys",
	"internal/procfs",
	"internal/daemon",
	"internal/app",
	"internal/tui",
}

func TestCoreDoesNotImportOSOrAdapters(t *testing.T) {
	for _, dir := range coreDirs {
		pkgDir := filepath.Join("..", dir)
		if _, err := os.Stat(pkgDir); err != nil {
			t.Fatalf("core package dir missing: %s", pkgDir)
		}
		for _, imp := range imports(t, pkgDir) {
			for _, bad := range forbidden {
				if strings.Contains(imp, bad) {
					t.Errorf("core package %q imports forbidden %q (breaks the OS-agnostic boundary)", dir, imp)
				}
			}
		}
	}
}

// imports parses non-test .go files in dir and returns their import paths.
func imports(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("glob %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var out []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		for _, imp := range file.Imports {
			out = append(out, strings.Trim(imp.Path.Value, `"`))
		}
	}
	return out
}
