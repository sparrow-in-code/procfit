package procfs

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/netikras/procfit/internal/model"
)

func TestParseSmapsRollup(t *testing.T) {
	body := "563e0000-563e1000 ---p 00000000 00:00 0 [rollup]\n" +
		"Rss:                1200 kB\n" +
		"Pss:                 800 kB\n" +
		"Shared_Clean:        400 kB\n" +
		"Private_Clean:       100 kB\n" +
		"Private_Dirty:       300 kB\n" +
		"Swap:                 64 kB\n" +
		"SwapPss:              32 kB\n"
	r := parseSmapsRollup(body)
	if r.pss != 800*1024 {
		t.Fatalf("pss = %d, want %d", r.pss, 800*1024)
	}
	if r.uss() != (100+300)*1024 {
		t.Fatalf("uss = %d, want %d", r.uss(), 400*1024)
	}
	if r.swapPss != 32*1024 {
		t.Fatalf("swap-pss = %d, want %d", r.swapPss, 32*1024)
	}
}

func writeProcFile(t *testing.T, root string, pid int, name, content string) {
	t.Helper()
	dir := filepath.Join(root, strconv.Itoa(pid))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProcMemCollector(t *testing.T) {
	root := t.TempDir()
	// pid 100: full memory detail.
	writeProcFile(t, root, 100, "smaps_rollup",
		"Pss:  800 kB\nPrivate_Clean: 100 kB\nPrivate_Dirty: 300 kB\nSwapPss: 32 kB\n")
	writeProcFile(t, root, 100, "oom_score", "667\n")
	writeProcFile(t, root, 100, "oom_score_adj", "-200\n")
	// pid 200: no smaps_rollup (kernel without page monitor) -> pss/uss unavailable.
	writeProcFile(t, root, 200, "oom_score", "10\n")

	c := NewProcMemCollector(root)
	procs := []model.Process{{PID: 100}, {PID: 200}}
	c.Collect(context.Background(), procs)

	if v := procs[0].Metric("pss"); !v.Present() || v.V != 800*1024 {
		t.Fatalf("pid100 pss = %+v, want 800 KiB", v)
	}
	if v := procs[0].Metric("uss"); !v.Present() || v.V != 400*1024 {
		t.Fatalf("pid100 uss = %+v, want 400 KiB", v)
	}
	if v := procs[0].Metric("oom-score"); !v.Present() || v.V != 667 {
		t.Fatalf("pid100 oom-score = %+v, want 667", v)
	}
	if v := procs[0].Metric("oom-score-adj"); !v.Present() || v.V != -200 {
		t.Fatalf("pid100 oom-score-adj = %+v, want -200", v)
	}
	// pid 200: no rollup -> pss unavailable (not zero); oom still present.
	if v := procs[1].Metric("pss"); v.Present() {
		t.Fatalf("pid200 pss should be unavailable, got %+v", v)
	}
	if v := procs[1].Metric("oom-score"); !v.Present() || v.V != 10 {
		t.Fatalf("pid200 oom-score = %+v, want 10", v)
	}
}
