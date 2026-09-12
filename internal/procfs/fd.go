package procfs

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/netikras/procfit/internal/model"
)

// maxFDScans bounds concurrent /proc/PID/fd directory scans so a large process
// count cannot create an I/O storm (RFC §22).
const maxFDScans = 32

// FDCollector classifies open file descriptors per process (RFC §13.2). Counts
// carry Sampled quality because the set can change mid-scan.
type FDCollector struct {
	root string
	sem  chan struct{}
}

// NewFDCollector builds a collector rooted at the given procfs path (default
// /proc when empty).
func NewFDCollector(root string) *FDCollector {
	if root == "" {
		root = "/proc"
	}
	return &FDCollector{root: root, sem: make(chan struct{}, maxFDScans)}
}

// ID identifies the collector.
func (c *FDCollector) ID() string { return "fd" }

// Metrics lists the metric ids produced.
func (c *FDCollector) Metrics() []model.MetricID {
	return []model.MetricID{
		"fd-total", "fd-files", "fd-sockets", "fd-pipes", "fd-anon",
		"fd-eventfd", "fd-epoll", "fd-timerfd", "fd-signalfd", "fd-inotify",
	}
}

// Collect enriches each process with fd counts, using bounded concurrency.
func (c *FDCollector) Collect(ctx context.Context, procs []model.Process) {
	done := make(chan struct{})
	var launched int
	for i := range procs {
		if ctx.Err() != nil {
			break
		}
		launched++
		go func(p *model.Process) {
			c.sem <- struct{}{}
			defer func() { <-c.sem; done <- struct{}{} }()
			c.collectOne(p)
		}(&procs[i])
	}
	for i := 0; i < launched; i++ {
		<-done
	}
}

func (c *FDCollector) collectOne(p *model.Process) {
	dir := filepath.Join(c.root, itoa(p.PID), "fd")
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.setUnavailable(p, err)
		return
	}
	var counts fdCounts
	for _, e := range entries {
		target, err := os.Readlink(filepath.Join(dir, e.Name()))
		if err != nil {
			continue // fd closed mid-scan
		}
		counts.add(classifyFD(target))
	}
	counts.apply(p)
}

func (c *FDCollector) setUnavailable(p *model.Process, err error) {
	avail := model.ReadError
	if os.IsPermission(err) {
		avail = model.PermissionDenied
	}
	for _, id := range c.Metrics() {
		p.SetMetric(id, model.Unavailable[float64](avail, "fd"))
	}
}

type fdCounts struct {
	total, files, sockets, pipes, anon         int
	eventfd, epoll, timerfd, signalfd, inotify int
}

func (f *fdCounts) add(kind string) {
	f.total++
	switch kind {
	case "file":
		f.files++
	case "socket":
		f.sockets++
	case "pipe":
		f.pipes++
	case "eventfd":
		f.anon++
		f.eventfd++
	case "epoll":
		f.anon++
		f.epoll++
	case "timerfd":
		f.anon++
		f.timerfd++
	case "signalfd":
		f.anon++
		f.signalfd++
	case "inotify":
		f.anon++
		f.inotify++
	case "anon":
		f.anon++
	}
}

func (f *fdCounts) apply(p *model.Process) {
	set := func(id string, v int) {
		p.SetMetric(model.MetricID(id), model.NewValue(float64(v), model.Sampled, "fd"))
	}
	set("fd-total", f.total)
	set("fd-files", f.files)
	set("fd-sockets", f.sockets)
	set("fd-pipes", f.pipes)
	set("fd-anon", f.anon)
	set("fd-eventfd", f.eventfd)
	set("fd-epoll", f.epoll)
	set("fd-timerfd", f.timerfd)
	set("fd-signalfd", f.signalfd)
	set("fd-inotify", f.inotify)
}

// classifyFD maps a /proc/PID/fd/* symlink target to a category. Pure/testable.
func classifyFD(target string) string {
	switch {
	case strings.HasPrefix(target, "socket:"):
		return "socket"
	case strings.HasPrefix(target, "pipe:"):
		return "pipe"
	case strings.HasPrefix(target, "anon_inode:"):
		return classifyAnon(target[len("anon_inode:"):])
	default:
		return "file"
	}
}

func classifyAnon(name string) string {
	name = strings.Trim(name, "[]")
	switch {
	case name == "eventfd":
		return "eventfd"
	case name == "eventpoll" || name == "[eventpoll]":
		return "epoll"
	case strings.Contains(name, "timerfd"):
		return "timerfd"
	case strings.Contains(name, "signalfd"):
		return "signalfd"
	case strings.Contains(name, "inotify"):
		return "inotify"
	default:
		return "anon"
	}
}
