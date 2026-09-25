package procfs

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/netikras/procfit/internal/model"
)

// SocketCollector classifies each process's sockets by protocol and TCP state
// (RFC §13.2) without eBPF or root: it reads the socket inodes from
// /proc/PID/fd/* and joins them against /proc/net/{tcp,tcp6,udp,udp6,unix}.
// Throughput (bytes/packets) is NOT here — that stays the eBPF net-* path, since
// /proc/net is socket-scoped, not per-process traffic.
//
// Limitation: it reads the collector's own network namespace view of /proc/net,
// so sockets held by processes in a different netns are simply not classified
// (counted in none), never wrong.
type SocketCollector struct {
	root string
	sem  chan struct{}
}

// NewSocketCollector builds a collector rooted at the given procfs path.
func NewSocketCollector(root string) *SocketCollector {
	if root == "" {
		root = "/proc"
	}
	return &SocketCollector{root: root, sem: make(chan struct{}, maxFDScans)}
}

// ID identifies the collector.
func (c *SocketCollector) ID() string { return "socket" }

// Metrics lists the produced ids.
func (c *SocketCollector) Metrics() []model.MetricID {
	return []model.MetricID{
		"sock-tcp", "sock-udp", "sock-unix",
		"sock-listen", "sock-estab", "sock-timewait", "sock-closewait",
	}
}

// sockInfo is a socket's protocol and (TCP) state, keyed by inode.
type sockInfo struct {
	proto string // tcp | udp | unix
	state string // TCP hex state ("" for udp/unix)
}

// Collect builds the inode→socket table once, then classifies each process's
// socket fds against it with bounded concurrency.
func (c *SocketCollector) Collect(ctx context.Context, procs []model.Process) {
	table := c.readNetTables()
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
			c.collectOne(p, table)
		}(&procs[i])
	}
	for i := 0; i < launched; i++ {
		<-done
	}
}

func (c *SocketCollector) collectOne(p *model.Process, table map[uint64]sockInfo) {
	dir := filepath.Join(c.root, itoa(p.PID), "fd")
	entries, err := os.ReadDir(dir)
	if err != nil {
		avail := model.ReadError
		if os.IsPermission(err) {
			avail = model.PermissionDenied
		}
		for _, id := range c.Metrics() {
			p.SetMetric(id, model.Unavailable[float64](avail, "socket"))
		}
		return
	}
	var sc sockCounts
	for _, e := range entries {
		target, err := os.Readlink(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if ino, ok := socketInode(target); ok {
			if info, ok := table[ino]; ok {
				sc.add(info)
			}
		}
	}
	sc.apply(p)
}

// sockCounts accumulates a process's socket classification.
type sockCounts struct {
	tcp, udp, unix                     int
	listen, estab, timewait, closewait int
}

func (s *sockCounts) add(info sockInfo) {
	switch info.proto {
	case "tcp":
		s.tcp++
		switch info.state {
		case "0A":
			s.listen++
		case "01":
			s.estab++
		case "06":
			s.timewait++
		case "08":
			s.closewait++
		}
	case "udp":
		s.udp++
	case "unix":
		s.unix++
	}
}

func (s *sockCounts) apply(p *model.Process) {
	set := func(id string, v int) {
		p.SetMetric(model.MetricID(id), model.NewValue(float64(v), model.Sampled, "socket"))
	}
	set("sock-tcp", s.tcp)
	set("sock-udp", s.udp)
	set("sock-unix", s.unix)
	set("sock-listen", s.listen)
	set("sock-estab", s.estab)
	set("sock-timewait", s.timewait)
	set("sock-closewait", s.closewait)
}

// readNetTables builds the inode→sockInfo map from the /proc/net socket tables.
func (c *SocketCollector) readNetTables() map[uint64]sockInfo {
	table := map[uint64]sockInfo{}
	add := func(file, proto string, inodeField, stateField int) {
		data, err := os.ReadFile(filepath.Join(c.root, "net", file))
		if err != nil {
			return
		}
		mergeNetTable(table, string(data), proto, inodeField, stateField)
	}
	// tcp/udp: state at field 3, inode at field 9. unix: inode at field 6, no state.
	add("tcp", "tcp", 9, 3)
	add("tcp6", "tcp", 9, 3)
	add("udp", "udp", 9, 3)
	add("udp6", "udp", 9, 3)
	add("unix", "unix", 6, -1)
	return table
}

// mergeNetTable folds one /proc/net table into the inode map (header line skipped;
// inode 0 — unbound — ignored). Pure and unit-tested.
func mergeNetTable(table map[uint64]sockInfo, data, proto string, inodeField, stateField int) {
	lines := strings.Split(data, "\n")
	for i, line := range lines {
		if i == 0 { // header
			continue
		}
		f := strings.Fields(line)
		need := inodeField
		if stateField > need {
			need = stateField
		}
		if len(f) <= need {
			continue
		}
		ino, err := strconv.ParseUint(f[inodeField], 10, 64)
		if err != nil || ino == 0 {
			continue
		}
		info := sockInfo{proto: proto}
		if stateField >= 0 {
			info.state = f[stateField]
		}
		table[ino] = info
	}
}

// socketInode parses the inode from a "socket:[12345]" fd symlink target.
func socketInode(target string) (uint64, bool) {
	const prefix = "socket:["
	if !strings.HasPrefix(target, prefix) || !strings.HasSuffix(target, "]") {
		return 0, false
	}
	n, err := strconv.ParseUint(target[len(prefix):len(target)-1], 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
