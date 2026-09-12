package procfs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/netikras/procfit/internal/model"
)

func TestClassifyFD(t *testing.T) {
	cases := map[string]string{
		"socket:[12345]":          "socket",
		"pipe:[999]":              "pipe",
		"anon_inode:[eventfd]":    "eventfd",
		"anon_inode:[eventpoll]":  "epoll",
		"anon_inode:[timerfd]":    "timerfd",
		"anon_inode:[signalfd]":   "signalfd",
		"anon_inode:inotify":      "inotify",
		"anon_inode:[perf_event]": "anon",
		"/usr/lib/libc.so":        "file",
		"/dev/null":               "file",
	}
	for target, want := range cases {
		if got := classifyFD(target); got != want {
			t.Errorf("classifyFD(%q) = %q, want %q", target, got, want)
		}
	}
}

func TestFDCollector_Collect(t *testing.T) {
	root := t.TempDir()
	fddir := filepath.Join(root, "100", "fd")
	if err := os.MkdirAll(fddir, 0o755); err != nil {
		t.Fatal(err)
	}
	links := map[string]string{
		"0": "/dev/pts/0",
		"3": "socket:[1]",
		"4": "socket:[2]",
		"5": "pipe:[3]",
		"6": "anon_inode:[eventfd]",
		"7": "anon_inode:[eventpoll]",
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(fddir, name)); err != nil {
			t.Fatal(err)
		}
	}
	c := NewFDCollector(root)
	procs := []model.Process{{PID: 100}}
	c.Collect(context.Background(), procs)
	m := procs[0]

	check := func(id string, want float64) {
		v, ok := m.Metric(model.MetricID(id)).Get()
		if !ok || v != want {
			t.Errorf("%s = (%v,%v), want %v", id, v, ok, want)
		}
	}
	check("fd-total", 6)
	check("fd-sockets", 2)
	check("fd-pipes", 1)
	check("fd-files", 1) // /dev/pts/0
	check("fd-eventfd", 1)
	check("fd-epoll", 1)
	check("fd-anon", 2)
	// Sampled quality (RFC §13.2).
	if q := m.Metric("fd-total").Quality; q != model.Sampled {
		t.Fatalf("fd-total quality = %q, want sampled", q)
	}
}

func TestFDCollector_Unavailable(t *testing.T) {
	c := NewFDCollector(t.TempDir()) // no /100/fd dir
	procs := []model.Process{{PID: 100}}
	c.Collect(context.Background(), procs)
	if procs[0].Metric("fd-total").Present() {
		t.Fatal("missing fd dir should yield unavailable fd-total, not a value")
	}
}
