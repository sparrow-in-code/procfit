package daemon

import (
	"context"
	"encoding/json"
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

func testEngine(t *testing.T) (*Engine, *testutil.FakeController) {
	t.Helper()
	id := model.ProcessInstanceID{BootID: "boot-test", PID: 1234, StartTime: 1234}
	fc := testutil.NewFakeController()
	fc.Add(1234, 0, id)
	st := ports.ProcStat{ID: id, PID: 1234, Comm: "worker", NiceAvail: model.Available, IOAvail: model.Available, NumThreads: 2}
	src := testutil.NewFakeSource([]ports.ProcStat{st})
	clk := testutil.NewFakeClock(time.Unix(1000, 0))
	mgr := control.NewManager(fc, clk, control.NewSafeguards(1, 1), "boot-test")
	eng := NewEngine(metrics.NewDefault(), query.NewDimensions(), src, fc, clk, mgr, nil, time.Second)
	if err := eng.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	return eng, fc
}

func serve(t *testing.T, eng *Engine, saver func() error) (*Client, context.CancelFunc) {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "d.sock")
	ln, err := Listen(sock)
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(eng, saver, os.Getuid())
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Serve(ctx, ln) }()
	cl, err := Dial(sock)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	return cl, cancel
}

func TestDaemon_HelloQueryHealth(t *testing.T) {
	eng, _ := testEngine(t)
	cl, cancel := serve(t, eng, func() error { return nil })
	defer cancel()
	defer cl.Close()

	hello, err := cl.Hello()
	if err != nil || hello.Version != ProtocolVersion || hello.Name == "" {
		t.Fatalf("hello wrong: %+v err=%v", hello, err)
	}

	resp, err := cl.Do(Request{Type: ReqQuery, Query: &QueryReq{GroupBy: "comm", Leaf: "none"}})
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal(resp.Rows, &env); err != nil {
		t.Fatalf("query rows not json: %v", err)
	}
	if env["schema_version"].(float64) != 1 {
		t.Fatalf("query envelope wrong: %v", env)
	}

	resp, err = cl.Do(Request{Type: ReqHealth})
	if err != nil || resp.Health == nil {
		t.Fatalf("health wrong: %+v err=%v", resp, err)
	}
	if resp.Health.Processes != 1 {
		t.Fatalf("health processes = %d, want 1", resp.Health.Processes)
	}
}

func TestDaemon_ControlSetNiceAndManaged(t *testing.T) {
	eng, fc := testEngine(t)
	saved := 0
	cl, cancel := serve(t, eng, func() error { saved++; return nil })
	defer cancel()
	defer cl.Close()

	// set-nice on pid target creates a managed target and applies control.
	resp, err := cl.Do(Request{Type: ReqControl, Control: &ControlReq{Op: "set-nice", Target: "pid:1234", Nice: intptr(10)}})
	if err != nil {
		t.Fatalf("set-nice: %v", err)
	}
	var res control.ApplyResult
	if err := json.Unmarshal(resp.Control, &res); err != nil {
		t.Fatal(err)
	}
	if res.Counts[control.StatusApplied] != 1 {
		t.Fatalf("expected applied, got %+v", res.Counts)
	}
	if fc.Nice[1234] != 10 {
		t.Fatalf("nice not applied: %d", fc.Nice[1234])
	}
	if saved == 0 {
		t.Fatal("state should have been saved after control")
	}

	// managed lists the target.
	resp, err = cl.Do(Request{Type: ReqManaged})
	if err != nil {
		t.Fatal(err)
	}
	var targets []control.Target
	if err := json.Unmarshal(resp.Managed, &targets); err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].Name != "pid:1234" {
		t.Fatalf("managed listing wrong: %+v", targets)
	}
}

func TestDaemon_QueryError(t *testing.T) {
	eng, _ := testEngine(t)
	cl, cancel := serve(t, eng, func() error { return nil })
	defer cancel()
	defer cl.Close()
	if _, err := cl.Do(Request{Type: ReqQuery, Query: &QueryReq{Select: "bogusfield == 1"}}); err == nil {
		t.Fatal("invalid select should error over IPC")
	}
}

func intptr(i int) *int { return &i }
