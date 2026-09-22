package ports

import (
	"context"

	"github.com/netikras/procfit/internal/model"
)

// ProcIO holds raw cumulative I/O counters for a process (bytes/calls since
// process start). Individual fields may be unreadable; the enclosing ProcStat's
// IOAvail records that.
type ProcIO struct {
	ReadBytes           uint64
	WriteBytes          uint64
	RChar               uint64
	WChar               uint64
	Syscr               uint64
	Syscw               uint64
	CancelledWriteBytes uint64
}

// ProcStat is a raw, OS-neutral snapshot of one process's cheap attributes and
// cumulative counters as read by a ProcessSource at a single instant. The
// sampler converts cumulative counters into per-second rate metrics by diffing
// consecutive generations; ProcStat itself carries no rates.
//
// Availability fields record when a group of values could not be read (e.g.
// permission denied on /proc/PID/io), so downstream never turns missing data
// into zero (RFC §6.4).
type ProcStat struct {
	ID ProcessInstanceIdentity

	PID, PPID, PGID, SID, TGID int
	UID, EUID, GID, EGID       uint32

	Comm         string
	Cmdline      []string
	CmdlineAvail model.Availability
	Exe          string
	ExeAvail     model.Availability
	Cwd          string
	CgroupPath   string

	State      model.ProcessState
	Nice       int
	NiceAvail  model.Availability
	NumThreads int
	StartTicks uint64

	// Cumulative CPU (clock ticks) and fault/ctxsw counters.
	UTimeTicks      uint64
	STimeTicks      uint64
	MinFlt          uint64
	MajFlt          uint64
	VoluntaryCtxt   uint64
	InvoluntaryCtxt uint64

	// BlkioTicks is cumulative block-I/O delay (clock ticks, /proc/stat field 42);
	// BlkioAvail is Disabled when kernel delay accounting is off (so the derived
	// blkio-delay is unavailable, not a misleading zero).
	BlkioTicks uint64
	BlkioAvail model.Availability

	// Memory (bytes).
	RSSBytes uint64
	VSZBytes uint64
	// Memory breakdown from /proc/PID/status (bytes): peak RSS (VmHWM), the
	// anon/file/shmem split of resident memory, and swapped-out size (VmSwap).
	RSSPeakBytes  uint64
	RSSAnonBytes  uint64
	RSSFileBytes  uint64
	RSSShmemBytes uint64
	SwapBytes     uint64

	// I/O counters and their availability.
	IO      ProcIO
	IOAvail model.Availability

	Namespaces model.NamespaceSet

	// Threads is populated only when the source is asked to enumerate them (leaf
	// thread mode). Empty otherwise, so the common path pays no per-thread cost.
	Threads []ThreadStat
}

// ThreadStat is a raw snapshot of one thread (task) within a process: identity
// plus the cumulative CPU counters needed to derive per-thread cpu rates.
type ThreadStat struct {
	TID        int
	StartTicks uint64
	Comm       string
	State      model.ProcessState
	UTimeTicks uint64
	STimeTicks uint64
}

// ProcessInstanceIdentity aliases the model identity so adapters need not import
// query/collect packages; it keeps the dependency arrow pointing inward.
type ProcessInstanceIdentity = model.ProcessInstanceID

// ProcessSource is the platform port for enumerating processes and reading their
// cheap attributes/counters. The Linux adapter (internal/procfs) reads /proc; a
// future OS is a sibling adapter implementing this same interface — no core
// changes (DEVELOPMENT.md §2.4).
type ProcessSource interface {
	// ID names the adapter (e.g. "procfs").
	ID() string
	// BootID returns a stable per-boot identifier used to scope identity.
	BootID() (string, error)
	// ClockTicksPerSec is USER_HZ, for converting CPU ticks to seconds.
	ClockTicksPerSec() int64
	// OnlineCPUs returns the number of online CPUs for cpu-normalized.
	OnlineCPUs() int
	// BootTimeUnix returns the wall-clock unix time (seconds) of system boot,
	// used to derive process age from start ticks. Returns 0 if unknown.
	BootTimeUnix() int64
	// List returns a raw snapshot of all visible processes at this instant.
	// A process vanishing mid-scan is a normal outcome and is simply omitted
	// (RFC §21.1); it must not fail the whole call.
	List(ctx context.Context) ([]ProcStat, error)
}
