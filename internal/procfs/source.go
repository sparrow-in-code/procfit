package procfs

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/netikras/procfit/internal/ports"
)

// Source is the Linux /proc-backed ports.ProcessSource. Its root is
// configurable so tests can point it at a fixture tree instead of the live
// /proc (RFC §14.1). A future OS would be a separate adapter package
// implementing the same port (DEVELOPMENT.md §2.4).
type Source struct {
	root     string
	hz       int64
	cpus     int
	bootID   string
	bootUnix int64
	pageSize int64
}

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
	return s, nil
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
