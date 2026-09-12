package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/netikras/procfit/internal/ports"
)

func TestMetricsList(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"metrics", "list"}); code != ExitOK {
		t.Fatalf("metrics list exit %d", code)
	}
	if !strings.Contains(out.String(), "cpu") {
		t.Fatal("metrics list should include cpu")
	}
	out.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"metrics", "list", "--format", "json"}); code != ExitOK {
		t.Fatalf("metrics list json exit %d", code)
	}
	if !strings.Contains(out.String(), `"id": "cpu"`) {
		t.Fatalf("metrics json missing cpu:\n%s", out.String())
	}
	out.Reset()
	errb.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"metrics", "bogus"}); code != ExitUsage {
		t.Fatalf("unknown metrics subcommand should be usage error, got %d", code)
	}
}

func TestCapabilities(t *testing.T) {
	_, restore := fakeAssembly(t, []ports.ProcStat{stat(1, "a", 1, 0, 1)})
	defer restore()
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"capabilities"}); code != ExitOK {
		t.Fatalf("capabilities exit %d", code)
	}
	if !strings.Contains(out.String(), "procfs") {
		t.Fatalf("capabilities should mention procfs:\n%s", out.String())
	}
	out.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"capabilities", "--format", "json"}); code != ExitOK {
		t.Fatalf("capabilities json exit %d", code)
	}
	if !strings.Contains(out.String(), `"name": "procfs"`) {
		t.Fatalf("capabilities json wrong:\n%s", out.String())
	}
}

func TestPS_SelectAndHaving(t *testing.T) {
	p1 := stat(1, "keep", 100, 0, 4096)
	p1.UID = 1000
	p2 := stat(2, "drop", 100, 0, 4096)
	p2.UID = 0
	g := []ports.ProcStat{p1, p2}
	_, restore := fakeAssembly(t, g, g)
	defer restore()

	var out, errb bytes.Buffer
	code := run(Env{Stdout: &out, Stderr: &errb}, []string{"ps", "--group-by", "none", "--leaf", "process", "--columns", "target", "--select", "uid == 1000"})
	if code != ExitOK {
		t.Fatalf("select exit %d stderr=%s", code, errb.String())
	}
	if strings.Contains(out.String(), "drop") || !strings.Contains(out.String(), "keep") {
		t.Fatalf("select uid==1000 wrong output:\n%s", out.String())
	}

	// Invalid field must be a usage error (type-checked, RFC §10.5).
	out.Reset()
	errb.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"ps", "--select", "bogusfield == 1"}); code != ExitUsage {
		t.Fatalf("invalid select field should be usage error, got %d", code)
	}
}

func TestPS_UnknownMetricAndFormatErrors(t *testing.T) {
	_, restore := fakeAssembly(t, []ports.ProcStat{stat(1, "a", 1, 0, 1)}, []ports.ProcStat{stat(1, "a", 1, 0, 1)})
	defer restore()

	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"ps", "--metric", "+bogus"}); code != ExitUsage {
		t.Fatalf("unknown metric should be usage error, got %d", code)
	}
	out.Reset()
	errb.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"ps", "--sort", "cpu:sideways"}); code != ExitUsage {
		t.Fatalf("bad sort direction should be usage error, got %d", code)
	}
	out.Reset()
	errb.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"ps", "--format", "xml", "--group-by", "none", "--leaf", "process"}); code != ExitUsage {
		t.Fatalf("unknown format should be usage error, got %d", code)
	}
	out.Reset()
	errb.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"ps", "--group-by", "none,comm", "--leaf", "none"}); code != ExitUsage {
		t.Fatalf("invalid group-by should be usage error, got %d", code)
	}
}

func TestHelp(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"help"}); code != ExitOK {
		t.Fatalf("help exit %d", code)
	}
	if !strings.Contains(out.String(), "Commands:") {
		t.Fatal("help should list commands")
	}
	out.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"--version"}); code != ExitOK {
		t.Fatalf("--version exit %d", code)
	}
}

func TestParseSortKeyForms(t *testing.T) {
	cases := map[string]struct {
		field string
		desc  bool
	}{
		"cpu":      {"cpu", false},
		"cpu:asc":  {"cpu", false},
		"cpu:desc": {"cpu", true},
		"-cpu":     {"cpu", true},
		"+comm":    {"comm", false},
	}
	for in, want := range cases {
		k, err := parseSortKey(in)
		if err != nil {
			t.Fatalf("parseSortKey(%q): %v", in, err)
		}
		if k.Field != want.field || k.Descending != want.desc {
			t.Errorf("parseSortKey(%q) = %+v, want %+v", in, k, want)
		}
	}
	if _, err := parseSortKey("cpu:bogus"); err == nil {
		t.Fatal("bad direction should error")
	}
}
