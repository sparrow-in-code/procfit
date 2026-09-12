package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/netikras/procfit/internal/control"
	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/ports"
	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/testutil"
)

func TestLoadPolicies(t *testing.T) {
	cfgDir := filepath.Join(t.TempDir(), "procfit")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `version = 1
[[managed]]
name = "browsers"
selector = 'comm == "chrome"'
[managed.control]
nice = 10
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Dir(cfgDir))

	specs, err := loadPolicies(metrics.NewDefault(), query.NewDimensions())
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || specs[0].Name != "browsers" || specs[0].Nice == nil || *specs[0].Nice != 10 {
		t.Fatalf("policy spec wrong: %+v", specs)
	}
}

func TestLoadPolicies_NoConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	specs, err := loadPolicies(metrics.NewDefault(), query.NewDimensions())
	if err != nil || specs != nil {
		t.Fatalf("no config should yield no policies: %v %v", specs, err)
	}
}

func TestEngine_ReconcilesConfigPolicy(t *testing.T) {
	id := model.ProcessInstanceID{BootID: "boot-test", PID: 1234, StartTime: 1234}
	fc := testutil.NewFakeController()
	fc.Add(1234, 0, id)
	st := ports.ProcStat{ID: id, PID: 1234, Comm: "chrome", NiceAvail: model.Available, IOAvail: model.Available}
	src := testutil.NewFakeSource([]ports.ProcStat{st})
	clk := testutil.NewFakeClock(time.Unix(1000, 0))
	mgr := control.NewManager(fc, clk, control.NewSafeguards(1, 1), "boot-test")
	eng := NewEngine(metrics.NewDefault(), query.NewDimensions(), src, fc, clk, mgr, nil, time.Second)

	nice := 9
	eng.SetPolicies([]control.PolicySpec{{Name: "b", Selector: `comm == "chrome"`, Nice: &nice, OnDrift: "report"}})
	if err := eng.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fc.Nice[1234] != 9 {
		t.Fatalf("reconcile should apply policy nice, got %d", fc.Nice[1234])
	}
}
