//go:build linux

package procfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/netikras/procfit/internal/ports"
)

// TestController_SelfReadsAndHarmlessOps exercises the real Linux controller
// against our own process using only safe operations (reads, setting nice to its
// current value, SIGCONT which is a no-op for a running process).
func TestController_SelfReadsAndHarmlessOps(t *testing.T) {
	src, err := New()
	if err != nil {
		t.Fatal(err)
	}
	c := NewController(src)
	self := os.Getpid()

	nice, err := c.GetNice(self)
	if err != nil {
		t.Fatalf("GetNice(self): %v", err)
	}
	if err := c.SetNice(self, nice); err != nil { // set to current value: no real change
		t.Fatalf("SetNice(self, same): %v", err)
	}
	id, ok := c.ReadIdentity(self)
	if !ok || id.PID != self {
		t.Fatalf("ReadIdentity(self) = %+v ok=%v", id, ok)
	}
	if _, ok := c.ReadCgroupOf(self); !ok {
		t.Log("ReadCgroupOf(self): no cgroup path (non-cgroup-v2 host?)")
	}
	if err := c.SendSignal(self, ports.SigCont); err != nil { // harmless for a running process
		t.Fatalf("SendSignal(self, CONT): %v", err)
	}
	if _, err := c.GetNice(1 << 30); err == nil {
		t.Fatal("GetNice on a nonexistent pid should error")
	}
	if err := c.SendSignal(self, ports.Signal("BOGUS")); err == nil {
		t.Fatal("unknown signal should error")
	}
}

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
