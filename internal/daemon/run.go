package daemon

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/netikras/procfit/internal/control"
	"github.com/netikras/procfit/internal/history"
	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/ports"
	"github.com/netikras/procfit/internal/procfs"
	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/resolve"
	"github.com/netikras/procfit/internal/state"
	"github.com/netikras/procfit/internal/sysclock"
)

// Daemon bundles the engine, server, and state store for one running instance.
type Daemon struct {
	engine *Engine
	server *Server
	store  *state.Store
	reg    *metrics.Registry
	dims   *query.Dimensions
}

// New builds a production daemon over the live /proc and the given state
// directory and sample interval.
func New(stateDir string, interval time.Duration) (*Daemon, error) {
	src, err := procfs.New()
	if err != nil {
		return nil, err
	}
	ctrl := procfs.NewController(src)
	clk := sysclock.New()
	store, err := state.Open(stateDir)
	if err != nil {
		return nil, err
	}
	bootID, _ := src.BootID()
	mgr := control.NewManager(ctrl, clk, control.NewSafeguards(os.Getpid(), os.Getppid()), bootID)
	if st, ok, err := store.Load(); err == nil && ok {
		mgr.LoadState(st)
	}
	resolvers := []ports.Resolver{resolve.NewUserResolver(""), resolve.NewSystemdResolver()}
	reg := metrics.NewDefault()
	dims := query.NewDimensions()
	engine := NewEngine(reg, dims, src, ctrl, clk, mgr, resolvers, interval)
	saver := func() error { return store.Save(mgr.State()) }
	server := NewServer(engine, saver, os.Getuid())
	d := &Daemon{engine: engine, server: server, store: store, reg: reg, dims: dims}
	// Load config policies (best-effort at startup; a bad config is reported but
	// does not prevent the daemon from serving observation).
	if cfg, ok, err := loadConfig(reg, dims); err == nil && ok {
		engine.SetPolicies(toPolicies(cfg.Managed))
		if cfg.State.History {
			if rec, err := history.Open(filepath.Join(stateDir, "history")); err == nil {
				mgr.SetRecorder(rec)
			}
		}
	}
	return d, nil
}

// Reload re-reads config and replaces the active policy set. A bad config leaves
// the prior policies active and returns the error (RFC §17.4).
func (d *Daemon) Reload() error {
	specs, err := loadPolicies(d.reg, d.dims)
	if err != nil {
		return err
	}
	d.engine.SetPolicies(specs)
	return nil
}

// Run starts the shared collector and serves clients on sockPath until the
// context is cancelled. It persists state on shutdown.
func (d *Daemon) Run(ctx context.Context, sockPath string) error {
	ln, err := Listen(sockPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = d.store.Save(d.engine.mgr.State())
		_ = os.Remove(sockPath)
	}()
	go func() { _ = d.engine.Run(ctx) }()
	go d.watchReload(ctx)
	return d.server.Serve(ctx, ln)
}

// watchReload reloads config policies on SIGHUP (RFC §17.4).
func (d *Daemon) watchReload(ctx context.Context) {
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)
	for {
		select {
		case <-ctx.Done():
			return
		case <-hup:
			_ = d.Reload()
		}
	}
}
