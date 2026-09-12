package daemon

import (
	"context"
	"sync"
	"time"

	"github.com/netikras/procfit/internal/collect"
	"github.com/netikras/procfit/internal/control"
	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/ports"
	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/queryspec"
)

// Engine is the daemon's shared collection + query/control core. One sampler
// feeds all clients (RFC §17): the background loop maintains the latest immutable
// generation, and query handlers build results from it without re-sampling.
type Engine struct {
	reg       *metrics.Registry
	dims      *query.Dimensions
	qengine   *query.Engine
	sampler   *collect.Sampler
	resolvers []ports.Resolver
	mgr       *control.Manager
	ctrl      ports.Controller

	interval time.Duration
	started  time.Time

	mu         sync.RWMutex
	latest     []model.Process
	generation int
	wallTime   time.Time
	elapsed    time.Duration
	lastScan   time.Duration
}

// NewEngine builds the engine over injected ports (so tests use fakes).
func NewEngine(reg *metrics.Registry, dims *query.Dimensions, src ports.ProcessSource, ctrl ports.Controller, clk ports.Clock, mgr *control.Manager, resolvers []ports.Resolver, interval time.Duration) *Engine {
	return &Engine{
		reg: reg, dims: dims, qengine: query.NewEngine(reg, dims),
		sampler: collect.NewSampler(src, clk), resolvers: resolvers, mgr: mgr, ctrl: ctrl,
		interval: interval, started: clk.Now(),
	}
}

// ResolveInstances resolves a target spec against the latest generation into
// concrete instances (RFC §8.5): pid:, managed:, selector:, group:, bare pid.
func (e *Engine) ResolveInstances(spec string) ([]control.Instance, error) {
	kind, arg, hasPrefix := splitTarget(spec)
	switch {
	case !hasPrefix:
		return e.instancesForPID(spec)
	case kind == "pid":
		return e.instancesForPID(arg)
	case kind == "managed":
		return e.mgr.InstancesOf(arg)
	case kind == "selector":
		return e.instancesBySelector(arg)
	case kind == "group":
		return e.instancesByGroup(arg)
	default:
		return nil, errUnknownTargetKind(kind)
	}
}

// lightMetrics is the metric set the shared loop computes every tick.
func (e *Engine) lightMetrics() []model.MetricID {
	ids, _ := e.reg.Resolve(metrics.ProfileLight)
	return ids
}

// Tick performs one sampling generation. Exposed for deterministic tests; Run
// calls it on an interval.
func (e *Engine) Tick(ctx context.Context) error {
	start := time.Now()
	snap, err := e.sampler.Sample(ctx, e.lightMetrics())
	if err != nil {
		return err
	}
	for i := range snap.Processes {
		for _, r := range e.resolvers {
			r.Resolve(&snap.Processes[i])
		}
	}
	e.mu.Lock()
	e.latest = snap.Processes
	e.generation = snap.Generation
	e.wallTime = snap.WallTime
	e.elapsed = snap.Elapsed
	e.lastScan = time.Since(start)
	e.mu.Unlock()
	return nil
}

// Run samples continuously until the context is cancelled.
func (e *Engine) Run(ctx context.Context) error {
	if err := e.Tick(ctx); err != nil && ctx.Err() == nil {
		return err
	}
	t := time.NewTicker(e.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			if err := e.Tick(ctx); err != nil && ctx.Err() == nil {
				return err
			}
		}
	}
}

func (e *Engine) snapshotInput() query.Input {
	e.mu.RLock()
	defer e.mu.RUnlock()
	procs := make([]model.Process, len(e.latest))
	copy(procs, e.latest)
	return query.Input{Generation: e.generation, WallTime: e.wallTime, Elapsed: e.elapsed, Processes: procs}
}

// RunQuery builds a query result from the latest shared generation.
func (e *Engine) RunQuery(f queryspec.Flags) (*query.Result, []string, error) {
	r, err := queryspec.Build(e.reg, e.dims, f)
	if err != nil {
		return nil, nil, err
	}
	res, err := e.qengine.Build(e.snapshotInput(), r.Spec)
	if err != nil {
		return nil, nil, err
	}
	return res, r.Columns, nil
}

// Health reports engine statistics (RFC §23).
func (e *Engine) Health(clients int) HealthResp {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return HealthResp{
		UptimeSeconds:  time.Since(e.started).Seconds(),
		Generation:     e.generation,
		Clients:        clients,
		LastScanMillis: float64(e.lastScan.Microseconds()) / 1000,
		Processes:      len(e.latest),
	}
}

// Manager exposes the control manager (single writer of runtime state).
func (e *Engine) Manager() *control.Manager { return e.mgr }
