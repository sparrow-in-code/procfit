package model

import "testing"

func TestBatteryStateLabelsAndStatus(t *testing.T) {
	for s, want := range map[BatteryState]string{
		BatteryUnknown: "unknown", BatteryCharging: "charging", BatteryDischarging: "discharging",
		BatteryNotCharging: "not-charging", BatteryFull: "full", BatteryMissing: "missing", BatteryMixed: "mixed",
		BatteryState(99): "unknown",
	} {
		if got := s.Label(); got != want {
			t.Fatalf("Label(%d) = %q, want %q", s, got, want)
		}
	}
	if BatteryStateFromStatus("Not charging") != BatteryNotCharging || BatteryStateFromStatus("weird") != BatteryUnknown {
		t.Fatal("status mapping wrong")
	}
	if l, ok := EnumLabel("battery-state", float64(BatteryCharging)); !ok || l != "charging" {
		t.Fatalf("EnumLabel = %q,%v", l, ok)
	}
	if _, ok := EnumLabel("cpu", 1); ok {
		t.Fatal("non-enum metric must not resolve a label")
	}
}
