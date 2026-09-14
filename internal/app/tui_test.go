package app

import (
	"context"
	"testing"
	"time"

	"github.com/netikras/procfit/internal/ports"
	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/queryspec"
	"github.com/netikras/procfit/internal/testutil"
	"github.com/netikras/procfit/internal/tui"
)

func TestOrphanGroup(t *testing.T) {
	managed := map[int]bool{1: true, 2: true, 3: true}
	liveSeen := map[int]bool{1: true} // pid 1 still alive; 2 and 3 exited
	names := map[int]string{2: "idea", 3: ""}

	g := orphanGroup(managed, liveSeen, names)
	if g == nil || g.Label != "orphans" || len(g.Sub) != 2 {
		t.Fatalf("orphans group wrong: %+v", g)
	}
	// Labelled by persisted name, falling back to pid:N; sorted.
	if g.Sub[0].Label != "idea" || g.Sub[1].Label != "pid:3" {
		t.Fatalf("orphan labels wrong: %q, %q", g.Sub[0].Label, g.Sub[1].Label)
	}
	// Ghost rows carry the pid so identity columns render.
	if g.Sub[0].Process == nil || g.Sub[0].Process.PID == 0 {
		t.Fatal("orphan row should carry a ghost process with its pid")
	}
	// No orphans -> no group.
	if orphanGroup(map[int]bool{1: true}, map[int]bool{1: true}, nil) != nil {
		t.Fatal("all-live should yield no orphans group")
	}
}

// TestTUIManagedTree_LiveThenOrphan drives the managed-panel provider end to end:
// a managed pid that is still live renders as a normal (browser-style) row, and
// once it exits it moves under the built-in "orphans" group keeping the name that
// was captured while it was alive (PM-0311/0312).
func TestTUIManagedTree_LiveThenOrphan(t *testing.T) {
	_, restore := fakeControl(t, pstat(100, "idea", 0))
	defer restore()
	// Manage pid 100 from the browser (stop): it becomes a managed target.
	if _, err := tuiControl()(tui.ControlRequest{PIDs: []int{100}, Label: "idea", Kind: tui.CtrlStop}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	clk := testutil.NewFakeClock(time.Unix(2000, 0))

	// Live: pid 100 present in the sampled source -> a live managed row, no orphans.
	liveSrc := testutil.NewFakeSource([]ports.ProcStat{stat(100, "idea", 100, 0, 4096)})
	res, cols, err := newAssemblyWith(liveSrc, clk).tuiManagedTree(ctx)(queryspec.Flags{})
	if err != nil {
		t.Fatalf("managed tree (live): %v", err)
	}
	if len(cols) == 0 {
		t.Fatal("expected resolved columns")
	}
	if findGroupRow(res.Rows, "orphans") != nil {
		t.Fatalf("a live managed pid must not be an orphan: %+v", res.Rows)
	}
	if !rowsMentionPID(res.Rows, 100) {
		t.Fatalf("managed live pid 100 missing from tree: %+v", res.Rows)
	}

	// Orphan: pid 100 no longer in the source -> bucketed under "orphans",
	// labelled by the name captured while it was alive.
	deadSrc := testutil.NewFakeSource([]ports.ProcStat{})
	res2, _, err := newAssemblyWith(deadSrc, clk).tuiManagedTree(ctx)(queryspec.Flags{})
	if err != nil {
		t.Fatalf("managed tree (orphan): %v", err)
	}
	g := findGroupRow(res2.Rows, "orphans")
	if g == nil || len(g.Sub) != 1 {
		t.Fatalf("expected exactly one orphan, got rows %+v", res2.Rows)
	}
	if g.Sub[0].Label != "idea" {
		t.Fatalf("orphan should keep the name captured while alive, got %q", g.Sub[0].Label)
	}
}

func findGroupRow(rows []*query.Row, label string) *query.Row {
	for _, r := range rows {
		if r.Kind == query.RowGroup && r.Label == label {
			return r
		}
	}
	return nil
}

func rowsMentionPID(rows []*query.Row, pid int) bool {
	for _, r := range rows {
		if r.Process != nil && r.Process.PID == pid {
			return true
		}
		if rowsMentionPID(r.Sub, pid) {
			return true
		}
	}
	return false
}
