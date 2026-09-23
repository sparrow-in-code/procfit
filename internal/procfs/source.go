package procfs

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/ports"
)

// Source is the Linux /proc-backed ports.ProcessSource. Its root is
// configurable so tests can point it at a fixture tree instead of the live
// /proc (RFC §14.1). A future OS would be a separate adapter package
// implementing the same port (DEVELOPMENT.md §2.4).
type Source struct {
	root         string
	hz           int64
	cpus         int
	bootID       string
	bootUnix     int64
	pageSize     int64
	enumThread   bool
	readWchan    bool
	readHostname bool
	// hostname is the host's own hostname, read once; used as the fallback when a
	// process has no HOSTNAME env (or environ is unreadable).
	hostname string
	// hostCache/hostCachePrev memoize per-instance HOSTNAME (environ is fixed at
	// exec). Two generations, rotated per List, bound memory to live processes.
	hostMu        sync.Mutex
	hostCache     map[string]string
	hostCachePrev map[string]string
	// blkioAvail is Available unless kernel delay accounting is off (then the
	// stat blkio-delay counter is always 0 and must read as Disabled, not zero).
	blkioAvail model.Availability
}

// SetEnumerateThreads toggles per-thread (/proc/PID/task) enumeration. It is
// off by default so the common process/none leaf modes pay no per-thread cost;
// the app turns it on only for `--leaf thread`.
func (s *Source) SetEnumerateThreads(on bool) { s.enumThread = on }

// SetReadWchan toggles reading /proc/PID/wchan per process. Off by default so the
// common path pays no extra read; the app turns it on only when a query
// references the wchan field (column/group-by/filter).
func (s *Source) SetReadWchan(on bool) { s.readWchan = on }

// SetReadHostname toggles reading /proc/PID/environ for the HOSTNAME env var. Off
// by default (environ is permission-gated and adds a read per process); the app
// turns it on only when a query references the hostname field.
func (s *Source) SetReadHostname(on bool) { s.readHostname = on }

// Option configures a Source.
type Option func(*Source)

// WithRoot overrides the procfs root (default "/proc").
func WithRoot(root string) Option { return func(s *Source) { s.root = root } }

// WithClockTicks overrides USER_HZ (default 100).
func WithClockTicks(hz int64) Option { return func(s *Source) { s.hz = hz } }

// New builds a Source and reads static system info (boot id, boot time).
func New(opts ...Option) (*Source, error) {
	s := &Source{root: "/proc", hz: 100, cpus: runtime.NumCPU(), pageSize: int64(os.Getpagesize())}
	for _, o := range opts {
		o(s)
	}
	if b, err := os.ReadFile(filepath.Join(s.root, "sys/kernel/random/boot_id")); err == nil {
		s.bootID = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile(filepath.Join(s.root, "stat")); err == nil {
		s.bootUnix = parseBtime(b)
	}
	s.hostname = readHostHostname(s.root)
	s.blkioAvail = detectDelayacct(s.root)
	return s, nil
}

// readHostHostname reads the host's own hostname from procfs
// (/proc/sys/kernel/hostname) so it honours a test's WithRoot, falling back to
// the os hostname when that read fails.
func readHostHostname(root string) string {
	if b, err := os.ReadFile(filepath.Join(root, "sys/kernel/hostname")); err == nil {
		if h := strings.TrimSpace(string(b)); h != "" {
			return h
		}
	}
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return ""
}

// detectDelayacct reports whether per-task block-I/O delay accounting is on.
// When kernel.task_delayacct is explicitly 0 the stat counter never advances, so
// blkio-delay must degrade to Disabled rather than read as a flat zero. A missing
// sysctl means older-kernel default-on, so we assume available.
func detectDelayacct(root string) model.Availability {
	b, err := os.ReadFile(filepath.Join(root, "sys/kernel/task_delayacct"))
	if err == nil && strings.TrimSpace(string(b)) == "0" {
		return model.Disabled
	}
	return model.Available
}

// rotateHostCache advances the hostname cache one generation: entries not touched
// (looked up) during the last List fall out, so the cache tracks live processes.
func (s *Source) rotateHostCache() {
	s.hostMu.Lock()
	s.hostCachePrev = s.hostCache
	s.hostCache = make(map[string]string, len(s.hostCachePrev))
	s.hostMu.Unlock()
}

// ID identifies the adapter.
func (s *Source) ID() string { return "procfs" }

// BootID returns the boot identifier.
func (s *Source) BootID() (string, error) { return s.bootID, nil }

// ClockTicksPerSec returns USER_HZ.
func (s *Source) ClockTicksPerSec() int64 { return s.hz }

// OnlineCPUs returns the CPU count.
func (s *Source) OnlineCPUs() int { return s.cpus }

// BootTimeUnix returns the system boot time.
func (s *Source) BootTimeUnix() int64 { return s.bootUnix }

// List enumerates processes. A process that vanishes mid-scan is skipped rather
// than failing the whole call (RFC §21.1).
func (s *Source) List(ctx context.Context) ([]ports.ProcStat, error) {
	if s.readHostname {
		s.rotateHostCache()
	}
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	out := make([]ports.ProcStat, 0, len(entries))
	for _, e := range entries {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		pid, ok := numericName(e.Name())
		if !ok {
			continue
		}
		st, ok := s.readProcess(pid)
		if ok {
			out = append(out, st)
		}
	}
	return out, nil
}

func numericName(name string) (int, bool) {
	n, err := strconv.Atoi(name)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}
