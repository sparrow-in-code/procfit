package procfs

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/netikras/procfit/internal/model"
)

// TempSensor is one detected temperature input.
type TempSensor struct {
	ID    model.MetricID // stable metric id, e.g. temp-coretemp-package-id-0
	Chip  string         // hwmon chip / thermal zone type, e.g. coretemp, nvme
	Label string         // sensor label, e.g. "Package id 0" (may be empty)
	path  string         // the *_input / temp file (millidegrees C)
}

// cpuChips are hwmon/thermal names that identify a CPU temperature source; the
// hottest of them is temp-cpu. Read from sysfs, never assumed present.
var cpuChips = map[string]bool{
	"coretemp": true, "k10temp": true, "zenpower": true, "cpu_thermal": true,
	"x86_pkg_temp": true, "cpu-thermal": true, "soc_thermal": true,
}

// TempCollector reads temperatures from hwmon (/sys/class/hwmon/*/temp*_input),
// falling back to thermal zones (/sys/class/thermal/thermal_zone*/temp) when
// hwmon exposes none. It reports the curated temp-cpu / temp-max plus one metric
// per detected sensor (whatever this machine has). Host-scoped.
type TempCollector struct {
	root    string
	sensors []TempSensor
}

// NewTempCollector discovers sensors under the given sysfs root (default /sys).
func NewTempCollector(root string) *TempCollector {
	if root == "" {
		root = "/sys"
	}
	return &TempCollector{root: root, sensors: discoverTemps(root)}
}

// ID names the source.
func (c *TempCollector) ID() string { return "temp" }

// Sensors returns the discovered sensors (for registering their descriptors).
func (c *TempCollector) Sensors() []TempSensor { return c.sensors }

// Metrics lists temp-cpu, temp-max, and every discovered sensor id.
func (c *TempCollector) Metrics() []model.MetricID {
	ids := []model.MetricID{"temp-cpu", "temp-max"}
	for _, s := range c.sensors {
		ids = append(ids, s.ID)
	}
	return ids
}

// CollectHost reads every sensor (°C) and derives temp-cpu / temp-max.
func (c *TempCollector) CollectHost(_ context.Context) map[model.MetricID]model.MetricValue {
	out := map[model.MetricID]model.MetricValue{
		"temp-cpu": unavailable("temp"),
		"temp-max": unavailable("temp"),
	}
	var maxAll, maxCPU float64
	haveAll, haveCPU := false, false
	for _, s := range c.sensors {
		milli, ok := readIntFile(s.path)
		if !ok {
			out[s.ID] = model.Unavailable[float64](model.ReadError, "temp")
			continue
		}
		deg := float64(milli) / 1000
		out[s.ID] = model.NewValue(deg, model.Exact, "temp")
		if !haveAll || deg > maxAll {
			maxAll, haveAll = deg, true
		}
		if cpuChips[s.Chip] && (!haveCPU || deg > maxCPU) {
			maxCPU, haveCPU = deg, true
		}
	}
	if haveAll {
		out["temp-max"] = model.NewValue(maxAll, model.Derived, "temp")
	}
	if haveCPU {
		out["temp-cpu"] = model.NewValue(maxCPU, model.Derived, "temp")
	}
	return out
}

// discoverTemps finds hwmon temperature inputs, or thermal zones when hwmon has
// none, and assigns each a unique, stable metric id.
func discoverTemps(root string) []TempSensor {
	sensors := discoverHwmonTemps(root)
	if len(sensors) == 0 {
		sensors = discoverThermalZones(root)
	}
	sort.Slice(sensors, func(i, j int) bool { return sensors[i].ID < sensors[j].ID })
	return dedupeIDs(sensors)
}

func discoverHwmonTemps(root string) []TempSensor {
	base := filepath.Join(root, "class/hwmon")
	chips, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var out []TempSensor
	for _, chip := range chips {
		dir := filepath.Join(base, chip.Name())
		name := readTrimFile(filepath.Join(dir, "name"))
		if name == "" {
			name = chip.Name()
		}
		inputs, _ := filepath.Glob(filepath.Join(dir, "temp*_input"))
		for _, in := range inputs {
			n := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(in), "temp"), "_input")
			label := readTrimFile(filepath.Join(dir, "temp"+n+"_label"))
			suffix := label
			if suffix == "" {
				suffix = "temp" + n
			}
			out = append(out, TempSensor{ID: tempID(name, suffix), Chip: name, Label: label, path: in})
		}
	}
	return out
}

func discoverThermalZones(root string) []TempSensor {
	zones, _ := filepath.Glob(filepath.Join(root, "class/thermal/thermal_zone*"))
	var out []TempSensor
	for _, z := range zones {
		typ := readTrimFile(filepath.Join(z, "type"))
		if typ == "" {
			typ = filepath.Base(z)
		}
		out = append(out, TempSensor{ID: tempID(typ, ""), Chip: typ, path: filepath.Join(z, "temp")})
	}
	return out
}

// tempID builds "temp-<chip>[-<label>]", lowercased, non-alphanumerics → '-'.
func tempID(chip, label string) model.MetricID {
	s := "temp-" + slug(chip)
	if l := slug(label); l != "" {
		s += "-" + l
	}
	return model.MetricID(s)
}

func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// dedupeIDs suffixes repeated ids (-2, -3, …) so two identical chips/labels
// (e.g. two NVMe drives) stay distinct. Input is pre-sorted for stable numbering.
func dedupeIDs(in []TempSensor) []TempSensor {
	seen := map[model.MetricID]int{}
	for i := range in {
		base := in[i].ID
		seen[base]++
		if n := seen[base]; n > 1 {
			in[i].ID = model.MetricID(string(base) + "-" + strconv.Itoa(n))
		}
	}
	return in
}

// Description is a human description for a discovered sensor's metric.
func (s TempSensor) Description() string {
	if s.Label != "" {
		return "temperature °C: " + s.Chip + " " + s.Label
	}
	return "temperature °C: " + s.Chip
}
