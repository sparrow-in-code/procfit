package model

// BatteryState is the host battery's charging state, carried as a numeric metric
// code (so it sorts, filters and exports like any metric) and rendered as a
// label. Codes are a stable public mapping — append new states, never renumber.
type BatteryState int

const (
	BatteryUnknown     BatteryState = 0 // driver reports "Unknown" or nothing usable
	BatteryCharging    BatteryState = 1
	BatteryDischarging BatteryState = 2
	BatteryNotCharging BatteryState = 3 // on AC but held (charge threshold / limit)
	BatteryFull        BatteryState = 4
	BatteryMissing     BatteryState = 5 // a battery slot is present but empty
	BatteryMixed       BatteryState = 6 // several batteries disagree
)

var batteryStateLabels = map[BatteryState]string{
	BatteryUnknown:     "unknown",
	BatteryCharging:    "charging",
	BatteryDischarging: "discharging",
	BatteryNotCharging: "not-charging",
	BatteryFull:        "full",
	BatteryMissing:     "missing",
	BatteryMixed:       "mixed",
}

// Label returns the human label for the state.
func (s BatteryState) Label() string {
	if l, ok := batteryStateLabels[s]; ok {
		return l
	}
	return "unknown"
}

// BatteryStateFromStatus maps a power_supply `status` string to a state.
func BatteryStateFromStatus(status string) BatteryState {
	switch status {
	case "Charging":
		return BatteryCharging
	case "Discharging":
		return BatteryDischarging
	case "Not charging":
		return BatteryNotCharging
	case "Full":
		return BatteryFull
	default:
		return BatteryUnknown
	}
}

// enumLabels maps enum-valued metrics (UnitEnum) to their code → label tables, so
// renderers can show a label without knowing each metric. Adding an enum metric
// is one entry here.
var enumLabels = map[MetricID]func(int) string{
	"battery-state": func(code int) string { return BatteryState(code).Label() },
}

// EnumLabel returns the label for an enum metric's numeric code; ok=false when id
// is not an enum metric.
func EnumLabel(id MetricID, code float64) (string, bool) {
	fn, ok := enumLabels[id]
	if !ok {
		return "", false
	}
	return fn(int(code)), true
}
