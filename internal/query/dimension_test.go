package query

import "testing"

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
	if !has("wchan") || !has("pstate") {
		t.Fatalf("IDs should include the wchan/pstate dimensions: %v", ids)
	}
}
