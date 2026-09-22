package app

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/netikras/procfit/internal/collect"
	"github.com/netikras/procfit/internal/control"
	"github.com/netikras/procfit/internal/history"
	"github.com/netikras/procfit/internal/meta"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/queryspec"
	"github.com/netikras/procfit/internal/render"
	"github.com/netikras/procfit/internal/tui"
)

// cmdTUI runs the interactive explorer (RFC §7.1, §19). It requires a TTY.
func cmdTUI(env Env, args []string) int {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	qf := bindQueryFlags(fs)
	interval := fs.Duration("interval", time.Second, "starting refresh interval, e.g. 500ms, 2s ([ ] adjust live)")
	setupUsage(env, fs, "tui", "interactive explorer (default on a TTY)",
		"procfit tui                                  # nav g/t/s/S//u; control n/N x/X z/Z (lower=apply, UPPER=lift; confirm y)",
		"procfit tui --interval 2s --group-by name --sort cpu:desc   # 2s refresh, grouped by name",
		"procfit tui -h                               # start with human units (toggle live with 'u')")
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
	}
	if err := applyConfigDefaults(fs, qf); err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitUsage
	}
	if qf.showConfig {
		printEffectiveSettings(env, fs, qf.sources)
		return ExitOK
	}
	if !env.IsTTY {
		fmt.Fprintf(env.Stderr, "%s: the TUI needs an interactive terminal; use '%s ps' or '%s stat'\n", meta.Name, meta.Name, meta.Name)
		return ExitUsage
	}
	a, err := newAssemblyFn()
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	deps := tui.Deps{
		Refresh:     a.tuiRefresh(context.Background()),
		ManagedTree: a.tuiManagedTree(context.Background()),
		Control:     tuiControl(),
		Drop:        tuiDrop(),
		History:     tuiHistory(),
		Interval:    *interval,
		Columns:     a.addableColumns(),
	}
	cli, err := tui.RunTerminal(deps, qf.toSpecFlags())
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	if cli != "" {
		fmt.Fprintln(env.Stdout, cli) // reproducible CLI for the final view (§27 Phase 3 exit)
	}
	return ExitOK
}

// addableColumns lists every displayable column id (metrics + structural), sorted
// — the choices for the TUI's interactive `+` column picker.
func (a *assembly) addableColumns() []string {
	ids := make([]string, 0)
	for _, d := range a.reg.All() {
		ids = append(ids, string(d.ID))
	}
	for _, c := range render.StructuralColumns() {
		ids = append(ids, c.ID)
	}
	sort.Strings(ids)
	return ids
}

// tuiRefresh builds a RefreshFunc backed by a persistent sampler so rates warm up
// across ticks; each refresh resolves the flags, samples, enriches, and builds.
func (a *assembly) tuiRefresh(ctx context.Context) tui.RefreshFunc {
	sampler := collect.NewSampler(a.src, a.clk)
	return func(f queryspec.Flags) (*query.Result, []render.Column, error) {
		r, err := queryspec.Build(a.reg, a.dims, f)
		if err != nil {
			return nil, nil, err
		}
		configureSource(a.src, r)
		snap, err := sampler.Sample(ctx, r.Needed)
		if err != nil {
			return nil, nil, err
		}
		a.applyResolvers(snap.Processes)
		a.enrich(ctx, snap.Processes, r.Needed)
		res, err := a.engine.Build(query.Input{
			Generation: snap.Generation, WallTime: snap.WallTime, Elapsed: snap.Elapsed, Processes: snap.Processes,
		}, r.Spec)
		if err != nil {
			return nil, nil, err
		}
		cols, err := render.ResolveColumnsMode(a.reg, r.Columns, r.Human)
		if err != nil {
			return nil, nil, err
		}
		return res, cols, nil
	}
}

// tuiControl builds the TUI's ControlFunc: it applies (or, DryRun, previews) a
// single-process control action via the same managed-target path as the CLI
// (auto-managing the pid), returning a summary string instead of printing — so
// it never corrupts the screen. State is loaded/saved per action.
func tuiControl() tui.ControlFunc {
	return func(req tui.ControlRequest) (string, error) {
		dir, _ := resolveStateDir("")
		c, err := newControlAsmFn(dir)
		if err != nil {
			return "", err
		}
		name := req.Target
		switch {
		case name != "":
			if c.mgr.Find(name) == nil {
				return "", fmt.Errorf("managed target %q not found", name)
			}
		case pickExistingTarget(c, req.PIDs) != "":
			// Already managed (e.g. acting from the managed panel): reuse the
			// existing target rather than creating a duplicate.
			name = pickExistingTarget(c, req.PIDs)
		default:
			// Browser: auto-manage the selected process(es) under a friendly name.
			insts := c.instancesForPIDs(req.PIDs)
			if len(insts) == 0 {
				return "", fmt.Errorf("no live instances for the selection")
			}
			for i := range insts { // persist a display name for orphans
				insts[i].Name = req.Label
			}
			name = tuiTargetName(req)
			c.mgr.Manage(name, control.ModeSnapshot, "", insts)
		}
		summary, err := applyTUIControl(c, name, req)
		if err != nil {
			return "", err
		}
		if !req.DryRun {
			if err := c.save(); err != nil {
				return "", err
			}
		}
		return summary, nil
	}
}

// tuiTargetName names the managed target for a TUI action recognizably in the
// managed panel: "<label>#<pid>" for a single process (e.g. "idea#281839"), or
// "tui:<label>" for a group. Stable across repeated actions on the same
// selection, so restore/unmanage address the same target.
func tuiTargetName(req tui.ControlRequest) string {
	if len(req.PIDs) == 1 {
		return fmt.Sprintf("%s#%d", sanitizeName(req.Label), req.PIDs[0])
	}
	return "tui:" + sanitizeName(req.Label)
}

// sanitizeName makes a row label safe and compact for use as a managed-target
// name (no spaces/slashes/control chars, bounded length).
func sanitizeName(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r == ' ', r == '\t', r == '/':
			return '_'
		case r < 0x20:
			return -1
		default:
			return r
		}
	}, strings.TrimSpace(s))
	if len(s) > 24 {
		s = s[:24]
	}
	if s == "" {
		return "target"
	}
	return s
}

// tuiHistory returns recent control-history events for a pid (most recent last),
// read from the opt-in audit log; empty when history is disabled/none.
func tuiHistory() tui.HistoryFunc {
	return func(pid int) []tui.HistoryEntry {
		dir, _ := resolveStateDir("")
		evs, err := history.ReadRecent(historyDir(dir), func(e history.Event) bool { return e.PID == pid }, 20)
		if err != nil {
			return nil
		}
		out := make([]tui.HistoryEntry, 0, len(evs))
		for _, e := range evs {
			out = append(out, tui.HistoryEntry{
				Time: e.Time.Format("15:04:05"), Action: e.Action, Field: e.Field, From: e.From, To: e.To,
			})
		}
		return out
	}
}

// tuiManagedTree renders the managed processes with the SAME grouping/tree as the
// browser (PM-0311/0312): it samples live processes, keeps only the managed pids,
// groups them via the engine per the current flags, and buckets exited/unresolved
// bindings under a built-in "orphans" group (ghost rows carrying the persisted
// pid + name). Names of live bindings are backfilled + saved so orphans stay
// labelled after exit.
func (a *assembly) tuiManagedTree(ctx context.Context) tui.RefreshFunc {
	sampler := collect.NewSampler(a.src, a.clk)
	return func(f queryspec.Flags) (*query.Result, []render.Column, error) {
		r, err := queryspec.Build(a.reg, a.dims, f)
		if err != nil {
			return nil, nil, err
		}
		configureSource(a.src, r)
		snap, err := sampler.Sample(ctx, r.Needed)
		if err != nil {
			return nil, nil, err
		}
		a.applyResolvers(snap.Processes)
		a.enrich(ctx, snap.Processes, r.Needed)

		managed, names := a.managedPIDs(snap.Processes)
		live := make([]model.Process, 0, len(managed))
		liveSeen := map[int]bool{}
		for i := range snap.Processes {
			if managed[snap.Processes[i].PID] {
				live = append(live, snap.Processes[i])
				liveSeen[snap.Processes[i].PID] = true
			}
		}
		res, err := a.engine.Build(query.Input{
			Generation: snap.Generation, WallTime: snap.WallTime, Elapsed: snap.Elapsed, Processes: live,
		}, r.Spec)
		if err != nil {
			return nil, nil, err
		}
		if orphans := orphanGroup(managed, liveSeen, names); orphans != nil {
			res.Rows = append(res.Rows, orphans)
		}
		cols, err := render.ResolveColumnsMode(a.reg, r.Columns, r.Human)
		if err != nil {
			return nil, nil, err
		}
		return res, cols, nil
	}
}

// managedPIDs returns the set of managed pids and their display names, backfilling
// (and persisting) each binding's name from the matching live process.
func (a *assembly) managedPIDs(live []model.Process) (map[int]bool, map[int]string) {
	managed, names := map[int]bool{}, map[int]string{}
	dir, _ := resolveStateDir("")
	c, err := newControlAsmFn(dir)
	if err != nil {
		return managed, names
	}
	byPID := map[int]string{}
	for i := range live {
		byPID[live[i].PID] = live[i].DisplayName()
	}
	changed := false
	st := c.mgr.State()
	for ti := range st.Targets {
		for bi := range st.Targets[ti].Bindings {
			b := &st.Targets[ti].Bindings[bi]
			managed[b.PID] = true
			if n, ok := byPID[b.PID]; ok && n != "" && n != b.Name {
				b.Name = n // capture the name while alive, for later orphan display
				changed = true
			}
			names[b.PID] = b.Name
		}
	}
	if changed {
		_ = c.save()
	}
	return managed, names
}

// orphanGroup builds the synthetic "orphans" group for managed pids that are no
// longer live, as ghost process rows labelled by their persisted name/pid.
func orphanGroup(managed, liveSeen map[int]bool, names map[int]string) *query.Row {
	var sub []*query.Row
	for pid := range managed {
		if liveSeen[pid] {
			continue
		}
		label := names[pid]
		if label == "" {
			label = fmt.Sprintf("pid:%d", pid)
		}
		sub = append(sub, &query.Row{
			Kind: query.RowProcess, Key: fmt.Sprintf("orphan:%d", pid), Label: label,
			Procs: 1, Leaves: 1, Process: &model.Process{PID: pid, Comm: names[pid]},
		})
	}
	if len(sub) == 0 {
		return nil
	}
	sort.Slice(sub, func(i, j int) bool { return sub[i].Label < sub[j].Label })
	return &query.Row{
		Kind: query.RowGroup, Key: "orphans", Label: "orphans",
		Children: len(sub), Leaves: len(sub), Sub: sub,
	}
}

// tuiDrop unmanages the target(s) owning the selected pids (managed-panel `d`).
func tuiDrop() tui.DropFunc {
	return func(pids []int) (string, error) {
		dir, _ := resolveStateDir("")
		c, err := newControlAsmFn(dir)
		if err != nil {
			return "", err
		}
		targets := targetsForPIDs(c, pids)
		if len(targets) == 0 {
			return "", fmt.Errorf("no managed target for the selection")
		}
		for _, name := range targets {
			if err := c.mgr.Unmanage(name); err != nil {
				return "", err
			}
		}
		if err := c.save(); err != nil {
			return "", err
		}
		return fmt.Sprintf("dropped %d target(s)", len(targets)), nil
	}
}

// targetsForPIDs returns the distinct managed target names that bind any of pids.
func targetsForPIDs(c *ctlAsm, pids []int) []string {
	want := map[int]bool{}
	for _, p := range pids {
		want[p] = true
	}
	var names []string
	seen := map[string]bool{}
	st := c.mgr.State()
	for ti := range st.Targets {
		t := &st.Targets[ti]
		for _, b := range t.Bindings {
			if want[b.PID] && !seen[t.Name] {
				seen[t.Name] = true
				names = append(names, t.Name)
			}
		}
	}
	return names
}

// pickExistingTarget returns the single managed target covering pids, or "".
func pickExistingTarget(c *ctlAsm, pids []int) string {
	if n := targetsForPIDs(c, pids); len(n) == 1 {
		return n[0]
	}
	return ""
}

// instancesForPIDs resolves each pid to a live instance, skipping any that
// vanished; the caller treats an empty result as "nothing to do".
func (c *ctlAsm) instancesForPIDs(pids []int) []control.Instance {
	var out []control.Instance
	for _, pid := range pids {
		insts, err := c.instancesForPID(pid)
		if err != nil {
			continue
		}
		out = append(out, insts...)
	}
	return out
}

// applyTUIControl performs one control action. Nice has a real dry-run through
// the manager (surfacing safeguards); the other actions have no dry-run at the
// manager level, so their preview is synthesized and only the confirmed apply
// touches the process.
func applyTUIControl(c *ctlAsm, name string, req tui.ControlRequest) (string, error) {
	if req.Kind == tui.CtrlNice {
		res, err := c.mgr.SetNice(name, req.Nice, req.DryRun)
		if err != nil {
			return "", err
		}
		if req.DryRun {
			return niceForecast(c, req.Nice, res), nil
		}
		return fmt.Sprintf("nice→%d  %s", req.Nice, summarizeResult(res)), nil
	}
	if req.DryRun {
		if req.Kind == tui.CtrlFreeze {
			return freezeForecast(c, req), nil
		}
		return synthForecast(req), nil
	}
	res, err := applyControlKind(c, name, req.Kind)
	if err != nil {
		return "", err
	}
	return summarizeResult(res), nil
}

// applyControlKind runs the non-nice control action on a named target. The TUI
// never force-freezes; the freeze safeguard applies.
func applyControlKind(c *ctlAsm, name string, kind tui.ControlKind) (*control.ApplyResult, error) {
	switch kind {
	case tui.CtrlRestoreNice:
		return c.mgr.RestoreNice(name)
	case tui.CtrlStop:
		r, _ := c.mgr.SetStop(name, true)
		return r, nil
	case tui.CtrlContinue:
		r, _ := c.mgr.SetStop(name, false)
		return r, nil
	case tui.CtrlFreeze:
		r, _ := c.mgr.SetFreeze(name, true, false, false)
		return r, nil
	case tui.CtrlThaw:
		r, _ := c.mgr.SetFreeze(name, false, false, false)
		return r, nil
	case tui.CtrlRestore:
		return c.mgr.Restore(name, false)
	}
	return nil, nil
}

// niceForecast builds the §19.3 preview for a group renice: a summary line, then
// a per-pid capture→change line (current nice → desired, with the would-be
// status incl. protected), and a privilege/restore warning when the change
// raises priority (lowering nice needs CAP_SYS_NICE and may not be restorable).
func niceForecast(c *ctlAsm, desired int, res *control.ApplyResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "renice → %d  (%s)", desired, summarizeResult(res))
	needsPriv := false
	for _, r := range res.Results {
		cur := "?"
		if n, err := c.ctrl.GetNice(r.PID); err == nil {
			cur = strconv.Itoa(n)
			if desired < n {
				needsPriv = true
			}
		}
		fmt.Fprintf(&b, "\n  pid %d: %s→%d  %s", r.PID, cur, desired, forecastStatus(r.Status))
	}
	if needsPriv {
		b.WriteString("\n  ! raising priority (lower nice) needs CAP_SYS_NICE; may be denied and not restorable unprivileged")
	}
	return b.String()
}

// freezeForecast previews a freeze: the distinct cgroup(s) it would pause, each
// flagged when the blast-radius guard would REFUSE it (session/self) — so the
// user sees "this freezes your session" before confirming, not after.
func freezeForecast(c *ctlAsm, req tui.ControlRequest) string {
	selfCg, _ := c.ctrl.ReadCgroupOf(os.Getpid())
	seen := map[string]bool{}
	var b strings.Builder
	fmt.Fprintf(&b, "freeze cgroup subtree(s) for %d process(es) [%s]", len(req.PIDs), req.Label)
	for _, pid := range req.PIDs {
		cg, ok := c.ctrl.ReadCgroupOf(pid)
		if !ok || cg == "" || seen[cg] {
			continue
		}
		seen[cg] = true
		line := "\n  " + cg
		if allowed, reason := control.FreezeSafety(cg, selfCg); !allowed {
			line += "   ⚠ REFUSED: " + reason + " (CLI --force to override)"
		}
		b.WriteString(line)
	}
	return b.String()
}

// synthForecast previews a non-nice group action (no manager dry-run exists): a
// summary line plus the affected pid list.
func synthForecast(req tui.ControlRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "would %s %d process(es) [%s]", req.Kind.Verb(), len(req.PIDs), req.Label)
	for _, pid := range req.PIDs {
		fmt.Fprintf(&b, "\n  pid %d", pid)
	}
	return b.String()
}

// forecastStatus relabels the dry-run "unchanged" marker as "would apply" so the
// preview reads as a forecast rather than a no-op.
func forecastStatus(s control.FieldStatus) string {
	if s == control.StatusUnchanged {
		return "would apply"
	}
	return string(s)
}

// summarizeResult renders an ApplyResult as a compact one-line status: per-pid
// for a single process, or aggregated status counts for a group.
func summarizeResult(res *control.ApplyResult) string {
	if res == nil || len(res.Results) == 0 {
		return "no matching instances"
	}
	if len(res.Results) == 1 {
		r := res.Results[0]
		if r.Error != "" {
			return fmt.Sprintf("pid %d: %s (%s)", r.PID, r.Status, r.Error)
		}
		return fmt.Sprintf("pid %d: %s", r.PID, r.Status)
	}
	parts := make([]string, 0, len(res.Counts))
	for status, n := range res.Counts {
		parts = append(parts, fmt.Sprintf("%s=%d", status, n))
	}
	sort.Strings(parts)
	return fmt.Sprintf("%d procs: %s", len(res.Results), strings.Join(parts, " "))
}
