package ports

// Signal is an OS-neutral signal identifier for the controller port.
type Signal string

const (
	SigStop Signal = "STOP"
	SigCont Signal = "CONT"
	SigTerm Signal = "TERM"
	SigKill Signal = "KILL"
	SigHUP  Signal = "HUP"
	SigINT  Signal = "INT"
)

// ControlCaps reports which control operations the backend supports under the
// current platform and privileges (RFC §14.3, §20).
type ControlCaps struct {
	Nice   bool
	Signal bool
	Freeze bool
}

// Controller is the platform port for mutating processes. The Linux adapter uses
// setpriority/kill (and pidfd where available); a future OS is a sibling adapter
// implementing this same interface. Observation never depends on it (control is
// a side plane, RFC §6.2).
type Controller interface {
	Capabilities() ControlCaps
	// GetNice returns the current nice value for a pid.
	GetNice(pid int) (int, error)
	// SetNice sets the nice value for a pid.
	SetNice(pid, nice int) error
	// SendSignal delivers a signal to a pid.
	SendSignal(pid int, sig Signal) error
	// ReadIdentity re-reads a process's canonical identity for revalidation
	// immediately before acting (RFC §21.2). ok is false if the pid is gone.
	ReadIdentity(pid int) (ProcessInstanceIdentity, bool)
	// ReadCgroupOf returns the cgroup v2 relative path of a pid (from
	// /proc/PID/cgroup "0::<path>"). ok is false if unknown.
	ReadCgroupOf(pid int) (string, bool)
	// FreezeCgroup sets the frozen state of an existing, writable cgroup by its
	// v2 relative path (RFC §15.5). It only operates on an existing delegated
	// cgroup; it never creates or migrates cgroups.
	FreezeCgroup(cgroupRelPath string, freeze bool) error
}
