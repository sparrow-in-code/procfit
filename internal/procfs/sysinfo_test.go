package procfs

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/netikras/procfit/internal/model"
)

// All trees below are fixtures under t.TempDir(); nothing reads the host.

func near(v, want float64) bool { return v > want-0.01 && v < want+0.01 }

func TestLoadAvgCollector(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "loadavg"), "0.52 1.25 2.00 2/1234 5678\n")
	hm := NewLoadAvgCollector(root).CollectHost(context.Background())
	for id, want := range map[model.MetricID]float64{"load-1": 0.52, "load-5": 1.25, "load-15": 2.00} {
		if v := hm[id]; !v.Present() || !near(v.V, want) {
			t.Fatalf("%s = %+v, want %v", id, v, want)
		}
	}
	// Missing file → unavailable, never zero.
	if v := NewLoadAvgCollector(t.TempDir()).CollectHost(context.Background())["load-1"]; v.Present() {
		t.Fatalf("missing loadavg must be unavailable, got %+v", v)
	}
	// Garbage → read error.
	bad := t.TempDir()
	mustWrite(t, filepath.Join(bad, "loadavg"), "nope\n")
	if v := NewLoadAvgCollector(bad).CollectHost(context.Background())["load-5"]; v.Availability != model.ReadError {
		t.Fatalf("malformed loadavg should be read_error, got %+v", v)
	}
}

func writeBat(t *testing.T, root, name string, fields map[string]string) {
	t.Helper()
	for k, v := range fields {
		mustWrite(t, filepath.Join(root, "class/power_supply", name, k), v+"\n")
	}
}

func TestBatteryCollector_ChargeStyleDischarging(t *testing.T) {
	root := t.TempDir()
	writeBat(t, root, "BAT0", map[string]string{
		"type": "Battery", "status": "Discharging",
		"charge_now": "2500000", "charge_full": "5000000", "current_now": "1000000", // µAh, µAh, µA
	})
	writeBat(t, root, "AC", map[string]string{"type": "Mains"}) // ignored
	hm := NewBatteryCollector(root).CollectHost(context.Background())

	if v := hm["battery-pct"]; !v.Present() || !near(v.V, 50) {
		t.Fatalf("battery-pct = %+v, want 50", v)
	}
	if v := hm["battery-charge-left"]; !v.Present() || !near(v.V, 2500) {
		t.Fatalf("battery-charge-left = %+v, want 2500 mAh", v)
	}
	// 2500 mAh / 1000 mA = 2.5 h = 9000 s to empty.
	if v := hm["battery-time-left"]; !v.Present() || !near(v.V, 9000) {
		t.Fatalf("battery-time-left = %+v, want 9000s", v)
	}
	// Charge-style driver: energy unavailable (not zero).
	if v := hm["battery-energy-left"]; v.Present() {
		t.Fatalf("energy-left must be unavailable for a charge-style battery, got %+v", v)
	}
}

func TestBatteryCollector_EnergyStyleCharging(t *testing.T) {
	root := t.TempDir()
	writeBat(t, root, "BAT0", map[string]string{
		"type": "Battery", "status": "Charging",
		"energy_now": "30000000", "energy_full": "60000000", "power_now": "15000000", // µWh, µWh, µW
	})
	hm := NewBatteryCollector(root).CollectHost(context.Background())
	if v := hm["battery-energy-left"]; !v.Present() || !near(v.V, 30) {
		t.Fatalf("battery-energy-left = %+v, want 30 Wh", v)
	}
	// Charging: (60-30) Wh / 15 W = 2 h = 7200 s to full.
	if v := hm["battery-time-left"]; !v.Present() || !near(v.V, 7200) {
		t.Fatalf("battery-time-left (to full) = %+v, want 7200s", v)
	}
	if v := hm["battery-charge-left"]; v.Present() {
		t.Fatalf("charge-left must be unavailable for an energy-style battery, got %+v", v)
	}
}

func TestBatteryCollector_TwoBatteriesAndFallbacks(t *testing.T) {
	root := t.TempDir()
	writeBat(t, root, "BAT0", map[string]string{"type": "Battery", "status": "Full",
		"charge_now": "1000000", "charge_full": "2000000"})
	writeBat(t, root, "BAT1", map[string]string{"type": "Battery", "status": "Full",
		"charge_now": "3000000", "charge_full": "4000000"})
	hm := NewBatteryCollector(root).CollectHost(context.Background())
	// Combined: 4000/6000 = 66.7%; full/idle → no time-left.
	if v := hm["battery-pct"]; !v.Present() || !near(v.V, 66.67) {
		t.Fatalf("combined battery-pct = %+v, want ~66.67", v)
	}
	if v := hm["battery-time-left"]; v.Present() {
		t.Fatalf("time-left must be unavailable when not (dis)charging, got %+v", v)
	}
	// capacity-only driver → level from capacity %.
	capRoot := t.TempDir()
	writeBat(t, capRoot, "BAT0", map[string]string{"type": "Battery", "status": "Unknown", "capacity": "77"})
	if v := NewBatteryCollector(capRoot).CollectHost(context.Background())["battery-pct"]; !v.Present() || !near(v.V, 77) {
		t.Fatalf("capacity fallback = %+v, want 77", v)
	}
	// No battery at all → everything unavailable.
	for id, v := range NewBatteryCollector(t.TempDir()).CollectHost(context.Background()) {
		if v.Present() {
			t.Fatalf("no battery: %s must be unavailable, got %+v", id, v)
		}
	}
}

func TestTempCollector_HwmonSensors(t *testing.T) {
	root := t.TempDir()
	hw := filepath.Join(root, "class/hwmon")
	mustWrite(t, filepath.Join(hw, "hwmon0/name"), "coretemp\n")
	mustWrite(t, filepath.Join(hw, "hwmon0/temp1_input"), "61000\n")
	mustWrite(t, filepath.Join(hw, "hwmon0/temp1_label"), "Package id 0\n")
	mustWrite(t, filepath.Join(hw, "hwmon0/temp2_input"), "58000\n")
	mustWrite(t, filepath.Join(hw, "hwmon0/temp2_label"), "Core 0\n")
	mustWrite(t, filepath.Join(hw, "hwmon1/name"), "nvme\n")
	mustWrite(t, filepath.Join(hw, "hwmon1/temp1_input"), "72500\n") // no label → temp1
	mustWrite(t, filepath.Join(hw, "hwmon2/name"), "nvme\n")         // a second NVMe drive
	mustWrite(t, filepath.Join(hw, "hwmon2/temp1_input"), "40000\n")

	c := NewTempCollector(root)
	got := map[model.MetricID]bool{}
	for _, s := range c.Sensors() {
		got[s.ID] = true
	}
	for _, want := range []model.MetricID{
		"temp-coretemp-package-id-0", "temp-coretemp-core-0", "temp-nvme-temp1", "temp-nvme-temp1-2",
	} {
		if !got[want] {
			t.Fatalf("sensor %q not discovered; got %v", want, got)
		}
	}
	hm := c.CollectHost(context.Background())
	if v := hm["temp-coretemp-package-id-0"]; !v.Present() || !near(v.V, 61) {
		t.Fatalf("package temp = %+v, want 61", v)
	}
	// temp-cpu = hottest CPU-chip sensor (coretemp 61), temp-max = hottest overall (nvme 72.5).
	if v := hm["temp-cpu"]; !v.Present() || !near(v.V, 61) {
		t.Fatalf("temp-cpu = %+v, want 61", v)
	}
	if v := hm["temp-max"]; !v.Present() || !near(v.V, 72.5) {
		t.Fatalf("temp-max = %+v, want 72.5", v)
	}
}

func TestTempCollector_ThermalFallbackAndNone(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "class/thermal/thermal_zone0/type"), "x86_pkg_temp\n")
	mustWrite(t, filepath.Join(root, "class/thermal/thermal_zone0/temp"), "55000\n")
	hm := NewTempCollector(root).CollectHost(context.Background())
	if v := hm["temp-x86-pkg-temp"]; !v.Present() || !near(v.V, 55) {
		t.Fatalf("thermal zone temp = %+v, want 55", v)
	}
	if v := hm["temp-cpu"]; !v.Present() || !near(v.V, 55) {
		t.Fatalf("x86_pkg_temp should count as CPU, got %+v", v)
	}
	// No sensors anywhere → curated temps unavailable, no per-sensor ids.
	none := NewTempCollector(t.TempDir())
	if len(none.Sensors()) != 0 {
		t.Fatal("empty sysfs must discover no sensors")
	}
	if v := none.CollectHost(context.Background())["temp-max"]; v.Present() {
		t.Fatalf("no sensors: temp-max must be unavailable, got %+v", v)
	}
}

func TestSlugAndTempID(t *testing.T) {
	if got := tempID("coretemp", "Package id 0"); got != "temp-coretemp-package-id-0" {
		t.Fatalf("tempID = %q", got)
	}
	if got := tempID("acpitz", ""); got != "temp-acpitz" {
		t.Fatalf("tempID without label = %q", got)
	}
	if got := slug("  Tctl / Tdie!! "); got != "tctl-tdie" {
		t.Fatalf("slug = %q", got)
	}
}
