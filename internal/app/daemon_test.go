package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestDaemonStatus_NotRunning(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // empty dir => no socket
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"daemon", "status"}); code != ExitDaemon {
		t.Fatalf("status with no daemon => %d, want ExitDaemon", code)
	}
	if !strings.Contains(errb.String(), "not running") {
		t.Fatalf("expected 'not running' message: %s", errb.String())
	}
}

func TestDaemonUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"daemon"}); code != ExitUsage {
		t.Fatalf("bare daemon => %d, want usage", code)
	}
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"daemon", "bogus"}); code != ExitUsage {
		t.Fatalf("unknown daemon subcommand => %d, want usage", code)
	}
}

func TestTUI_RequiresTTY(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb, IsTTY: false}, []string{"tui"}); code != ExitUsage {
		t.Fatalf("tui without a TTY => %d, want usage", code)
	}
	if !strings.Contains(errb.String(), "terminal") {
		t.Fatalf("expected TTY message: %s", errb.String())
	}
}
