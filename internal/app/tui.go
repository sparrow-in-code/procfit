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
		"procfit tui                                  # nav g(group) t(leaf) s/S / u; control n(ice) x/c z/Z R (confirm y)",
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
		Refresh:       a.tuiRefresh(context.Background()),
		Control:       tuiControl(),
		Managed:       tuiManaged(),
		ManagedAction: tuiManagedAction(),
		History:       tuiHistory(),
		Interval:      *interval,
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

// tuiRefresh builds a RefreshFunc backed by a persistent sampler so rates warm up
// across ticks; each refresh resolves the flags, samples, enriches, and builds.
func (a *assembly) tuiRefresh(ctx context.Context) tui.RefreshFunc {
	sampler := collect.NewSampler(a.src, a.clk)
	return func(f queryspec.Flags) (*query.Result, []render.Column, error) {
		r, err := queryspec.Build(a.reg, a.dims, f)
		if err != nil {
			return nil, nil, err
		}
		setThreadEnum(a.src, r.Spec.Leaf == query.LeafThread)
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
		if name != "" {
			// Managed panel: act on the retained target as-is.
			if c.mgr.Find(name) == nil {
				return "", fmt.Errorf("managed target %q not found", name)
			}
		} else {
			// Browser: auto-manage the selected process(es) under a friendly name.
			insts := c.instancesForPIDs(req.PIDs)
			if len(insts) == 0 {
				return "", fmt.Errorf("no live instances for the selection")
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

// tuiManaged lists the current managed targets for the TUI managed panel.
func tuiManaged() tui.ManagedFunc {
	return func() []tui.ManagedRow {
		dir, _ := resolveStateDir("")
		c, err := newControlAsmFn(dir)
		if err != nil {
			return nil
		}
		st := c.mgr.State()
		rows := make([]tui.ManagedRow, 0, len(st.Targets))
		for _, t := range st.Targets {
			nice := ""
			if t.DesiredNice != nil {
				nice = strconv.Itoa(*t.DesiredNice)
			}
			seen := "-"
			if !t.LastSeen.IsZero() {
				seen = t.LastSeen.Format("15:04:05")
			}
			pids := make([]int, 0, len(t.Bindings))
			for _, b := range t.Bindings {
				pids = append(pids, b.PID)
			}
			rows = append(rows, tui.ManagedRow{
				Name: t.Name, Mode: string(t.BindingMode), Active: t.Active,
				Members: len(t.Bindings), PIDs: pids, Nice: nice,
				Stopped:  t.DesiredStop != nil && *t.DesiredStop,
				Frozen:   t.DesiredFreeze != nil && *t.DesiredFreeze,
				LastSeen: seen,
			})
		}
		return rows
	}
}

// tuiManagedAction restores or unmanages a target by name from the managed panel.
func tuiManagedAction() tui.ManagedActionFunc {
	return func(name string, kind tui.ManagedActionKind) (string, error) {
		dir, _ := resolveStateDir("")
		c, err := newControlAsmFn(dir)
		if err != nil {
			return "", err
		}
		switch kind {
		case tui.ManagedRestore:
			res, err := c.mgr.Restore(name, false)
			if err != nil {
				return "", err
			}
			if err := c.save(); err != nil {
				return "", err
			}
			return "restored " + name + ": " + summarizeResult(res), nil
		case tui.ManagedUnmanage:
			if err := c.mgr.Unmanage(name); err != nil {
				return "", err
			}
			if err := c.save(); err != nil {
				return "", err
			}
			return "unmanaged " + name, nil
		}
		return "", fmt.Errorf("unknown managed action")
	}
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
	var res *control.ApplyResult
	switch req.Kind {
	case tui.CtrlStop:
		res, _ = c.mgr.SetStop(name, true)
	case tui.CtrlContinue:
		res, _ = c.mgr.SetStop(name, false)
	case tui.CtrlFreeze:
		res, _ = c.mgr.SetFreeze(name, true, false, false) // TUI never force-freezes the session
	case tui.CtrlThaw:
		res, _ = c.mgr.SetFreeze(name, false, false, false)
	case tui.CtrlRestore:
		r, err := c.mgr.Restore(name, false)
		if err != nil {
			return "", err
		}
		res = r
	}
	return summarizeResult(res), nil
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
