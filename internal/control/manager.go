package control

import (
	"fmt"

	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/ports"
)

// Instance is a resolved (identity, pid) pair the manager can act on.
type Instance struct {
	ID  model.ProcessInstanceID
	PID int
}

// BindingResult is the per-instance outcome of an apply/restore (RFC §21.4).
type BindingResult struct {
	PID    int
	Status FieldStatus
	Error  string
}

// ApplyResult aggregates per-binding outcomes.
type ApplyResult struct {
	Results []BindingResult
	Counts  map[FieldStatus]int
}

func newApplyResult() *ApplyResult { return &ApplyResult{Counts: map[FieldStatus]int{}} }

func (r *ApplyResult) add(pid int, status FieldStatus, err string) {
	r.Results = append(r.Results, BindingResult{PID: pid, Status: status, Error: err})
	r.Counts[status]++
}

// Manager owns the in-memory runtime state and performs control operations. It
// depends only on the Controller and Clock ports plus safeguards.
type Manager struct {
	ctrl   ports.Controller
	clk    ports.Clock
	sg     Safeguards
	bootID string
	state  *State
}

// NewManager constructs a manager with fresh state.
func NewManager(ctrl ports.Controller, clk ports.Clock, sg Safeguards, bootID string) *Manager {
	return &Manager{
		ctrl:   ctrl,
		clk:    clk,
		sg:     sg,
		bootID: bootID,
		state:  &State{SchemaVersion: SchemaVersion, BootID: bootID},
	}
}

// State returns the current runtime state.
func (m *Manager) State() *State { return m.state }

// LoadState adopts a persisted state, discarding concrete bindings on a boot-id
// mismatch while retaining logical targets (RFC §16.5).
func (m *Manager) LoadState(s *State) {
	if s == nil {
		return
	}
	if s.BootID != m.bootID {
		for i := range s.Targets {
			s.Targets[i].Bindings = nil
			s.Targets[i].Active = false
		}
		s.BootID = m.bootID
	}
	m.state = s
}

// Find returns a target by id or name.
func (m *Manager) Find(idOrName string) *Target {
	for i := range m.state.Targets {
		t := &m.state.Targets[i]
		if t.ID == idOrName || t.Name == idOrName {
			return t
		}
	}
	return nil
}

// Manage creates (or updates) a runtime managed target with the given instances
// as membership. Membership alone applies no control (RFC §6.6).
func (m *Manager) Manage(name string, mode BindingMode, selector string, instances []Instance) *Target {
	t := m.Find(name)
	if t == nil {
		m.state.Targets = append(m.state.Targets, Target{
			ID: "runtime:" + name, Name: name, Origin: OriginRuntime, BindingMode: mode, Selector: selector,
		})
		t = &m.state.Targets[len(m.state.Targets)-1]
	}
	t.BindingMode = mode
	if mode == ModeSnapshot {
		t.SnapshotIDs = nil
		for _, in := range instances {
			t.SnapshotIDs = append(t.SnapshotIDs, in.ID)
		}
	}
	m.bindInstances(t, instances)
	return t
}

// bindInstances refreshes a target's bindings from the resolved instances,
// preserving captured control fields for instances that persist.
func (m *Manager) bindInstances(t *Target, instances []Instance) {
	old := map[string]Binding{}
	for _, b := range t.Bindings {
		old[b.ID.Key()] = b
	}
	var next []Binding
	for _, in := range instances {
		b := Binding{ID: in.ID, PID: in.PID}
		if prev, ok := old[in.ID.Key()]; ok {
			b.Nice = prev.Nice
			b.Stop = prev.Stop
		}
		next = append(next, b)
	}
	t.Bindings = next
	t.Active = len(next) > 0
	if t.Active {
		t.LastSeen = m.clk.Now()
	}
	m.state.Generation++
	m.state.UpdatedAt = m.clk.Now()
}

// SetNice applies a desired nice value to all bindings of a target.
func (m *Manager) SetNice(target string, nice int, dryRun bool) (*ApplyResult, error) {
	t := m.Find(target)
	if t == nil {
		return nil, fmt.Errorf("target %q not found", target)
	}
	t.DesiredNice = &nice
	res := newApplyResult()
	for i := range t.Bindings {
		m.applyNiceBinding(&t.Bindings[i], nice, dryRun, res)
	}
	m.state.UpdatedAt = m.clk.Now()
	return res, nil
}

func (m *Manager) applyNiceBinding(b *Binding, desired int, dryRun bool, res *ApplyResult) {
	if ok, _ := m.sg.CheckPID(b.PID); !ok {
		res.add(b.PID, StatusSkipped, "protected")
		return
	}
	if !m.revalidate(b) {
		res.add(b.PID, StatusStale, "identity changed (pid reused)")
		return
	}
	if dryRun {
		res.add(b.PID, StatusUnchanged, "dry-run")
		return
	}
	// Capture original exactly once, before the first mutation (RFC §15.2).
	if b.Nice == nil {
		orig, err := m.ctrl.GetNice(b.PID)
		if err != nil {
			res.add(b.PID, StatusFailed, err.Error())
			return
		}
		b.Nice = &NiceField{Original: orig, Backend: "setpriority", CapturedAt: m.clk.Now()}
	}
	b.Nice.Desired = desired
	if err := m.ctrl.SetNice(b.PID, desired); err != nil {
		b.Nice.Status = StatusDenied
		b.Nice.Error = err.Error()
		res.add(b.PID, StatusDenied, err.Error())
		return
	}
	b.Nice.ChangedAt = m.clk.Now()
	m.observeNice(b)
	res.add(b.PID, b.Nice.Status, b.Nice.Error)
}

// observeNice reads the current nice value and updates status/drift.
func (m *Manager) observeNice(b *Binding) {
	obs, err := m.ctrl.GetNice(b.PID)
	if err != nil {
		b.Nice.Status = StatusVanished
		b.Nice.Error = err.Error()
		return
	}
	b.Nice.Observed = obs
	b.Nice.ObservedAt = m.clk.Now()
	if obs == b.Nice.Desired {
		b.Nice.Status = StatusApplied
		b.Nice.Error = ""
	} else {
		b.Nice.Status = StatusDrifted
	}
}

// revalidate re-reads identity immediately before acting (RFC §21.2).
func (m *Manager) revalidate(b *Binding) bool {
	id, ok := m.ctrl.ReadIdentity(b.PID)
	if !ok {
		return false
	}
	return id.SameProcess(b.ID)
}
