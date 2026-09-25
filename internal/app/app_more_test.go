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

func TestMetricsCheck_ReportsAvailability(t *testing.T) {
	_, restore := fakeAssembly(t, []ports.ProcStat{stat(1, "init", 50, 0, 4096)})
	defer restore()

	var out, errb bytes.Buffer
	// `--check` without the `list` subcommand must work (leading flag, not a subcmd).
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"metrics", "--check"}); code != ExitOK {
		t.Fatalf("metrics --check exit %d: %s", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "AVAIL") {
		t.Fatalf("expected an AVAIL column:\n%s", got)
	}
	if !rowHasStatus(got, "rss", "yes") { // a plain gauge is available on the fake host
		t.Fatalf("rss should report available:\n%s", got)
	}
	// JSON carries the availability too.
	out.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"metrics", "list", "--check", "--format", "json"}); code != ExitOK {
		t.Fatalf("metrics --check json exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), `"available"`) {
		t.Fatalf("json should include an available field:\n%s", out.String())
	}
}

// rowHasStatus reports whether the catalog line for metric id contains status.
func rowHasStatus(catalog, id, status string) bool {
	for _, line := range strings.Split(catalog, "\n") {
		f := strings.Fields(line)
		if len(f) > 0 && f[0] == id && strings.Contains(line, status) {
			return true
		}
	}
	return false
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
