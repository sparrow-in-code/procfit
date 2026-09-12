package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/netikras/procfit/internal/control"
	"github.com/netikras/procfit/internal/model"
)

func TestStore_SaveLoadRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "procfit")
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Directory must be 0700.
	info, _ := os.Stat(dir)
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode = %v, want 0700", info.Mode().Perm())
	}

	st := &control.State{
		SchemaVersion: control.SchemaVersion, BootID: "boot", Generation: 3, UpdatedAt: time.Unix(1000, 0).UTC(),
		Targets: []control.Target{{ID: "runtime:x", Name: "x", Origin: control.OriginRuntime, BindingMode: control.ModeFollow,
			Bindings: []control.Binding{{ID: model.ProcessInstanceID{PID: 7, StartTime: 70}, PID: 7, Nice: &control.NiceField{Original: 0, Desired: 10, Observed: 10, Status: control.StatusApplied}}}}},
	}
	if err := s.Save(st); err != nil {
		t.Fatal(err)
	}
	// File must be 0600.
	fi, _ := os.Stat(s.path())
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v, want 0600", fi.Mode().Perm())
	}

	got, ok, err := s.Load()
	if err != nil || !ok {
		t.Fatalf("load failed ok=%v err=%v", ok, err)
	}
	if got.Generation != 3 || len(got.Targets) != 1 || got.Targets[0].Bindings[0].Nice.Desired != 10 {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestStore_LoadAbsent(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "p"))
	if _, ok, err := s.Load(); ok || err != nil {
		t.Fatalf("absent state should be ok=false err=nil, got ok=%v err=%v", ok, err)
	}
}

func TestStore_CorruptQuarantined(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	s, _ := Open(dir)
	if err := os.WriteFile(s.path(), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.Load(); ok || err == nil {
		t.Fatal("corrupt file should error and be quarantined")
	}
	if _, err := os.Stat(s.path() + ".corrupt"); err != nil {
		t.Fatal("corrupt file should be moved to .corrupt")
	}
}

func TestStore_AtomicOverwrite(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	s, _ := Open(dir)
	_ = s.Save(&control.State{Generation: 1, BootID: "b"})
	_ = s.Save(&control.State{Generation: 2, BootID: "b"})
	got, _, _ := s.Load()
	if got.Generation != 2 {
		t.Fatalf("second save should win, got gen %d", got.Generation)
	}
	// No leftover temp files.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Fatalf("leftover temp file %s", e.Name())
		}
	}
}
