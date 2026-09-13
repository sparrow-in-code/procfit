package app

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/netikras/procfit/internal/daemon"
	"github.com/netikras/procfit/internal/meta"
)

// cmdDaemon implements `procfit daemon run|status` (RFC §17).
func cmdDaemon(env Env, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "usage: daemon run|status [flags]")
		return ExitUsage
	}
	switch args[0] {
	case "run":
		return daemonRun(env, args[1:])
	case "status":
		return daemonStatus(env, args[1:])
	default:
		fmt.Fprintf(env.Stderr, "unknown daemon subcommand %q\n", args[0])
		return ExitUsage
	}
}

func daemonSockPath(dir, override string) string {
	if override != "" {
		return override
	}
	return filepath.Join(dir, meta.Name+".sock")
}

func daemonRun(env Env, args []string) int {
	fs := flag.NewFlagSet("daemon run", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	stateDir := fs.String("state-dir", "", "runtime state directory")
	sock := fs.String("socket", "", "unix socket path")
	interval := fs.Duration("interval", time.Second, "shared sample interval")
	setupUsage(env, fs, "daemon run", "run the per-user engine daemon (shared collection + policy enforcement)",
		"procfit daemon run                     # foreground; Ctrl-C to stop, SIGHUP to reload config",
		"procfit daemon run --interval 2s       # slower shared sampling")
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
	}
	dir, ephemeral := resolveStateDir(*stateDir)
	if ephemeral {
		fmt.Fprintf(env.Stderr, "warning: no XDG_RUNTIME_DIR; using %s\n", dir)
	}
	sp := daemonSockPath(dir, *sock)

	d, err := daemon.New(dir, *interval)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Fprintf(env.Stdout, "%s daemon listening on %s (interval %s)\n", meta.Name, sp, *interval)
	if err := d.Run(ctx, sp); err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	return ExitOK
}

func daemonStatus(env Env, args []string) int {
	fs := flag.NewFlagSet("daemon status", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	stateDir := fs.String("state-dir", "", "runtime state directory")
	sock := fs.String("socket", "", "unix socket path")
	setupUsage(env, fs, "daemon status", "query a running daemon's health",
		"procfit daemon status   # prints version, caps, uptime, generation, clients, processes")
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
	}
	dir, _ := resolveStateDir(*stateDir)
	cl, err := daemon.Dial(daemonSockPath(dir, *sock))
	if err != nil {
		fmt.Fprintf(env.Stderr, "daemon not running (%v)\n", err)
		return ExitDaemon
	}
	defer cl.Close()

	hello, err := cl.Hello()
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitDaemon
	}
	resp, err := cl.Do(daemon.Request{Type: daemon.ReqHealth})
	if err != nil || resp.Health == nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitDaemon
	}
	h := resp.Health
	fmt.Fprintf(env.Stdout, "daemon: %s v%d (nice=%v signal=%v)\n", hello.Name, hello.Version, hello.NiceCtl, hello.SignalCtl)
	fmt.Fprintf(env.Stdout, "uptime=%.0fs generation=%d clients=%d processes=%d last-scan=%.2fms\n",
		h.UptimeSeconds, h.Generation, h.Clients, h.Processes, h.LastScanMillis)
	return ExitOK
}
