package app

import (
	"context"
	"time"

	"github.com/netikras/procfit/internal/collect"
	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/ports"
	"github.com/netikras/procfit/internal/procfs"
	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/queryspec"
	"github.com/netikras/procfit/internal/resolve"
	"github.com/netikras/procfit/internal/sysclock"
)

// assembly wires the observation stack. It is constructed per command so tests
// can inject fakes for the source and clock.
type assembly struct {
	reg       *metrics.Registry
	dims      *query.Dimensions
	src       ports.ProcessSource
	clk       ports.Clock
	engine    *query.Engine
	resolvers []ports.Resolver
	// wait returns a channel that fires after d; injectable so tests can avoid
	// real sleeps and advance a fake clock instead.
	wait func(d time.Duration) <-chan time.Time
}

// newAssemblyFn is the assembly constructor commands use. It is a package var so
// tests can substitute a fake-backed assembly (dependency injection seam).
var newAssemblyFn = newAssembly

// newAssembly builds the production assembly over the live /proc.
func newAssembly() (*assembly, error) {
	src, err := procfs.New()
	if err != nil {
		return nil, err
	}
	return newAssemblyWith(src, sysclock.New()), nil
}

// newAssemblyWith builds an assembly over injected ports (for tests).
func newAssemblyWith(src ports.ProcessSource, clk ports.Clock) *assembly {
	reg := metrics.NewDefault()
	dims := query.NewDimensions()
	resolvers := []ports.Resolver{resolve.NewUserResolver(""), resolve.NewSystemdResolver()}
	return &assembly{reg: reg, dims: dims, src: src, clk: clk, engine: query.NewEngine(reg, dims), resolvers: resolvers, wait: time.After}
}

// applyResolvers decorates each process with derived labels (user, unit, …).
func (a *assembly) applyResolvers(procs []model.Process) {
	for i := range procs {
		for _, r := range a.resolvers {
			r.Resolve(&procs[i])
		}
	}
}

// warmup is the default delay between the two samples ps takes for rates.
const warmup = time.Second

// sampleForResult produces a query.Input, taking a warm-up second sample when
// rate metrics are requested unless instant is set (RFC §7.2).
func (a *assembly) sampleForResult(ctx context.Context, r queryspec.Resolved, instant bool) (query.Input, error) {
	s := collect.NewSampler(a.src, a.clk)
	snap, err := s.Sample(ctx, r.Needed)
	if err != nil {
		return query.Input{}, err
	}
	if !instant && a.hasRate(r.Needed) {
		select {
		case <-ctx.Done():
			return query.Input{}, ctx.Err()
		case <-a.wait(warmup):
		}
		snap, err = s.Sample(ctx, r.Needed)
		if err != nil {
			return query.Input{}, err
		}
	}
	a.applyResolvers(snap.Processes)
	return query.Input{
		Generation: snap.Generation,
		WallTime:   snap.WallTime,
		Elapsed:    snap.Elapsed,
		Processes:  snap.Processes,
	}, nil
}

// hasRate reports whether any requested metric is a rate (needs two samples).
func (a *assembly) hasRate(ids []model.MetricID) bool {
	for _, id := range ids {
		if d, ok := a.reg.Get(string(id)); ok && d.IsRate() {
			return true
		}
	}
	return false
}
