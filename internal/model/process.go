package model

// MetricID is the stable, kebab-case public identifier of a metric (RFC §13).
type MetricID string

// MetricValue is a numeric metric reading with availability/quality metadata.
// Non-numeric attributes (process state, nice) are modelled as structured
// fields on Process rather than forced into this map.
type MetricValue = Value[float64]

// Process is the normalized, OS-agnostic process entity (RFC §11.1). Fields that
// may be permission-restricted carry availability via CmdlineState etc.; an
// empty Cmdline is distinguished from an unreadable one.
type Process struct {
	ID ProcessInstanceID

	PID, TGID, PPID, PGID, SID int
	UID, EUID, GID, EGID       uint32

	Comm         string
	User         string
	Cmdline      []string
	CmdlineAvail Availability
	Exe          string
	Cwd          string
	CgroupPath   string
	SystemdUnit  string
	AppID        string
	ContainerID  string
	PodUID       string

	Namespaces NamespaceSet
	State      ProcessState
	Flags      ProcessFlags

	// Nice is the observed nice value; NiceAvail records whether it was read.
	Nice      int
	NiceAvail Availability

	// NumThreads is the observed thread count (used for the P/T count column
	// even when threads are not individually enumerated).
	NumThreads int

	// StartTicks is the raw starttime (field 22) in clock ticks since boot.
	StartTicks uint64

	// Metrics holds numeric metric samples keyed by canonical id.
	Metrics map[MetricID]MetricValue

	// Threads holds enumerated threads when leaf/thread mode requires it.
	Threads []Thread
}

// Thread is a normalized thread entity.
type Thread struct {
	ID      ThreadInstanceID
	Comm    string
	State   ProcessState
	Metrics map[MetricID]MetricValue
}

// Metric returns a metric value, or an Unavailable placeholder (never a real
// zero) when the process has no reading for it (RFC §6.4).
func (p *Process) Metric(id MetricID) MetricValue {
	if p.Metrics != nil {
		if v, ok := p.Metrics[id]; ok {
			return v
		}
	}
	return Unavailable[float64](Disabled, "")
}

// SetMetric records a metric value, allocating the map on first use.
func (p *Process) SetMetric(id MetricID, v MetricValue) {
	if p.Metrics == nil {
		p.Metrics = make(map[MetricID]MetricValue)
	}
	p.Metrics[id] = v
}
