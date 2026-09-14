package app

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/netikras/procfit/internal/history"
)

func TestCmdHistory(t *testing.T) {
	dir := t.TempDir()
	rec, err := history.Open(filepath.Join(dir, "history"))
	if err != nil {
		t.Fatal(err)
	}
	rec.Record(history.Event{Action: "applied", PID: 42, Field: "nice", From: "0", To: "10"})

	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"history", "--state-dir", dir, "--pid", "42"}); code != ExitOK {
		t.Fatalf("history exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "applied") || !strings.Contains(out.String(), "0→10") {
		t.Fatalf("history output wrong:\n%s", out.String())
	}

	// A pid with no events reports the enable hint, not an error.
	out.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"history", "--state-dir", dir, "--pid", "999"}); code != ExitOK {
		t.Fatalf("empty history exit %d", code)
	}
	if !strings.Contains(out.String(), "no control history") {
		t.Fatalf("expected empty-history hint:\n%s", out.String())
	}
}
