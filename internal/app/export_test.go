package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/netikras/procfit/internal/ports"
)

func TestExport_OneShotPrometheus(t *testing.T) {
	_, restore := fakeAssembly(t, []ports.ProcStat{stat(1, "init", 50, 0, 4096)})
	defer restore()

	var out, errb bytes.Buffer
	code := run(Env{Stdout: &out, Stderr: &errb}, []string{"export", "--profile", "process", "--instant"})
	if code != ExitOK {
		t.Fatalf("export exit %d: %s", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "# TYPE procfit_rss gauge") {
		t.Fatalf("expected a prometheus rss family, got:\n%s", got)
	}
	if !strings.Contains(got, "procfit_rss{") {
		t.Fatalf("expected a labelled rss series, got:\n%s", got)
	}
}
