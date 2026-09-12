package model

// ProcessState models the kernel process state as a structured value rather than
// an opaque string, so filters can address fields while renderers compose a
// familiar ps-style summary (RFC §11.3).
type ProcessState struct {
	// Code is the single-letter kernel state: R,S,D,T,t,Z,X,I,...
	Code rune
}

// Known single-letter Linux process state codes.
const (
	StateRunning     = 'R'
	StateSleeping    = 'S'
	StateDiskSleep   = 'D'
	StateStopped     = 'T'
	StateTracingStop = 't'
	StateZombie      = 'Z'
	StateDead        = 'X'
	StateIdle        = 'I'
)

// Description returns a human label for the state code.
func (s ProcessState) Description() string {
	switch s.Code {
	case StateRunning:
		return "running"
	case StateSleeping:
		return "sleeping"
	case StateDiskSleep:
		return "disk-sleep"
	case StateStopped:
		return "stopped"
	case StateTracingStop:
		return "tracing-stop"
	case StateZombie:
		return "zombie"
	case StateDead:
		return "dead"
	case StateIdle:
		return "idle"
	case 0:
		return "unknown"
	default:
		return string(s.Code)
	}
}

// Stopped reports whether the observed kernel state is a stopped state. Note
// this is the OBSERVED state, distinct from procfit's own stop intent (RFC
// §15.4).
func (s ProcessState) Stopped() bool {
	return s.Code == StateStopped || s.Code == StateTracingStop
}

// ProcessFlags are derived boolean attributes (RFC §11.3). Renderers compose a
// PSTATE column from state + flags; filters address the structured booleans.
type ProcessFlags struct {
	SessionLeader bool
	Multithreaded bool
	Foreground    bool
	HighPriority  bool
	LowPriority   bool
	LockedMemory  bool
}
