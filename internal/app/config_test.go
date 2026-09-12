package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestConfigCheck(t *testing.T) {
	good := writeTemp(t, "c.toml", "version = 1\ngroup_by = [\"comm\"]\nleaf = \"none\"\n")
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"config", "check", good}); code != ExitOK {
		t.Fatalf("check good exit %d stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "OK") {
		t.Fatalf("expected OK: %s", out.String())
	}

	bad := writeTemp(t, "bad.toml", "version = 1\nbogus = true\n")
	out.Reset()
	errb.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"config", "check", bad}); code != ExitUsage {
		t.Fatalf("check bad should be usage error, got %d", code)
	}
}

func TestConfigConvert(t *testing.T) {
	src := writeTemp(t, "c.toml", "version = 1\ninterval = \"2s\"\ngroup_by = [\"comm\"]\nleaf = \"none\"\n")
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"config", "convert", src, "--to", "json"}); code != ExitOK {
		t.Fatalf("convert exit %d stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), `"interval": "2s"`) {
		t.Fatalf("converted json missing interval:\n%s", out.String())
	}
}

func TestConfigDump(t *testing.T) {
	src := writeTemp(t, "c.toml", "version = 1\ngroup_by = [\"comm\"]\nleaf = \"none\"\n")
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"config", "dump", "--effective", "--config", src, "--format", "yaml"}); code != ExitOK {
		t.Fatalf("dump exit %d stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "leaf: none") {
		t.Fatalf("dump missing leaf:\n%s", out.String())
	}
}

func TestConfigDumpDefaultsNoFile(t *testing.T) {
	// Point XDG at an empty dir so discovery finds nothing and defaults are used.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"config", "dump", "--effective"}); code != ExitOK {
		t.Fatalf("dump defaults exit %d stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "version = 1") {
		t.Fatalf("default dump missing version:\n%s", out.String())
	}
}

func TestConfigCheckNoFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"config", "check"}); code != ExitOK {
		t.Fatalf("check no-file exit %d", code)
	}
	if !strings.Contains(out.String(), "no config file") {
		t.Fatalf("expected no-config message: %s", out.String())
	}
}

func TestConfigUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"config"}); code != ExitUsage {
		t.Fatalf("bare config should be usage error, got %d", code)
	}
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"config", "bogus"}); code != ExitUsage {
		t.Fatalf("unknown subcommand should be usage error, got %d", code)
	}
}
