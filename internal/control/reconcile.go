package control

// PolicySpec is a config-defined managed policy (RFC §6.6, §9.5): a persistent
// follow-mode target with an optional desired control and a drift mode.
type PolicySpec struct {
	Name     string
	Selector string
	Nice     *int
	Stop     *bool
	OnDrift  string
}

// SetPolicies installs config-defined policy targets, replacing any previous
// config-origin targets while leaving runtime-manual targets untouched. Existing
// policy targets keep their captured bindings so originals are not lost on reload.
func (m *Manager) SetPolicies(specs []PolicySpec) {
	prev := map[string]Target{}
	kept := make([]Target, 0, len(m.state.Targets))
	for _, t := range m.state.Targets {
		if t.Origin == OriginConfig {
			prev[t.Name] = t
			continue
		}
		kept = append(kept, t)
	}
	for _, s := range specs {
		t := Target{
			ID: "config:" + s.Name, Name: s.Name, Origin: OriginConfig,
			BindingMode: ModeFollow, Selector: s.Selector,
			DesiredNice: s.Nice, DesiredStop: s.Stop, OnDrift: normalizeDrift(s.OnDrift),
		}
		if old, ok := prev[s.Name]; ok {
			t.Bindings = old.Bindings // preserve captured originals across reloads
		}
		kept = append(kept, t)
	}
	m.state.Targets = kept
}

func normalizeDrift(mode string) string {
	switch mode {
	case "reapply", "adopt", "ignore":
		return mode
	default:
		return "report"
	}
}

// Reconcile re-resolves every config policy target against the current process
// set (via resolve) and enforces desired control (RFC §15.7): new matches
// capture their original before the first apply; drift on existing bindings is
// handled per the target's OnDrift mode.
func (m *Manager) Reconcile(resolve func(selector string) ([]Instance, error)) {
	for i := range m.state.Targets {
		t := &m.state.Targets[i]
		if t.Origin != OriginConfig {
			continue
		}
		insts, err := resolve(t.Selector)
		if err != nil {
			continue
		}
		m.bindInstances(t, insts)
		if t.DesiredNice != nil {
			m.reconcileNice(t)
		}
	}
	m.state.UpdatedAt = m.clk.Now()
}

func (m *Manager) reconcileNice(t *Target) {
	res := newApplyResult()
	for j := range t.Bindings {
		b := &t.Bindings[j]
		if b.Nice == nil {
			m.applyNiceBinding(b, *t.DesiredNice, false, res) // new match: capture + apply
			continue
		}
		m.reconcileDrift(b, *t.DesiredNice, t.OnDrift)
	}
}

// reconcileDrift enforces the drift policy for a binding whose original is
// already captured. reapply is guarded and only re-applies when observed differs
// from desired, so two controllers cannot fight every tick unnecessarily.
func (m *Manager) reconcileDrift(b *Binding, desired int, mode string) {
	if !m.revalidate(b) {
		b.Nice.Status = StatusStale
		return
	}
	obs, err := m.ctrl.GetNice(b.PID)
	if err != nil {
		b.Nice.Status = StatusVanished
		return
	}
	b.Nice.Observed = obs
	b.Nice.ObservedAt = m.clk.Now()
	if obs == desired {
		b.Nice.Status = StatusApplied
		return
	}
	switch mode {
	case "reapply":
		if err := m.ctrl.SetNice(b.PID, desired); err == nil {
			b.Nice.Desired = desired
			m.observeNice(b)
		}
	case "adopt":
		b.Nice.Desired = obs // treat observed as new desired; original preserved
		b.Nice.Status = StatusApplied
	case "ignore":
		b.Nice.Status = StatusApplied
	default: // report
		b.Nice.Status = StatusDrifted
	}
}
