package model

import "fmt"

// ProcessInstanceID is the canonical identity of a running process. A numeric
// PID alone is NEVER identity (RFC §6.3, §31.5): PIDs are reused, so a stale
// reference must never resolve to a different process. Identity is scoped by
// boot and PID namespace and pinned by the kernel start time.
type ProcessInstanceID struct {
	// BootID from /proc/sys/kernel/random/boot_id; scopes identity to one boot.
	BootID string
	// PIDNSInode is the inode of the process's PID namespace.
	PIDNSInode uint64
	// PID is the process id within that namespace view.
	PID int
	// StartTime is field 22 of /proc/<pid>/stat (clock ticks since boot). Two
	// processes with the same PID but different start times are different
	// instances.
	StartTime uint64
}

// Zero reports whether the id is unset.
func (id ProcessInstanceID) Zero() bool {
	return id == ProcessInstanceID{}
}

// SameProcess reports whether two ids refer to the same process instance.
// Equality requires PID and StartTime to match; BootID and PIDNSInode further
// scope identity when known (non-empty/non-zero).
func (id ProcessInstanceID) SameProcess(other ProcessInstanceID) bool {
	if id.PID != other.PID || id.StartTime != other.StartTime {
		return false
	}
	if id.BootID != "" && other.BootID != "" && id.BootID != other.BootID {
		return false
	}
	if id.PIDNSInode != 0 && other.PIDNSInode != 0 && id.PIDNSInode != other.PIDNSInode {
		return false
	}
	return true
}

// Key returns a stable string key suitable for maps and cross-generation
// correlation within a boot.
func (id ProcessInstanceID) Key() string {
	return fmt.Sprintf("%s/%d/%d/%d", id.BootID, id.PIDNSInode, id.PID, id.StartTime)
}

// String is a compact human form.
func (id ProcessInstanceID) String() string {
	return fmt.Sprintf("pid:%d@%d", id.PID, id.StartTime)
}

// ThreadInstanceID identifies a thread within a process (RFC §5, §11).
type ThreadInstanceID struct {
	Process   ProcessInstanceID
	TID       int
	StartTime uint64
}

// Zero reports whether the id is unset.
func (t ThreadInstanceID) Zero() bool { return t == ThreadInstanceID{} }

// SameThread reports whether two thread ids refer to the same thread instance.
func (t ThreadInstanceID) SameThread(other ThreadInstanceID) bool {
	return t.TID == other.TID && t.StartTime == other.StartTime && t.Process.SameProcess(other.Process)
}

// Key returns a stable string key for maps.
func (t ThreadInstanceID) Key() string {
	return fmt.Sprintf("%s#%d/%d", t.Process.Key(), t.TID, t.StartTime)
}

// String is a compact human form.
func (t ThreadInstanceID) String() string {
	return fmt.Sprintf("tid:%d/%d@%d", t.Process.PID, t.TID, t.StartTime)
}
