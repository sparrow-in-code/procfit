package testutil

import (
	"fmt"

	"github.com/netikras/procfit/internal/ports"
)

// FakeController is an in-memory ports.Controller for testing the control plane
// without touching real processes. It models nice values, delivered signals, and
// per-pid identity so stale-PID refusal can be exercised.
type FakeController struct {
	Caps       ports.ControlCaps
	Nice       map[int]int
	Identity   map[int]ports.ProcessInstanceIdentity
	Signals    map[int][]ports.Signal
	SetNiceErr map[int]error   // optional per-pid error (e.g. permission denied)
	Cgroups    map[int]string  // pid -> cgroup rel path
	Frozen     map[string]bool // cgroup rel path -> frozen
	FreezeErr  map[string]error
}

// NewFakeController returns a controller with nice+signal capabilities.
func NewFakeController() *FakeController {
	return &FakeController{
		Caps:       ports.ControlCaps{Nice: true, Signal: true, Freeze: true},
		Nice:       map[int]int{},
		Identity:   map[int]ports.ProcessInstanceIdentity{},
		Signals:    map[int][]ports.Signal{},
		SetNiceErr: map[int]error{},
		Cgroups:    map[int]string{},
		Frozen:     map[string]bool{},
		FreezeErr:  map[string]error{},
	}
}

// ReadCgroupOf returns the configured cgroup path for a pid.
func (f *FakeController) ReadCgroupOf(pid int) (string, bool) {
	p, ok := f.Cgroups[pid]
	return p, ok
}

// FreezeCgroup records the frozen state of a cgroup path.
func (f *FakeController) FreezeCgroup(rel string, freeze bool) error {
	if err := f.FreezeErr[rel]; err != nil {
		return err
	}
	f.Frozen[rel] = freeze
	return nil
}

// Add registers a process with a nice value and identity.
func (f *FakeController) Add(pid int, nice int, id ports.ProcessInstanceIdentity) {
	f.Nice[pid] = nice
	f.Identity[pid] = id
}

// Remove simulates a process vanishing.
func (f *FakeController) Remove(pid int) {
	delete(f.Nice, pid)
	delete(f.Identity, pid)
}

func (f *FakeController) Capabilities() ports.ControlCaps { return f.Caps }

func (f *FakeController) GetNice(pid int) (int, error) {
	n, ok := f.Nice[pid]
	if !ok {
		return 0, fmt.Errorf("pid %d not found", pid)
	}
	return n, nil
}

func (f *FakeController) SetNice(pid, nice int) error {
	if err := f.SetNiceErr[pid]; err != nil {
		return err
	}
	if _, ok := f.Nice[pid]; !ok {
		return fmt.Errorf("pid %d not found", pid)
	}
	f.Nice[pid] = nice
	return nil
}

func (f *FakeController) SendSignal(pid int, sig ports.Signal) error {
	if _, ok := f.Identity[pid]; !ok {
		return fmt.Errorf("pid %d not found", pid)
	}
	f.Signals[pid] = append(f.Signals[pid], sig)
	return nil
}

func (f *FakeController) ReadIdentity(pid int) (ports.ProcessInstanceIdentity, bool) {
	id, ok := f.Identity[pid]
	return id, ok
}
