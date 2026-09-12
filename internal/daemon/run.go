package daemon

import (
	"context"
	"os"
	"time"

	"github.com/netikras/procfit/internal/control"
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
	engine := NewEngine(metrics.NewDefault(), query.NewDimensions(), src, ctrl, clk, mgr, resolvers, interval)
	saver := func() error { return store.Save(mgr.State()) }
	server := NewServer(engine, saver, os.Getuid())
	return &Daemon{engine: engine, server: server, store: store}, nil
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
	return d.server.Serve(ctx, ln)
}
