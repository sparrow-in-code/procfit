//go:build linux

package procfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestController_FreezeCgroup(t *testing.T) {
	cgRoot := t.TempDir()
	// A cgroup v2 hierarchy with a writable unit cgroup.
	if err := os.WriteFile(filepath.Join(cgRoot, "cgroup.controllers"), []byte("cpu memory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unit := filepath.Join(cgRoot, "system.slice", "app.service")
	if err := os.MkdirAll(unit, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unit, "cgroup.freeze"), []byte("0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	src, _ := New(WithRoot(t.TempDir()))
	c := NewController(src).WithCgroupRoot(cgRoot)

	if !c.Capabilities().Freeze {
		t.Fatal("freeze capability should be detected when cgroup.controllers exists")
	}
	if err := c.FreezeCgroup("/system.slice/app.service", true); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(unit, "cgroup.freeze"))
	if string(data) != "1" {
		t.Fatalf("cgroup.freeze = %q, want 1", string(data))
	}
	if err := c.FreezeCgroup("/system.slice/app.service", false); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(unit, "cgroup.freeze"))
	if string(data) != "0" {
		t.Fatalf("cgroup.freeze = %q, want 0", string(data))
	}
	// Non-existent cgroup errors.
	if err := c.FreezeCgroup("/nope.service", true); err == nil {
		t.Fatal("freezing a missing cgroup should error")
	}
}
