package control

import (
	"fmt"

	"github.com/netikras/procfit/internal/ports"
)

// Restore returns controlled fields to their captured originals. It refuses to
// overwrite externally-drifted state unless force is set (RFC §15.8): a drifted
// binding yields StatusDrifted and the caller maps that to exit code 7.
func (m *Manager) Restore(target string, force bool) (*ApplyResult, error) {
	t := m.Find(target)
	if t == nil {
		return nil, fmt.Errorf("target %q not found", target)
	}
	res := newApplyResult()
	for i := range t.Bindings {
		m.restoreNiceBinding(&t.Bindings[i], force, res)
	}
	t.DesiredNice = nil
	m.state.UpdatedAt = m.clk.Now()
	return res, nil
}

func (m *Manager) restoreNiceBinding(b *Binding, force bool, res *ApplyResult) {
	if b.Nice == nil {
		res.add(b.PID, StatusUnchanged, "no captured original")
		return
	}
	if !m.revalidate(b) {
		res.add(b.PID, StatusStale, "identity changed (pid reused)")
		return
	}
	// Detect external drift before overwriting (RFC §15.8).
	obs, err := m.ctrl.GetNice(b.PID)
	if err != nil {
		res.add(b.PID, StatusVanished, err.Error())
		return
	}
	if obs != b.Nice.Desired && !force {
		b.Nice.Observed = obs
		b.Nice.Status = StatusDrifted
		res.add(b.PID, StatusDrifted, "observed value drifted from desired; refuse to overwrite (use --force)")
		return
	}
	if err := m.ctrl.SetNice(b.PID, b.Nice.Original); err != nil {
		res.add(b.PID, StatusDenied, err.Error())
		return
	}
	b.Nice.Desired = b.Nice.Original
	m.observeNice(b)
	b.Nice.Status = StatusRestored
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
func (m *Manager) Signal(instances []Instance, sig ports.Signal) *ApplyResult {
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
func (m *Manager) SetStop(target string, stop bool) (*ApplyResult, error) {
	t := m.Find(target)
	if t == nil {
		return nil, fmt.Errorf("target %q not found", target)
	}
	t.DesiredStop = &stop
	sig := ports.SigStop
	if !stop {
		sig = ports.SigCont
	}
	res := newApplyResult()
	for i := range t.Bindings {
		m.applyStopBinding(&t.Bindings[i], stop, sig, res)
	}
	m.state.UpdatedAt = m.clk.Now()
	return res, nil
}

func (m *Manager) applyStopBinding(b *Binding, stop bool, sig ports.Signal, res *ApplyResult) {
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
	res.add(b.PID, StatusApplied, "")
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
