package app

import (
	"flag"
	"fmt"
	"strings"

	"github.com/netikras/procfit/internal/control"
	"github.com/netikras/procfit/internal/ports"
)

// defaultMode returns the binding mode for a target spec (RFC §8.6): explicit
// pid/tid default to snapshot; managed/group/selector default to follow.
func defaultMode(spec string) control.BindingMode {
	switch {
	case strings.HasPrefix(spec, "pid:"), strings.HasPrefix(spec, "tid:"):
		return control.ModeSnapshot
	default:
		return control.ModeFollow
	}
}

func targetName(spec string) string {
	if kind, arg, ok := strings.Cut(spec, ":"); ok && kind == "managed" {
		return arg
	}
	return spec
}

func cmdManaged(env Env, args []string) int {
	fs := flag.NewFlagSet("managed", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	stateDir := fs.String("state-dir", "", "runtime state directory")
	setupUsage(env, fs, "managed", "list managed targets and their bindings",
		"procfit managed   # TARGET/MODE/STATE/P(bindings)/NICE; INACTIVE targets are retained")
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
	}
	c, code := openControl(env, *stateDir)
	if code != ExitOK {
		return code
	}
	st := c.mgr.State()
	if len(st.Targets) == 0 {
		fmt.Fprintln(env.Stdout, "no managed targets")
		return ExitOK
	}
	fmt.Fprintf(env.Stdout, "%-20s %-9s %-8s %-6s %s\n", "TARGET", "MODE", "STATE", "P", "NICE")
	for _, t := range st.Targets {
		state := "INACTIVE"
		if t.Active {
			state = "ACTIVE"
		}
		nice := "-"
		if t.DesiredNice != nil {
			nice = fmt.Sprintf("%d", *t.DesiredNice)
		}
		fmt.Fprintf(env.Stdout, "%-20s %-9s %-8s %-6d %s\n", t.Name, t.BindingMode, state, len(t.Bindings), nice)
	}
	return ExitOK
}

func cmdManage(env Env, args []string) int {
	fs := flag.NewFlagSet("manage", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	stateDir := fs.String("state-dir", "", "runtime state directory")
	name := fs.String("name", "", "target name (defaults from spec)")
	nice := fs.Int("nice", 0, "desired nice value")
	setNice := fs.Bool("set-nice", false, "apply --nice")
	stop := fs.Bool("stop", false, "apply SIGSTOP intent")
	snapshot := fs.Bool("snapshot", false, "bind only currently-matching instances")
	follow := fs.Bool("follow", false, "re-resolve continuously")
	setupUsage(env, fs, "manage", "add managed membership, optionally applying control",
		"procfit manage pid:1234 --name chrome            # track a process (membership only)",
		"procfit manage pid:1234 --nice 10 --set-nice     # track + renice to 10",
		"procfit manage 'selector:comm == \"chrome\"' --follow --nice 15 --set-nice   # follow all chrome",
		"Targets: pid:N, tid:N/M, managed:NAME, group:dim=val, selector:EXPR, or a bare PID.")
	spec, rest := extractPositional(args)
	if err := fs.Parse(rest); err != nil {
		return parseExit(err)
	}
	if spec == "" {
		fs.Usage()
		return ExitUsage
	}
	return manageRun(env, *stateDir, spec, manageOpts{
		name: *name, nice: *nice, setNice: *setNice, stop: *stop, snapshot: *snapshot, follow: *follow,
	})
}

type manageOpts struct {
	name                            string
	nice                            int
	setNice, stop, snapshot, follow bool
}

func manageRun(env Env, stateDir, spec string, o manageOpts) int {
	c, code := openControl(env, stateDir)
	if code != ExitOK {
		return code
	}
	instances, err := c.resolveInstances(spec)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitNotFound
	}
	mode := defaultMode(spec)
	if o.snapshot {
		mode = control.ModeSnapshot
	}
	if o.follow {
		mode = control.ModeFollow
	}
	tname := o.name
	if tname == "" {
		tname = targetName(spec)
	}
	c.mgr.Manage(tname, mode, selectorOf(spec), instances)
	fmt.Fprintf(env.Stdout, "managed %q: %d instance(s), mode=%s\n", tname, len(instances), mode)

	exit := c.applySetOps(env, tname, setOps{nice: o.nice, setNice: o.setNice, stop: o.stop})
	if err := c.save(); err != nil {
		fmt.Fprintf(env.Stderr, "save state: %v\n", err)
		return ExitRuntime
	}
	return exit
}

func selectorOf(spec string) string {
	if kind, arg, ok := strings.Cut(spec, ":"); ok && kind == "selector" {
		return arg
	}
	return ""
}

func cmdSet(env Env, args []string) int {
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	stateDir := fs.String("state-dir", "", "runtime state directory")
	nice := fs.Int("nice", 0, "desired nice value")
	setNice := fs.Bool("set-nice", false, "apply --nice")
	stop := fs.Bool("stop", false, "apply SIGSTOP intent")
	cont := fs.Bool("continue", false, "clear procfit SIGSTOP intent")
	freeze := fs.Bool("freeze", false, "freeze the target's cgroup(s)")
	thaw := fs.Bool("thaw", false, "thaw the target's cgroup(s)")
	dryRun := fs.Bool("dry-run", false, "show what would change without acting")
	setupUsage(env, fs, "set", "apply/update control on a managed target",
		"procfit set managed:chrome --nice 15 --set-nice   # renice; prints per-pid result",
		"procfit set managed:chrome --stop                 # SIGSTOP intent (Space in TUI)",
		"procfit set managed:chrome --freeze               # cgroup v2 freeze (if delegated)",
		"procfit set pid:1234 --nice 5 --set-nice --dry-run   # preview without acting")
	spec, rest := extractPositional(args)
	if err := fs.Parse(rest); err != nil {
		return parseExit(err)
	}
	if spec == "" {
		fs.Usage()
		return ExitUsage
	}
	c, code := openControl(env, *stateDir)
	if code != ExitOK {
		return code
	}
	tname, code := c.ensureTarget(env, spec)
	if code != ExitOK {
		return code
	}
	ops := setOps{nice: *nice, setNice: *setNice, stop: *stop, cont: *cont, freeze: *freeze, thaw: *thaw, dryRun: *dryRun}
	exit := c.applySetOps(env, tname, ops)
	if !*dryRun {
		if err := c.save(); err != nil {
			fmt.Fprintf(env.Stderr, "save state: %v\n", err)
			return ExitRuntime
		}
	}
	return exit
}

// setOps captures the mutations requested by `set`.
type setOps struct {
	nice                                      int
	setNice, stop, cont, freeze, thaw, dryRun bool
}

// applySetOps applies the requested control mutations to a target and returns
// the aggregate exit code.
func (c *ctlAsm) applySetOps(env Env, tname string, o setOps) int {
	exit := ExitOK
	if o.setNice {
		res, err := c.mgr.SetNice(tname, o.nice, o.dryRun)
		if err != nil {
			fmt.Fprintf(env.Stderr, "%v\n", err)
			return ExitNotFound
		}
		printResult(env, res)
		exit = maxExit(exit, exitForResult(res))
	}
	if o.stop || o.cont {
		res, _ := c.mgr.SetStop(tname, o.stop)
		printResult(env, res)
		exit = maxExit(exit, exitForResult(res))
	}
	if o.freeze || o.thaw {
		res, _ := c.mgr.SetFreeze(tname, o.freeze)
		printResult(env, res)
		exit = maxExit(exit, exitForResult(res))
	}
	return exit
}

// ensureTarget returns an existing managed target name or creates one from the
// spec, so `set`/`restore` retain managed state (RFC §8.6).
func (c *ctlAsm) ensureTarget(env Env, spec string) (string, int) {
	name := targetName(spec)
	if c.mgr.Find(name) != nil {
		return name, ExitOK
	}
	instances, err := c.resolveInstances(spec)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return "", ExitNotFound
	}
	c.mgr.Manage(name, defaultMode(spec), selectorOf(spec), instances)
	return name, ExitOK
}

func cmdRestore(env Env, args []string) int {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	stateDir := fs.String("state-dir", "", "runtime state directory")
	force := fs.Bool("force", false, "overwrite externally-drifted state")
	setupUsage(env, fs, "restore", "restore captured original control values",
		"procfit restore managed:chrome          # revert nice to originals; exit 7 if it drifted",
		"procfit restore managed:chrome --force  # overwrite external drift back to originals")
	spec, rest := extractPositional(args)
	if err := fs.Parse(rest); err != nil {
		return parseExit(err)
	}
	if spec == "" {
		fs.Usage()
		return ExitUsage
	}
	c, code := openControl(env, *stateDir)
	if code != ExitOK {
		return code
	}
	res, err := c.mgr.Restore(targetName(spec), *force)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitNotFound
	}
	printResult(env, res)
	if err := c.save(); err != nil {
		fmt.Fprintf(env.Stderr, "save state: %v\n", err)
		return ExitRuntime
	}
	return exitForResult(res)
}

func cmdUnmanage(env Env, args []string) int {
	fs := flag.NewFlagSet("unmanage", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	stateDir := fs.String("state-dir", "", "runtime state directory")
	setupUsage(env, fs, "unmanage", "stop tracking a target (kernel state left untouched)",
		"procfit unmanage managed:chrome   # drop membership; does NOT restore (use restore first)")
	spec, rest := extractPositional(args)
	if err := fs.Parse(rest); err != nil {
		return parseExit(err)
	}
	if spec == "" {
		fs.Usage()
		return ExitUsage
	}
	c, code := openControl(env, *stateDir)
	if code != ExitOK {
		return code
	}
	if err := c.mgr.Unmanage(targetName(spec)); err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitNotFound
	}
	if err := c.save(); err != nil {
		fmt.Fprintf(env.Stderr, "save state: %v\n", err)
		return ExitRuntime
	}
	fmt.Fprintf(env.Stdout, "unmanaged %q\n", targetName(spec))
	return ExitOK
}

func cmdSignal(env Env, args []string) int {
	fs := flag.NewFlagSet("signal", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	stateDir := fs.String("state-dir", "", "runtime state directory")
	yes := fs.Bool("yes", false, "skip confirmation for destructive signals")
	setupUsage(env, fs, "signal", "send a signal to a resolved target",
		"procfit signal HUP pid:1234        # SIGHUP a process (identity revalidated first)",
		"procfit signal TERM managed:chrome --yes   # TERM all bound pids (--yes: non-interactive)")
	// Two positionals: <sig> <target>.
	var positionals []string
	var flagArgs []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			flagArgs = append(flagArgs, a)
		} else {
			positionals = append(positionals, a)
		}
	}
	if err := fs.Parse(flagArgs); err != nil {
		return parseExit(err)
	}
	if len(positionals) < 2 {
		fmt.Fprintln(env.Stderr, "usage: signal <SIG> <target> [--yes]")
		return ExitUsage
	}
	sig := ports.Signal(strings.ToUpper(strings.TrimPrefix(positionals[0], "SIG")))
	spec := positionals[1]
	if destructive(sig) && !*yes && !env.IsTTY {
		fmt.Fprintf(env.Stderr, "%s is destructive; pass --yes to confirm non-interactively\n", sig)
		return ExitUsage
	}
	c, code := openControl(env, *stateDir)
	if code != ExitOK {
		return code
	}
	instances, err := c.resolveInstances(spec)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitNotFound
	}
	res := c.mgr.Signal(instances, sig)
	printResult(env, res)
	return exitForResult(res)
}

func destructive(sig ports.Signal) bool {
	return sig == ports.SigTerm || sig == ports.SigKill
}

func openControl(env Env, stateDirFlag string) (*ctlAsm, int) {
	dir, ephemeral := resolveStateDir(stateDirFlag)
	if ephemeral {
		fmt.Fprintf(env.Stderr, "warning: no XDG_RUNTIME_DIR; using %s (restore metadata not durable across reboots)\n", dir)
	}
	c, err := newControlAsmFn(dir)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return nil, ExitRuntime
	}
	return c, ExitOK
}

func maxExit(a, b int) int {
	if b > a {
		return b
	}
	return a
}
