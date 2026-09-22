package query

import (
	"testing"

	"github.com/netikras/procfit/internal/model"
)

func TestDimensions_IDs(t *testing.T) {
	d := NewDimensions()
	ids := d.IDs()
	if len(ids) == 0 {
		t.Fatal("expected registered dimensions")
	}
	// IDs is the single source of truth for the --group-by help; every listed id
	// must actually resolve, so the help can never advertise a bad dimension.
	for _, id := range ids {
		if !d.Has(id) {
			t.Fatalf("IDs lists %q but Has(%q) is false", id, id)
		}
	}
	has := func(x string) bool {
		for _, id := range ids {
			if id == x {
				return true
			}
		}
		return false
	}
	if !has("wchan") || !has("pstate") || !has("hostname") {
		t.Fatalf("IDs should include the wchan/pstate/hostname dimensions: %v", ids)
	}
}

func TestDimensions_HostnameGroups(t *testing.T) {
	d := NewDimensions()
	dim, ok := d.Get("hostname")
	if !ok {
		t.Fatal("hostname dimension should be registered")
	}
	// A containerized process buckets by its HOSTNAME env; a bare one (empty)
	// does not group (the source supplies the host fallback before this stage).
	key, label, ok := dim.Key(&model.Process{Hostname: "web-1"})
	if !ok || key != "web-1" || label != "web-1" {
		t.Fatalf("hostname grouping wrong: key=%q label=%q ok=%v", key, label, ok)
	}
	if _, _, ok := dim.Key(&model.Process{}); ok {
		t.Fatal("empty hostname must not group")
	}
}
