package testutil

import (
	"context"

	"github.com/netikras/procfit/internal/ports"
)

// FakeSource is a ports.ProcessSource backed by predefined generations. Each
// call to List returns the next generation, so tests can model counters
// increasing over time to exercise rate sampling. When generations are
// exhausted the last one repeats.
type FakeSource struct {
	Boot     string
	HZ       int64
	CPUs     int
	BootUnix int64
	Gens     [][]ports.ProcStat
	ListErr  error

	idx int
}

// NewFakeSource builds a source with sensible defaults and the given generations.
func NewFakeSource(gens ...[]ports.ProcStat) *FakeSource {
	return &FakeSource{Boot: "boot-test", HZ: 100, CPUs: 4, BootUnix: 1_600_000_000, Gens: gens}
}

// BootTimeUnix returns the configured boot time.
func (f *FakeSource) BootTimeUnix() int64 { return f.BootUnix }

// ID identifies the fake.
func (f *FakeSource) ID() string { return "fake" }

// BootID returns the configured boot id.
func (f *FakeSource) BootID() (string, error) { return f.Boot, nil }

// ClockTicksPerSec returns the configured USER_HZ.
func (f *FakeSource) ClockTicksPerSec() int64 { return f.HZ }

// OnlineCPUs returns the configured cpu count.
func (f *FakeSource) OnlineCPUs() int { return f.CPUs }

// List returns the next generation of process snapshots.
func (f *FakeSource) List(ctx context.Context) ([]ports.ProcStat, error) {
	if f.ListErr != nil {
		return nil, f.ListErr
	}
	if len(f.Gens) == 0 {
		return nil, nil
	}
	i := f.idx
	if i >= len(f.Gens) {
		i = len(f.Gens) - 1
	} else {
		f.idx++
	}
	return f.Gens[i], nil
}
