package app

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func writeCfg(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestApplyConfigDefaults_MergeAndPrecedence(t *testing.T) {
	cfg := writeCfg(t, `
version = 1
group_by = ["comm"]
leaf = "none"
columns = ["target", "cpu"]
target_width = 25
[metrics]
profile = "light"
[[sort]]
field = "cpu"
direction = "desc"
`)
	fs := flag.NewFlagSet("ps", flag.ContinueOnError)
	qf := bindQueryFlags(fs)
	// CLI sets group-by (must win over config) and points at the config file.
	if err := fs.Parse([]string{"--config", cfg, "--group-by", "user"}); err != nil {
		t.Fatal(err)
	}
	if err := applyConfigDefaults(fs, qf); err != nil {
		t.Fatal(err)
	}
	if qf.groupBy != "user" {
		t.Fatalf("CLI group-by must win, got %q", qf.groupBy)
	}
	if qf.leaf != "none" || qf.columns != "target,cpu" || qf.targetWidth != 25 || qf.profile != "light" {
		t.Fatalf("config defaults not applied: %+v", qf)
	}
	if qf.sortSpec != "cpu:desc" {
		t.Fatalf("sort from config = %q, want cpu:desc", qf.sortSpec)
	}
}

func TestApplyConfigDefaults_NoConfig(t *testing.T) {
	cfg := writeCfg(t, "version = 1\ntarget_width = 99\n")
	fs := flag.NewFlagSet("ps", flag.ContinueOnError)
	qf := bindQueryFlags(fs)
	if err := fs.Parse([]string{"--config", cfg, "--no-config"}); err != nil {
		t.Fatal(err)
	}
	if err := applyConfigDefaults(fs, qf); err != nil {
		t.Fatal(err)
	}
	if qf.targetWidth != 0 {
		t.Fatalf("--no-config must ignore the file, got target_width %d", qf.targetWidth)
	}
}

func TestApplyConfigDefaults_BadConfig(t *testing.T) {
	cfg := writeCfg(t, "version = 1\nbogus_key = true\n")
	fs := flag.NewFlagSet("ps", flag.ContinueOnError)
	qf := bindQueryFlags(fs)
	_ = fs.Parse([]string{"--config", cfg})
	if err := applyConfigDefaults(fs, qf); err == nil {
		t.Fatal("a malformed config should be an error")
	}
}
