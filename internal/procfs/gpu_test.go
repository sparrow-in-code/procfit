package procfs

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/netikras/procfit/internal/model"
)

// writeDRM creates root/<pid>/fd/<fd> -> /dev/dri/renderD128 and the matching
// fdinfo record. writeFDInfo overwrites just the fdinfo body (used mid-window).
func writeDRM(t *testing.T, root string, pid, fd int, content string) {
	t.Helper()
	writeFD(t, root, pid, fd, "/dev/dri/renderD128", content)
}

func writeFD(t *testing.T, root string, pid, fd int, target, content string) {
	t.Helper()
	fddir := filepath.Join(root, strconv.Itoa(pid), "fd")
	fiDir := filepath.Join(root, strconv.Itoa(pid), "fdinfo")
	if err := os.MkdirAll(fddir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(fddir, strconv.Itoa(fd))
	_ = os.Remove(link)
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	writeFDInfo(t, root, pid, fd, content)
}

func writeFDInfo(t *testing.T, root string, pid, fd int, content string) {
	t.Helper()
	p := filepath.Join(root, strconv.Itoa(pid), "fdinfo", strconv.Itoa(fd))
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGPUCollector_WindowAndDedup(t *testing.T) {
	root := t.TempDir()

	// pid 100: amdgpu client whose render engine advances 50ms over a 100ms
	// window, and holds 1 MiB of VRAM.
	before := "drm-driver:\tamdgpu\ndrm-pdev:\t0000:03:00.0\ndrm-client-id:\t1\n" +
		"drm-engine-render:\t1000000 ns\ndrm-engine-copy:\t0 ns\ndrm-memory-vram:\t1024 KiB\n"
	after := "drm-driver:\tamdgpu\ndrm-pdev:\t0000:03:00.0\ndrm-client-id:\t1\n" +
		"drm-engine-render:\t51000000 ns\ndrm-engine-copy:\t0 ns\ndrm-memory-vram:\t1024 KiB\n"
	writeDRM(t, root, 100, 7, before)

	// pid 200: not a GPU client (a plain fd).
	writeFD(t, root, 200, 3, "/dev/null", "pos:\t0\n")

	// pid 300: two fds sharing one client-id (dup) → engine + memory counted once;
	// memory falls back to drm-total when no resident/memory keys are present.
	dup := "drm-driver:\ti915\ndrm-pdev:\t0000:00:02.0\ndrm-client-id:\t9\n" +
		"drm-engine-render:\t10000000 ns\ndrm-engine-capacity-render:\t2\ndrm-total-local0:\t2048 KiB\n"
	writeDRM(t, root, 300, 4, dup)
	writeDRM(t, root, 300, 5, dup)

	c := NewGPUCollector(root)
	c.window = 100 * time.Millisecond
	c.sleep = func(time.Duration) { // mid-window: advance pid 100's counter
		writeFDInfo(t, root, 100, 7, after)
	}

	procs := []model.Process{{PID: 100}, {PID: 200}, {PID: 300}}
	c.Collect(context.Background(), procs)

	// pid 100: 50ms busy / 100ms window = 50%.
	if v := procs[0].Metric("gpu"); !v.Present() || v.V < 49.9 || v.V > 50.1 {
		t.Fatalf("pid100 gpu = %+v, want ~50%%", v)
	}
	if v := procs[0].Metric("gpu-mem"); !v.Present() || v.V != 1024*1024 {
		t.Fatalf("pid100 gpu-mem = %+v, want 1 MiB", v)
	}

	// pid 200: not a GPU client → both metrics absent.
	if v := procs[1].Metric("gpu"); v.Present() {
		t.Fatalf("pid200 must have no gpu metric, got %+v", v)
	}
	if v := procs[1].Metric("gpu-mem"); v.Present() {
		t.Fatalf("pid200 must have no gpu-mem metric, got %+v", v)
	}

	// pid 300: dup fds deduped → busy delta 0 (no advance) = 0%, memory once (2 MiB
	// from drm-total, not 4). The engine-capacity line must NOT inflate busy.
	if v := procs[2].Metric("gpu"); !v.Present() || v.V != 0 {
		t.Fatalf("pid300 gpu = %+v, want 0%%", v)
	}
	if v := procs[2].Metric("gpu-mem"); !v.Present() || v.V != 2048*1024 {
		t.Fatalf("pid300 gpu-mem = %+v, want 2 MiB (deduped)", v)
	}
}

func TestGPUCollector_NoDRMSkipsWindow(t *testing.T) {
	root := t.TempDir()
	writeFD(t, root, 1, 0, "/dev/null", "pos:\t0\n")
	c := NewGPUCollector(root)
	slept := false
	c.sleep = func(time.Duration) { slept = true }
	procs := []model.Process{{PID: 1}}
	c.Collect(context.Background(), procs)
	if slept {
		t.Fatal("with no DRM clients the collector must skip the window sleep")
	}
	if procs[0].Metric("gpu").Present() {
		t.Fatal("a non-GPU process must not get a gpu value")
	}
}

func TestGPUCollector_PermissionSurfaced(t *testing.T) {
	// A pid whose /proc/PID/fd cannot be read should report a reason, not "no GPU".
	c := NewGPUCollector(t.TempDir()) // dir has no such pid → read fails (not-exist)
	procs := []model.Process{{PID: 4242}}
	c.Collect(context.Background(), procs)
	// Not-exist maps to Vanished; either way it must be non-present with a reason,
	// and NOT silently absent-as-"no GPU". Since nothing had a DRM client the
	// window is skipped, but read errors are still surfaced.
	v := procs[0].Metric("gpu")
	if v.Present() || v.Availability != model.Vanished {
		t.Fatalf("missing pid should surface Vanished, got %+v", v)
	}
}

func TestParseDRMFdinfo(t *testing.T) {
	body := "pos:\t0\nflags:\t02000002\n" +
		"drm-driver:\tamdgpu\ndrm-pdev:\t0000:03:00.0\ndrm-client-id:\t42\n" +
		"drm-engine-gfx:\t100 ns\ndrm-engine-compute:\t250 ns\n" +
		"drm-engine-capacity-gfx:\t1\n" + // must be ignored
		"drm-resident-vram:\t512 KiB\ndrm-total-vram:\t4096 KiB\n"
	c := parseDRMFdinfo(body)
	if !c.isDRM || c.clientID != "42" || c.pdev != "0000:03:00.0" {
		t.Fatalf("identity parse wrong: %+v", c)
	}
	if c.busyNs != 350 { // 100 + 250, capacity excluded
		t.Fatalf("engine ns = %d, want 350", c.busyNs)
	}
	// resident wins over total for the memory figure.
	if got := c.memBytes(); got != 512*1024 {
		t.Fatalf("memBytes = %d, want %d (resident preferred)", got, 512*1024)
	}
	// A non-DRM record parses as not-a-client.
	if parseDRMFdinfo("pos:\t0\nino:\t1\n").isDRM {
		t.Fatal("a plain fdinfo must not be classified as DRM")
	}
}

func TestParseMemBytes(t *testing.T) {
	cases := map[string]uint64{
		"1024 KiB": 1024 * 1024,
		"2 MiB":    2 * 1024 * 1024,
		"1 GiB":    1024 * 1024 * 1024,
		"4096 B":   4096,
		"512":      512, // no unit → bytes
		"":         0,
	}
	for in, want := range cases {
		if got := parseMemBytes(in); got != want {
			t.Fatalf("parseMemBytes(%q) = %d, want %d", in, got, want)
		}
	}
}
