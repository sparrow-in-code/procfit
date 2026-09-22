package control

import (
	"fmt"
	"strconv"

	"github.com/netikras/procfit/internal/ports"
)

// Restore returns controlled fields to their captured originals. It refuses to
// overwrite externally-drifted state unless force is set (RFC §15.8): a drifted
// binding yields StatusDrifted and the caller maps that to exit code 7.
func (m *Manager) Restore(target string, force, dryRun bool) (*ApplyResult, error) {
	t := m.Find(target)
	if t == nil {
		return nil, fmt.Errorf("target %q not found", target)
	}
	res := newApplyResult()
	// Thaw FIRST: a frozen task can block /proc reads, so unblock it before
	// revalidating/renicing (otherwise restore could hang on a frozen target).
	if t.DesiredFreeze != nil && *t.DesiredFreeze {
		done := map[string]bool{}
		for i := range t.Bindings {
			m.freezeBinding(&t.Bindings[i], false, false, dryRun, done, res)
		}
	}
	for i := range t.Bindings {
		m.restoreNiceBinding(&t.Bindings[i], force, dryRun, res)
		m.restoreStopBinding(&t.Bindings[i], dryRun, res)
	}
	if !dryRun {
		t.DesiredNice, t.DesiredStop, t.DesiredFreeze = nil, nil, nil
	}
	m.state.UpdatedAt = m.clk.Now()
	return res, nil
}

// RestoreNice reverts only the nice field of a target to its captured original
// (the uppercase "lift" of a renice), clearing the nice intent.
func (m *Manager) RestoreNice(target string, dryRun bool) (*ApplyResult, error) {
	t := m.Find(target)
	if t == nil {
		return nil, fmt.Errorf("target %q not found", target)
	}
	res := newApplyResult()
	for i := range t.Bindings {
		m.restoreNiceBinding(&t.Bindings[i], false, dryRun, res)
	}
	if !dryRun {
		t.DesiredNice = nil
	}
	m.state.UpdatedAt = m.clk.Now()
	return res, nil
}

// restoreStopBinding resumes a process procfit itself stopped (SIGCONT).
func (m *Manager) restoreStopBinding(b *Binding, dryRun bool, res *ApplyResult) {
	if b.Stop == nil || !b.Stop.DesiredStop {
		return // not stopped by procfit; nothing to undo
	}
	if !m.revalidate(b) {
		res.add(b.PID, StatusStale, "identity changed (pid reused)")
		return
	}
	if dryRun {
		res.add(b.PID, StatusUnchanged, "dry-run: would continue (SIGCONT)")
		return
	}
	if err := m.ctrl.SendSignal(b.PID, ports.SigCont); err != nil {
		res.add(b.PID, StatusDenied, err.Error())
		return
	}
	b.Stop.DesiredStop = false
	b.Stop.Status = StatusRestored
	m.audit("continued", b.PID, "stop", "", "false")
	res.add(b.PID, StatusRestored, "")
}

func (m *Manager) restoreNiceBinding(b *Binding, force, dryRun bool, res *ApplyResult) {
	if b.Nice == nil {
		return // nothing to restore for nice; stop/freeze handled separately
	}
	if !m.revalidate(b) {
		res.add(b.PID, StatusStale, "identity changed (pid reused)")
		return
	}
	// Detect external drift before overwriting (RFC §15.8). This is a read, so it
	// is safe to evaluate even in dry-run to preview a refuse-on-drift outcome.
	obs, err := m.ctrl.GetNice(b.PID)
	if err != nil {
		res.add(b.PID, StatusVanished, err.Error())
		return
	}
	if obs != b.Nice.Desired && !force {
		if dryRun {
			res.add(b.PID, StatusDrifted, "dry-run: would refuse (drifted; use --force)")
			return
		}
		b.Nice.Observed = obs
		b.Nice.Status = StatusDrifted
		res.add(b.PID, StatusDrifted, "observed value drifted from desired; refuse to overwrite (use --force)")
		return
	}
	if dryRun {
		res.add(b.PID, StatusUnchanged, fmt.Sprintf("dry-run: would restore nice→%d", b.Nice.Original))
		return
	}
	if err := m.ctrl.SetNice(b.PID, b.Nice.Original); err != nil {
		res.add(b.PID, StatusDenied, err.Error())
		return
	}
	b.Nice.Desired = b.Nice.Original
	m.observeNice(b)
	b.Nice.Status = StatusRestored
	m.audit("restored", b.PID, "nice", "", strconv.Itoa(b.Nice.Original))
	res.add(b.PID, StatusRestored, "")
}

// Unmanage removes a target's membership. The kernel state is left untouched
// (RFC §15.8): restore must be requested explicitly.
func (m *Manager) Unmanage(target string) error {
	for i := range m.state.Targets {
		if m.state.Targets[i].ID == target || m.state.Targets[i].Name == target {
			m.state.Targets = append(m.state.Targets[:i], m.state.Targets[i+1:]...)
			m.state.UpdatedAt = m.clk.Now()
			return nil
		}
	}
	return fmt.Errorf("target %q not found", target)
}

// Signal delivers a signal to a set of instances, applying safeguards and
// revalidating identity first (RFC §21.2). Destructive-signal confirmation is a
// caller (CLI) concern.
func (m *Manager) Signal(instances []Instance, sig ports.Signal, dryRun bool) *ApplyResult {
	res := newApplyResult()
	for _, in := range instances {
		if ok, _ := m.sg.CheckPID(in.PID); !ok {
			res.add(in.PID, StatusSkipped, "protected")
			continue
		}
		id, ok := m.ctrl.ReadIdentity(in.PID)
		if !ok || !id.SameProcess(in.ID) {
			res.add(in.PID, StatusStale, "identity changed or vanished")
			continue
		}
		if dryRun {
			res.add(in.PID, StatusUnchanged, "dry-run: would send SIG"+string(sig))
			continue
		}
		if err := m.ctrl.SendSignal(in.PID, sig); err != nil {
			res.add(in.PID, StatusFailed, err.Error())
			continue
		}
		res.add(in.PID, StatusApplied, "")
	}
	return res
}

// SetStop records procfit's SIGSTOP/SIGCONT intent for a target and delivers the
// signal. The intent is tracked separately from the observed kernel state
// (RFC §15.4); a pre-stopped task is never blindly continued on restore.
func (m *Manager) SetStop(target string, stop, dryRun bool) (*ApplyResult, error) {
	t := m.Find(target)
	if t == nil {
		return nil, fmt.Errorf("target %q not found", target)
	}
	if !dryRun {
		t.DesiredStop = &stop
	}
	sig := ports.SigStop
	if !stop {
		sig = ports.SigCont
	}
	res := newApplyResult()
	for i := range t.Bindings {
		m.applyStopBinding(&t.Bindings[i], stop, sig, dryRun, res)
	}
	m.state.UpdatedAt = m.clk.Now()
	return res, nil
}

func (m *Manager) applyStopBinding(b *Binding, stop bool, sig ports.Signal, dryRun bool, res *ApplyResult) {
	if ok, _ := m.sg.CheckPID(b.PID); !ok {
		res.add(b.PID, StatusSkipped, "protected")
		return
	}
	if !m.revalidate(b) {
		res.add(b.PID, StatusStale, "identity changed (pid reused)")
		return
	}
	// Continue is only honoured for stops procfit owns (RFC §15.4).
	if !stop && (b.Stop == nil || !b.Stop.DesiredStop) {
		res.add(b.PID, StatusSkipped, "not stopped by procfit; refusing to continue")
		return
	}
	if dryRun {
		verb := "stop (SIGSTOP)"
		if !stop {
			verb = "continue (SIGCONT)"
		}
		res.add(b.PID, StatusUnchanged, "dry-run: would "+verb)
		return
	}
	if err := m.ctrl.SendSignal(b.PID, sig); err != nil {
		res.add(b.PID, StatusFailed, err.Error())
		return
	}
	if b.Stop == nil {
		b.Stop = &StopField{}
	}
	b.Stop.DesiredStop = stop
	b.Stop.ChangedAt = m.clk.Now()
	b.Stop.Status = StatusApplied
	action := "stopped"
	if !stop {
		action = "continued"
	}
	m.audit(action, b.PID, "stop", "", strconv.FormatBool(stop))
	res.add(b.PID, StatusApplied, "")
}

// SetFreeze freezes or thaws the cgroup(s) backing a target's bindings (RFC
// §15.5). It operates on existing cgroups only; each distinct cgroup is acted on
// once. Freeze intent is tracked on the target.
func (m *Manager) SetFreeze(target string, freeze, force, dryRun bool) (*ApplyResult, error) {
	t := m.Find(target)
	if t == nil {
		return nil, fmt.Errorf("target %q not found", target)
	}
	if !dryRun {
		t.DesiredFreeze = &freeze
	}
	res := newApplyResult()
	done := map[string]bool{}
	for i := range t.Bindings {
		m.freezeBinding(&t.Bindings[i], freeze, force, dryRun, done, res)
	}
	m.state.UpdatedAt = m.clk.Now()
	return res, nil
}

func (m *Manager) freezeBinding(b *Binding, freeze, force, dryRun bool, done map[string]bool, res *ApplyResult) {
	if ok, _ := m.sg.CheckPID(b.PID); !ok {
		res.add(b.PID, StatusSkipped, "protected")
		return
	}
	cg, ok := m.ctrl.ReadCgroupOf(b.PID)
	if !ok || cg == "" {
		res.add(b.PID, StatusUnavailable, "no cgroup")
		return
	}
	if done[cg] {
		res.add(b.PID, StatusUnchanged, "cgroup already handled")
		return
	}
	done[cg] = true
	// Blast-radius guard: freezing a cgroup pauses its whole subtree, so refuse to
	// freeze our own session / a user slice unless forced. Thaw is never gated.
	if freeze && !force {
		if ok, reason := m.freezeAllowed(cg); !ok {
			res.add(b.PID, StatusSkipped, reason)
			return
		}
	}
	if dryRun {
		verb := "freeze"
		if !freeze {
			verb = "thaw"
		}
		res.add(b.PID, StatusUnchanged, "dry-run: would "+verb+" "+cg)
		return
	}
	if err := m.ctrl.FreezeCgroup(cg, freeze); err != nil {
		res.add(b.PID, StatusFailed, err.Error())
		return
	}
	action := "frozen"
	if !freeze {
		action = "thawed"
	}
	m.audit(action, b.PID, "freeze", "", cg)
	res.add(b.PID, StatusApplied, "")
}

// freezeAllowed applies the blast-radius safety check against the cgroup, using
// procfit's own cgroup to detect a self/session freeze.
func (m *Manager) freezeAllowed(cg string) (bool, string) {
	selfCg, _ := m.ctrl.ReadCgroupOf(m.sg.SelfPID)
	return FreezeSafety(cg, selfCg)
}

// InstancesOf returns the concrete instances currently bound to a managed
// target, for control operations that address it by name.
func (m *Manager) InstancesOf(name string) ([]Instance, error) {
	t := m.Find(name)
	if t == nil {
		return nil, fmt.Errorf("managed target %q not found", name)
	}
	out := make([]Instance, 0, len(t.Bindings))
	for _, b := range t.Bindings {
		out = append(out, Instance{ID: b.ID, PID: b.PID})
	}
	return out, nil
}

// Rebind refreshes a follow target's bindings from a freshly resolved instance
// set, preserving captured control fields and retaining the target when the set
// is empty (INACTIVE, RFC §16.6).
func (m *Manager) Rebind(target string, instances []Instance) error {
	t := m.Find(target)
	if t == nil {
		return fmt.Errorf("target %q not found", target)
	}
	if t.BindingMode == ModeSnapshot {
		return nil // snapshot targets never rebind (RFC §16.6)
	}
	m.bindInstances(t, instances)
	return nil
}
