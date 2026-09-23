package procfs

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/ports"
)

// NewPowerSources returns every power backend present on this host, in priority
// order: powercap/RAPL (Intel & AMD), hwmon sensors, then the laptop battery
// (PM-0506). The collector MERGES their domains — RAPL/hwmon supply
// pkg/core/dram, the battery supplies whole-system draw — so a laptop with both
// reports all of them, and a domain one backend can't read is still filled by
// another that can. A backend that is absent is omitted; one present but
// currently unreadable is still returned so its reason surfaces.
func NewPowerSources(sysRoot string) []ports.PowerSource {
	if sysRoot == "" {
		sysRoot = "/sys"
	}
	candidates := []ports.PowerSource{
		&powercapSource{root: sysRoot},
		&hwmonSource{root: sysRoot},
		&batterySource{root: sysRoot},
	}
	var out []ports.PowerSource
	for _, s := range candidates {
		if _, ok := s.Read(); ok {
			out = append(out, s)
		}
	}
	return out
}

// powercapSource reads /sys/class/powercap/intel-rapl:*/{name,energy_uj}. The
// powercap tree is shared by Intel and modern AMD, so no vendor string is
// assumed; the domain comes from the `name` file.
type powercapSource struct{ root string }

func (s *powercapSource) ID() string { return "powercap" }

func (s *powercapSource) Read() ([]ports.PowerReading, bool) {
	base := filepath.Join(s.root, "class/powercap")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, false
	}
	var out []ports.PowerReading
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "intel-rapl:") {
			continue
		}
		dom := canonicalRAPLDomain(readTrimFile(filepath.Join(base, name, "name")))
		if dom == "" {
			continue
		}
		uj, ok := readUintFile(filepath.Join(base, name, "energy_uj"))
		av := model.Available
		if !ok {
			av = model.PermissionDenied // energy_uj is root-only on many kernels
		}
		out = append(out, ports.PowerReading{Domain: dom, Value: uj, Kind: ports.EnergyCounter, Avail: av})
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// canonicalRAPLDomain maps a RAPL `name` (package-0, core, uncore, dram, psys) to
// a canonical domain, or "" to skip (psys overlaps package).
func canonicalRAPLDomain(name string) string {
	switch {
	case strings.HasPrefix(name, "package"):
		return "pkg"
	case name == "core":
		return "core"
	case name == "uncore":
		return "uncore"
	case name == "dram":
		return "dram"
	default:
		return ""
	}
}

// hwmonSource reads /sys/class/hwmon/hwmon*/{power,energy}1_input — broad vendor
// coverage (incl. amd_energy and platform sensors). power*_input is instantaneous
// microwatts; energy*_input is cumulative microjoules.
type hwmonSource struct{ root string }

func (s *hwmonSource) ID() string { return "hwmon" }

func (s *hwmonSource) Read() ([]ports.PowerReading, bool) {
	base := filepath.Join(s.root, "class/hwmon")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, false
	}
	var out []ports.PowerReading
	for _, e := range entries {
		dir := filepath.Join(base, e.Name())
		if uw, ok := readUintFile(filepath.Join(dir, "power1_input")); ok {
			out = append(out, ports.PowerReading{Domain: "pkg", Value: uw, Kind: ports.InstantPower, Avail: model.Available})
			continue
		}
		if uj, ok := readUintFile(filepath.Join(dir, "energy1_input")); ok {
			out = append(out, ports.PowerReading{Domain: "pkg", Value: uj, Kind: ports.EnergyCounter, Avail: model.Available})
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// batterySource reads /sys/class/power_supply/*/ for a battery's power draw:
// power_now (microwatts), else voltage_now × current_now. This is the
// vendor/arch-neutral whole-system draw on any laptop (Intel/AMD/ARM), valid on
// discharge.
type batterySource struct{ root string }

func (s *batterySource) ID() string { return "battery" }

func (s *batterySource) Read() ([]ports.PowerReading, bool) {
	base := filepath.Join(s.root, "class/power_supply")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, false
	}
	for _, e := range entries {
		dir := filepath.Join(base, e.Name())
		if readTrimFile(filepath.Join(dir, "type")) != "Battery" {
			continue
		}
		if uw, ok := batteryMicrowatts(dir); ok {
			return []ports.PowerReading{{Domain: "system", Value: uw, Kind: ports.InstantPower, Avail: model.Available}}, true
		}
		// A battery is present but its power is not readable right now (e.g. full/AC).
		return []ports.PowerReading{{Domain: "system", Kind: ports.InstantPower, Avail: model.WarmingUp}}, true
	}
	return nil, false
}

// batteryMicrowatts returns the battery's instantaneous draw in microwatts, from
// power_now or voltage_now(µV) × current_now(µA).
func batteryMicrowatts(dir string) (uint64, bool) {
	if uw, ok := readUintFile(filepath.Join(dir, "power_now")); ok {
		return uw, true
	}
	uv, okv := readUintFile(filepath.Join(dir, "voltage_now"))
	ua, okc := readUintFile(filepath.Join(dir, "current_now"))
	if okv && okc {
		// mV × mA = µW (1e-3 V × 1e-3 A = 1e-6 W).
		return (uv / 1000) * (ua / 1000), true
	}
	return 0, false
}

// readTrimFile reads a sysfs file and trims whitespace, "" on error.
func readTrimFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
