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

func TestApplyConfigDefaults_CollapseGroups(t *testing.T) {
	cfg := writeCfg(t, "version = 1\ncollapse_groups = true\n")

	// Config turns it on.
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	qf := bindQueryFlags(fs)
	if err := fs.Parse([]string{"--config", cfg}); err != nil {
		t.Fatal(err)
	}
	if err := applyConfigDefaults(fs, qf); err != nil {
		t.Fatal(err)
	}
	if !qf.collapseGroups || qf.sources["collapse-groups"] != SourceConfig {
		t.Fatalf("config should set collapse-groups (source config), got %v/%s",
			qf.collapseGroups, qf.sources["collapse-groups"])
	}
	if !qf.toSpecFlags().CollapseGroups {
		t.Fatal("toSpecFlags must carry CollapseGroups to the TUI model")
	}

	// Env overrides config off; a later --collapse-groups on the CLI wins again.
	t.Setenv("PROCFIT_COLLAPSE_GROUPS", "false")
	fs2 := flag.NewFlagSet("tui", flag.ContinueOnError)
	qf2 := bindQueryFlags(fs2)
	if err := fs2.Parse([]string{"--config", cfg, "--collapse-groups"}); err != nil {
		t.Fatal(err)
	}
	if err := applyConfigDefaults(fs2, qf2); err != nil {
		t.Fatal(err)
	}
	if !qf2.collapseGroups || qf2.sources["collapse-groups"] != SourceArgs {
		t.Fatalf("CLI --collapse-groups must win, got %v/%s",
			qf2.collapseGroups, qf2.sources["collapse-groups"])
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

func TestApplyConfigDefaults_EnvAndSources(t *testing.T) {
	t.Setenv("PROCFIT_LEAF", "thread")
	fs := flag.NewFlagSet("ps", flag.ContinueOnError)
	qf := bindQueryFlags(fs)
	if err := fs.Parse([]string{"--no-config", "--group-by", "comm"}); err != nil {
		t.Fatal(err)
	}
	if err := applyConfigDefaults(fs, qf); err != nil {
		t.Fatal(err)
	}
	if qf.leaf != "thread" || qf.sources["leaf"] != SourceEnv {
		t.Fatalf("env should set leaf=thread (source env), got %q/%s", qf.leaf, qf.sources["leaf"])
	}
	if qf.groupBy != "comm" || qf.sources["group-by"] != SourceArgs {
		t.Fatalf("args should win group-by, got %q/%s", qf.groupBy, qf.sources["group-by"])
	}
	if qf.sources["columns"] != SourceDefault {
		t.Fatalf("unset columns should be source=default, got %s", qf.sources["columns"])
	}
}

func TestApplyConfigDefaults_PrecedenceReorder(t *testing.T) {
	t.Setenv("PROCFIT_FORMAT", "json")
	fs := flag.NewFlagSet("ps", flag.ContinueOnError)
	qf := bindQueryFlags(fs)
	// With env placed above args, env outranks the CLI --format.
	if err := fs.Parse([]string{"--no-config", "--config-precedence", "config,args,env", "--format", "csv"}); err != nil {
		t.Fatal(err)
	}
	if err := applyConfigDefaults(fs, qf); err != nil {
		t.Fatal(err)
	}
	if qf.format != "json" || qf.sources["format"] != SourceEnv {
		t.Fatalf("env should outrank args when reordered, got %q/%s", qf.format, qf.sources["format"])
	}
}

func TestApplyConfigDefaults_ConfigBelowEnv(t *testing.T) {
	cfg := writeCfg(t, "version = 1\nleaf = \"none\"\n")
	t.Setenv("PROCFIT_LEAF", "thread")
	fs := flag.NewFlagSet("ps", flag.ContinueOnError)
	qf := bindQueryFlags(fs)
	if err := fs.Parse([]string{"--config", cfg}); err != nil {
		t.Fatal(err)
	}
	if err := applyConfigDefaults(fs, qf); err != nil {
		t.Fatal(err)
	}
	// Default order: config < env, so env wins over the file.
	if qf.leaf != "thread" || qf.sources["leaf"] != SourceEnv {
		t.Fatalf("env should outrank config, got %q/%s", qf.leaf, qf.sources["leaf"])
	}
}

func TestParsePrecedence(t *testing.T) {
	if _, err := parsePrecedence("config,bogus"); err == nil {
		t.Fatal("an unknown layer must be rejected")
	}
	order, err := parsePrecedence("")
	if err != nil || len(order) != 4 {
		t.Fatalf("empty spec should give the 4-layer default, got %v (%v)", order, err)
	}
	// default is always forced to the floor even if omitted or placed elsewhere.
	if order, _ := parsePrecedence("args,config,env"); order[0] != SourceDefault {
		t.Fatalf("default must be the floor, got %v", order)
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
