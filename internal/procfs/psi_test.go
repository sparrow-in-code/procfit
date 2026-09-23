package procfs

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/netikras/procfit/internal/model"
)

func TestParsePSISomeAvg10(t *testing.T) {
	body := "some avg10=1.50 avg60=0.80 avg300=0.20 total=123456\n" +
		"full avg10=0.30 avg60=0.10 avg300=0.05 total=6789\n"
	if v, ok := parsePSISomeAvg10(body); !ok || v != 1.50 {
		t.Fatalf("some avg10 = %v ok=%v, want 1.50", v, ok)
	}
	if _, ok := parsePSISomeAvg10("full avg10=0.30\n"); ok {
		t.Fatal("a body with no 'some' line must not parse")
	}
}

func TestPSICollector(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "pressure/cpu"), "some avg10=2.00 avg60=1.0 avg300=0.5 total=9\n")
	mustWrite(t, filepath.Join(root, "pressure/io"),
		"some avg10=0.00 avg60=0.0 avg300=0.0 total=0\nfull avg10=0.00 avg60=0.0 avg300=0.0 total=0\n")
	// memory file absent -> psi-mem unavailable (CONFIG_PSI partial / not exposed).

	c := NewPSICollector(root)
	procs := []model.Process{{PID: 1}, {PID: 2}}
	c.Collect(context.Background(), procs)

	for _, p := range procs {
		if v := p.Metric("psi-cpu"); !v.Present() || v.V != 2.0 {
			t.Fatalf("psi-cpu = %+v, want 2.0", v)
		}
		if v := p.Metric("psi-io"); !v.Present() || v.V != 0.0 {
			t.Fatalf("psi-io = %+v, want 0.0 (present)", v)
		}
		if v := p.Metric("psi-mem"); v.Present() {
			t.Fatalf("psi-mem must be unavailable (no file), got %+v", v)
		}
	}
}
