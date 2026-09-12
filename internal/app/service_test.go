package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceInstallUninstall(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"service", "install"}); code != ExitOK {
		t.Fatalf("install exit %d stderr=%s", code, errb.String())
	}
	unit := filepath.Join(cfg, "systemd", "user", "procfit.service")
	data, err := os.ReadFile(unit)
	if err != nil {
		t.Fatalf("unit not written: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "daemon run") || !strings.Contains(s, "WantedBy=default.target") {
		t.Fatalf("unit content wrong:\n%s", s)
	}
	// Install must NOT auto-enable: it only prints the enable command.
	if !strings.Contains(out.String(), "systemctl --user enable") {
		t.Fatalf("should print (not run) the enable command:\n%s", out.String())
	}

	out.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"service", "uninstall"}); code != ExitOK {
		t.Fatalf("uninstall exit %d", code)
	}
	if _, err := os.Stat(unit); !os.IsNotExist(err) {
		t.Fatal("unit should be removed")
	}
}

func TestServiceUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"service"}); code != ExitUsage {
		t.Fatalf("bare service should be usage, got %d", code)
	}
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"service", "bogus"}); code != ExitUsage {
		t.Fatalf("unknown subcommand should be usage, got %d", code)
	}
}
