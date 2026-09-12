package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFileRecorder_AppendsNDJSON(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "history")
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Directory must be private.
	info, _ := os.Stat(dir)
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode = %v, want 0700", info.Mode().Perm())
	}
	r.Record(Event{Action: "applied", Target: "chrome", PID: 7, Field: "nice", From: "0", To: "10"})
	r.Record(Event{Action: "restored", PID: 7, Field: "nice", To: "0"})

	f, err := os.Open(filepath.Join(dir, "events.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fi, _ := f.Stat()
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v, want 0600", fi.Mode().Perm())
	}
	var n int
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("line %d not json: %v", n, err)
		}
		if e.Time.IsZero() {
			t.Fatal("event should have a timestamp")
		}
		n++
	}
	if n != 2 {
		t.Fatalf("want 2 events, got %d", n)
	}
}

func TestNop(t *testing.T) {
	Nop{}.Record(Event{Action: "applied"}) // must not panic or write anything
}
