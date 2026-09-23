package ports

import "github.com/netikras/procfit/internal/model"

// PowerKind distinguishes a cumulative energy counter (which needs a delta over
// time to yield watts) from an instantaneous power reading.
type PowerKind int

const (
	// EnergyCounter is a cumulative energy value in microjoules.
	EnergyCounter PowerKind = iota
	// InstantPower is an instantaneous power value in microwatts.
	InstantPower
)

// PowerReading is one energy/power domain read from a PowerSource.
type PowerReading struct {
	// Domain is the canonical domain: pkg, core, uncore, dram, or system.
	Domain string
	// Value is microjoules (EnergyCounter) or microwatts (InstantPower).
	Value uint64
	Kind  PowerKind
	Avail model.Availability
}

// PowerSource reads whole-system/socket energy or power. Vendor and arch
// specifics — Intel/AMD RAPL via the powercap tree, hwmon sensors, or the laptop
// battery — live entirely in the adapter (RFC §13/§24); the core selects the
// first available backend and derives watts, hardcoding no vendor string. Read
// is cheap (no sleep): the collector owns any measurement window.
type PowerSource interface {
	// ID names the backend (powercap/hwmon/battery), for capability reporting.
	ID() string
	// Read returns the current per-domain readings; ok=false when the backend is
	// not present on this host.
	Read() (readings []PowerReading, ok bool)
}
