package history

import (
	"path/filepath"
	"testing"
)

func TestReadRecent(t *testing.T) {
	dir := t.TempDir()
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	r.Record(Event{Action: "applied", PID: 1, Field: "nice", From: "0", To: "5"})
	r.Record(Event{Action: "applied", PID: 2, Field: "nice", From: "0", To: "5"})
	r.Record(Event{Action: "restored", PID: 1, Field: "nice", From: "5", To: "0"})

	all, err := ReadRecent(dir, nil, 0)
	if err != nil || len(all) != 3 {
		t.Fatalf("all: got %d (%v)", len(all), err)
	}
	p1, _ := ReadRecent(dir, func(e Event) bool { return e.PID == 1 }, 0)
	if len(p1) != 2 {
		t.Fatalf("pid 1 filter: got %d", len(p1))
	}
	lim, _ := ReadRecent(dir, nil, 1)
	if len(lim) != 1 || lim[0].Action != "restored" {
		t.Fatalf("limit should keep the most recent: %+v", lim)
	}
	// A missing log is not an error.
	none, err := ReadRecent(filepath.Join(dir, "nope"), nil, 0)
	if err != nil || len(none) != 0 {
		t.Fatalf("missing log should be empty/no-error: %d %v", len(none), err)
	}
}
