package procfs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/netikras/procfit/internal/model"
)

// writeProc builds a minimal fake /proc tree for testing List without root.
func writeProc(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "sys/kernel/random/boot_id"), "boot-xyz\n")
	mustWrite(t, filepath.Join(root, "stat"), "cpu 1 2 3\nbtime 1600000000\n")

	// pid 100: normal process
	p := filepath.Join(root, "100")
	mustWrite(t, filepath.Join(p, "stat"), "100 (bash) S 1 100 100 0 -1 0 50 0 60 0 5 3 0 0 20 0 2 0 987654 4096000 25 0 0 0 0 0 0 0 0 0 0 0 0 0")
	mustWrite(t, filepath.Join(p, "status"), "Uid:\t1000\t1000\t1000\t1000\nGid:\t1000\t1000\t1000\t1000\nvoluntary_ctxt_switches:\t9\nnonvoluntary_ctxt_switches:\t1\n")
	mustWrite(t, filepath.Join(p, "io"), "rchar: 10\nwchar: 20\nread_bytes: 4096\nwrite_bytes: 8192\nsyscr: 3\nsyscw: 4\n")
	mustWrite(t, filepath.Join(p, "cmdline"), "bash\x00-i\x00")
	mustSymlink(t, "pid:[4026531836]", filepath.Join(p, "ns/pid"))
	mustSymlink(t, "net:[4026531999]", filepath.Join(p, "ns/net"))

	// pid 200: io permission-like (no io file) to exercise availability
	q := filepath.Join(root, "200")
	mustWrite(t, filepath.Join(q, "stat"), "200 (worker proc) R 100 100 100 0 -1 0 0 0 0 0 40 10 0 0 20 0 1 0 111 2048000 12 0 0 0 0 0 0 0 0 0 0 0 0 0")
	mustWrite(t, filepath.Join(q, "cmdline"), "")
	return root
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func TestSource_List(t *testing.T) {
	root := writeProc(t)
	src, err := New(WithRoot(root), WithClockTicks(100))
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := src.BootID(); b != "boot-xyz" {
		t.Fatalf("boot id = %q", b)
	}
	if src.BootTimeUnix() != 1600000000 {
		t.Fatalf("boot time = %d", src.BootTimeUnix())
	}

	stats, err := src.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Fatalf("want 2 procs, got %d", len(stats))
	}

	byPID := map[int]int{}
	for i, s := range stats {
		byPID[s.PID] = i
	}
	p := stats[byPID[100]]
	if p.Comm != "bash" || p.PPID != 1 {
		t.Fatalf("pid 100 fields wrong: %+v", p)
	}
	if p.UID != 1000 || p.VoluntaryCtxt != 9 {
		t.Fatalf("pid 100 status not read: uid=%d vol=%d", p.UID, p.VoluntaryCtxt)
	}
	if p.IOAvail != model.Available || p.IO.ReadBytes != 4096 {
		t.Fatalf("pid 100 io wrong: avail=%s read=%d", p.IOAvail, p.IO.ReadBytes)
	}
	if p.ID.PIDNSInode != 4026531836 {
		t.Fatalf("pid 100 pidns inode = %d", p.ID.PIDNSInode)
	}
	if got, want := p.Namespaces[model.NSNet], uint64(4026531999); got != want {
		t.Fatalf("netns inode = %d, want %d", got, want)
	}
	if len(p.Cmdline) != 2 || p.Cmdline[0] != "bash" {
		t.Fatalf("cmdline wrong: %v", p.Cmdline)
	}
	// RSS pages(25) * pagesize
	if p.RSSBytes == 0 {
		t.Fatal("rss should be non-zero")
	}

	q := stats[byPID[200]]
	if q.Comm != "worker proc" {
		t.Fatalf("pid 200 comm = %q", q.Comm)
	}
	if q.IOAvail == model.Available {
		t.Fatalf("pid 200 has no io file; IOAvail should be unavailable, got %s", q.IOAvail)
	}
}

func TestSource_Hostname(t *testing.T) {
	root := writeProc(t)
	mustWrite(t, filepath.Join(root, "sys/kernel/hostname"), "real-host\n")
	// pid 100 is "in a container": environ carries HOSTNAME.
	mustWrite(t, filepath.Join(root, "100/environ"), "PATH=/usr/bin\x00HOSTNAME=web-1\x00TERM=xterm\x00")
	// pid 200 has no HOSTNAME env -> must fall back to the host hostname.
	mustWrite(t, filepath.Join(root, "200/environ"), "PATH=/usr/bin\x00")

	src, _ := New(WithRoot(root), WithClockTicks(100))

	// Off by default: environ is not read, hostname stays empty (cost on demand).
	stats, _ := src.List(context.Background())
	for _, s := range stats {
		if s.Hostname != "" {
			t.Fatalf("hostname must be off by default, got %q for pid %d", s.Hostname, s.PID)
		}
	}

	src.SetReadHostname(true)
	stats, _ = src.List(context.Background())
	got := map[int]string{}
	for _, s := range stats {
		got[s.PID] = s.Hostname
	}
	if got[100] != "web-1" {
		t.Fatalf("pid 100 HOSTNAME env should win, got %q", got[100])
	}
	if got[200] != "real-host" {
		t.Fatalf("pid 200 should fall back to the host hostname, got %q", got[200])
	}
}

func TestSource_EnumerateThreads(t *testing.T) {
	root := writeProc(t)
	p := filepath.Join(root, "100")
	mustWrite(t, filepath.Join(p, "task/100/stat"), "100 (bash) S 1 100 100 0 -1 0 50 0 60 0 5 3 0 0 20 0 2 0 987654 4096000 25 0 0 0 0 0 0 0 0 0 0 0 0 0")
	mustWrite(t, filepath.Join(p, "task/145/stat"), "145 (worker) R 1 100 100 0 -1 0 0 0 0 0 7 2 0 0 20 0 2 0 987700 4096000 25 0 0 0 0 0 0 0 0 0 0 0 0 0")

	src, _ := New(WithRoot(root), WithClockTicks(100))

	// Off by default: the common path pays no per-thread cost.
	stats, _ := src.List(context.Background())
	for _, s := range stats {
		if len(s.Threads) != 0 {
			t.Fatalf("threads must be off by default, got %d", len(s.Threads))
		}
	}

	// Enabled: pid 100's two tasks are enumerated with TID + comm.
	src.SetEnumerateThreads(true)
	stats, _ = src.List(context.Background())
	tids := map[int]string{}
	for _, s := range stats {
		if s.PID == 100 {
			for _, th := range s.Threads {
				tids[th.TID] = th.Comm
			}
		}
	}
	if len(tids) != 2 || tids[100] != "bash" || tids[145] != "worker" {
		t.Fatalf("thread enumeration wrong: %v", tids)
	}
}

func TestSource_Wchan(t *testing.T) {
	root := writeProc(t)
	mustWrite(t, filepath.Join(root, "100", "wchan"), "do_epoll_wait\n")
	mustWrite(t, filepath.Join(root, "200", "wchan"), "0\n") // running -> normalized to empty
	src, _ := New(WithRoot(root), WithClockTicks(100))

	// Off by default: the common path pays no wchan read.
	stats, _ := src.List(context.Background())
	for _, s := range stats {
		if s.Wchan != "" {
			t.Fatalf("wchan must be off by default, got %q", s.Wchan)
		}
	}

	// Enabled: the blocked pid's symbol is read; "0" (running) normalizes to empty.
	src.SetReadWchan(true)
	stats, _ = src.List(context.Background())
	got := map[int]string{}
	for _, s := range stats {
		got[s.PID] = s.Wchan
	}
	if got[100] != "do_epoll_wait" {
		t.Fatalf("pid100 wchan = %q, want do_epoll_wait", got[100])
	}
	if got[200] != "" {
		t.Fatalf("pid200 wchan '0' should normalize to empty, got %q", got[200])
	}
}

func TestSource_VanishedProcessSkipped(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "stat"), "btime 1\n")
	// A pid dir with no stat file (as if it vanished mid-scan).
	if err := os.MkdirAll(filepath.Join(root, "999"), 0o755); err != nil {
		t.Fatal(err)
	}
	src, _ := New(WithRoot(root))
	stats, err := src.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 0 {
		t.Fatalf("vanished process should be skipped, got %d", len(stats))
	}
}
