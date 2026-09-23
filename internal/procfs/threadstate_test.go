package procfs

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/netikras/procfit/internal/model"
)

func TestThreadStateCollector(t *testing.T) {
	root := t.TempDir()
	// pid 100: three threads — one R, one D, one S. comm has a space+paren to
	// exercise the "state after the last )" parse.
	mustWrite(t, filepath.Join(root, "100/task/100/stat"), "100 (weird )name) R 1 100 100 0 -1 0 0 0 0 0 1 1 0 0 20 0 3 0 1 1 1")
	mustWrite(t, filepath.Join(root, "100/task/101/stat"), "101 (io) D 1 100 100 0 -1 0 0 0 0 0 1 1 0 0 20 0 3 0 1 1 1")
	mustWrite(t, filepath.Join(root, "100/task/102/stat"), "102 (idle) S 1 100 100 0 -1 0 0 0 0 0 1 1 0 0 20 0 3 0 1 1 1")

	c := NewThreadStateCollector(root)
	procs := []model.Process{{PID: 100}, {PID: 200}} // 200 has no task dir
	c.Collect(context.Background(), procs)

	if v := procs[0].Metric("threads-running"); !v.Present() || v.V != 1 {
		t.Fatalf("pid100 threads-running = %+v, want 1", v)
	}
	if v := procs[0].Metric("threads-uninterruptible"); !v.Present() || v.V != 1 {
		t.Fatalf("pid100 threads-uninterruptible = %+v, want 1", v)
	}
	if v := procs[1].Metric("threads-running"); v.Present() {
		t.Fatalf("pid200 (no task dir) must be unavailable, got %+v", v)
	}
}

func TestReadTaskState(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "stat")
	mustWrite(t, p, "42 (a )b) D 1 2 3")
	if st := readTaskState(p); st != 'D' {
		t.Fatalf("state = %q, want D", st)
	}
	if st := readTaskState(filepath.Join(root, "missing")); st != 0 {
		t.Fatalf("missing file should yield 0, got %q", st)
	}
}
