package procfs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeCstateTime(t *testing.T, root string, cpu, state int, us uint64) {
	t.Helper()
	dir := filepath.Join(root, "devices/system/cpu",
		"cpu"+itoa(cpu), "cpuidle", "state"+itoa(state))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "time"), []byte(itoa(int(us))+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCstateHelpers(t *testing.T) {
	if !isCPUDir("cpu0") || !isCPUDir("cpu12") {
		t.Fatal("cpuN must be a CPU dir")
	}
	if isCPUDir("cpufreq") || isCPUDir("cpuidle") || isCPUDir("cpu") {
		t.Fatal("cpufreq/cpuidle/cpu must not be CPU dirs")
	}
	if i, ok := stateIndex("state3"); !ok || i != 3 {
		t.Fatalf("state3 index = %d ok=%v", i, ok)
	}
	if _, ok := stateIndex("driver"); ok {
		t.Fatal("non-state name must not parse")
	}
}

func TestCstateCollector_Residency(t *testing.T) {
	root := t.TempDir()
	// Two CPUs, states 0..3. Shallow (0/1) are ignored; deep (2/3) count.
	for _, cpu := range []int{0, 1} {
		writeCstateTime(t, root, cpu, 0, 1_000_000) // POLL
		writeCstateTime(t, root, cpu, 1, 500_000)   // C1
		writeCstateTime(t, root, cpu, 2, 200_000)   // deep
		writeCstateTime(t, root, cpu, 3, 800_000)   // deep
	}
	// A non-CPU sibling must be ignored.
	_ = os.MkdirAll(filepath.Join(root, "devices/system/cpu/cpufreq"), 0o755)

	c := NewCstateCollector(root)
	c.window = 100 * time.Millisecond // windowUs*cpus = 100000*2 = 200000
	c.sleep = func(time.Duration) {
		// +150ms of deep residency total across the two CPUs -> 75%.
		writeCstateTime(t, root, 0, 3, 800_000+75_000)
		writeCstateTime(t, root, 1, 3, 800_000+75_000)
	}
	hm := c.CollectHost(context.Background())
	v := hm["cstate-deep-residency"]
	if !v.Present() || v.V < 74.9 || v.V > 75.1 {
		t.Fatalf("deep residency = %+v, want ~75%%", v)
	}
	if v.Source != "cstate" {
		t.Fatalf("source = %q, want cstate", v.Source)
	}
}

func TestCstateCollector_NoCpuidle(t *testing.T) {
	c := NewCstateCollector(t.TempDir()) // empty sysfs: no cpuidle framework
	c.sleep = func(time.Duration) { t.Fatal("must not window when cpuidle is absent") }
	if v := c.CollectHost(context.Background())["cstate-deep-residency"]; v.Present() {
		t.Fatalf("absent cpuidle must be unavailable, got %+v", v)
	}
}
