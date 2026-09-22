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
	// One unified catalog: a metric (cpu), a structural column (wchan), and a
	// dimension-only field (cgroup) all appear, with the USE-flags header.
	for _, want := range []string{"USE flags", "cpu", "wchan", "cgroup", "DESCRIPTION"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("metrics catalog missing %q:\n%s", want, out.String())
		}
	}
	out.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"metrics", "list", "--format", "json"}); code != ExitOK {
		t.Fatalf("metrics list json exit %d", code)
	}
	for _, want := range []string{`"id": "cpu"`, `"column"`, `"group_by"`, `"filterable"`, `"metric"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("metrics json missing %q:\n%s", want, out.String())
		}
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

func TestPS_DefaultColumns(t *testing.T) {
	g := []ports.ProcStat{stat(1, "worker", 100, 0, 4096)}
	_, restore := fakeAssembly(t, g, g)
	defer restore()
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"ps"}); code != ExitOK {
		t.Fatalf("ps default exit %d stderr=%s", code, errb.String())
	}
	// Default columns include target, pt, cpu, rss headers.
	for _, h := range []string{"TARGET", "P/T", "CPU", "RSS"} {
		if !strings.Contains(out.String(), h) {
			t.Fatalf("default header %q missing:\n%s", h, out.String())
		}
	}
}

func TestPS_Formats(t *testing.T) {
	g := []ports.ProcStat{stat(1, "worker", 100, 0, 4096)}
	_, restore := fakeAssembly(t, g, g)
	defer restore()
	for _, f := range []string{"csv", "ndjson", "wide"} {
		var out, errb bytes.Buffer
		if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"ps", "--format", f, "--group-by", "none", "--leaf", "process"}); code != ExitOK {
			t.Fatalf("ps --format %s exit %d stderr=%s", f, code, errb.String())
		}
		if out.Len() == 0 {
			t.Fatalf("ps --format %s produced no output", f)
		}
	}
}

func TestStat_NDJSON(t *testing.T) {
	g1 := []ports.ProcStat{stat(1, "worker", 100, 0, 4096)}
	g2 := []ports.ProcStat{stat(1, "worker", 150, 0, 4096)}
	_, restore := fakeAssembly(t, g1, g2)
	defer restore()
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"stat", "1s", "--count", "2", "--format", "ndjson", "--group-by", "comm", "--leaf", "none"}); code != ExitOK {
		t.Fatalf("stat ndjson exit %d stderr=%s", code, errb.String())
	}
	// Exactly one meta record across the whole stream.
	if n := strings.Count(out.String(), `"record":"meta"`); n != 1 {
		t.Fatalf("expected exactly 1 meta record, got %d:\n%s", n, out.String())
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
