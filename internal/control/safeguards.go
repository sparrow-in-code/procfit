package control

import "fmt"

// Safeguards refuses control of processes that must not be touched by default
// (RFC §15.9). It is intentionally conservative; --force-system (not wired here)
// would relax the software checks but can never bypass kernel permissions.
type Safeguards struct {
	SelfPID   int
	SelfPPID  int
	SafetyMax int // max instances without confirmation (0 = unlimited)
}

// NewSafeguards builds safeguards for the current process.
func NewSafeguards(selfPID, selfPPID int) Safeguards {
	return Safeguards{SelfPID: selfPID, SelfPPID: selfPPID, SafetyMax: 256}
}

// CheckPID reports whether a pid may be controlled, and if not, why.
func (s Safeguards) CheckPID(pid int) (bool, string) {
	switch {
	case pid <= 1:
		return false, "refusing to control PID 1 / invalid pid"
	case pid == s.SelfPID:
		return false, "refusing to control the procfit process itself"
	case s.SelfPPID != 0 && pid == s.SelfPPID:
		return false, "refusing to control procfit's parent process"
	default:
		return true, ""
	}
}

// CheckCount validates a resolved instance count against the safety limit.
func (s Safeguards) CheckCount(n int, confirmed bool) error {
	if s.SafetyMax > 0 && n > s.SafetyMax && !confirmed {
		return fmt.Errorf("target resolves to %d instances (limit %d); confirm to proceed", n, s.SafetyMax)
	}
	return nil
}
