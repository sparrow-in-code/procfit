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
	if err := fs.Parse(args); err != nil {
		return ExitUsage
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
	spec, rest := extractPositional(args)
	if err := fs.Parse(rest); err != nil {
		return ExitUsage
	}
	if spec == "" {
		fmt.Fprintln(env.Stderr, "usage: manage <target> [--nice N --set-nice] [--stop] [--snapshot|--follow]")
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
	mode := defaultMode(spec)
	if *snapshot {
		mode = control.ModeSnapshot
	}
	if *follow {
		mode = control.ModeFollow
	}
	tname := *name
	if tname == "" {
		tname = targetName(spec)
	}
	c.mgr.Manage(tname, mode, selectorOf(spec), instances)
	fmt.Fprintf(env.Stdout, "managed %q: %d instance(s), mode=%s\n", tname, len(instances), mode)

	exit := ExitOK
	if *setNice {
		res, _ := c.mgr.SetNice(tname, *nice, false)
		printResult(env, res)
		exit = maxExit(exit, exitForResult(res))
	}
	if *stop {
		res, _ := c.mgr.SetStop(tname, true)
		printResult(env, res)
		exit = maxExit(exit, exitForResult(res))
	}
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
	dryRun := fs.Bool("dry-run", false, "show what would change without acting")
	spec, rest := extractPositional(args)
	if err := fs.Parse(rest); err != nil {
		return ExitUsage
	}
	if spec == "" {
		fmt.Fprintln(env.Stderr, "usage: set <target> [--nice N --set-nice] [--stop|--continue]")
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
	exit := ExitOK
	if *setNice {
		res, err := c.mgr.SetNice(tname, *nice, *dryRun)
		if err != nil {
			fmt.Fprintf(env.Stderr, "%v\n", err)
			return ExitNotFound
		}
		printResult(env, res)
		exit = maxExit(exit, exitForResult(res))
	}
	if *stop || *cont {
		res, _ := c.mgr.SetStop(tname, *stop)
		printResult(env, res)
		exit = maxExit(exit, exitForResult(res))
	}
	if !*dryRun {
		if err := c.save(); err != nil {
			fmt.Fprintf(env.Stderr, "save state: %v\n", err)
			return ExitRuntime
		}
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
	spec, rest := extractPositional(args)
	if err := fs.Parse(rest); err != nil {
		return ExitUsage
	}
	if spec == "" {
		fmt.Fprintln(env.Stderr, "usage: restore <target> [--force]")
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
	spec, rest := extractPositional(args)
	if err := fs.Parse(rest); err != nil {
		return ExitUsage
	}
	if spec == "" {
		fmt.Fprintln(env.Stderr, "usage: unmanage <target>")
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
		return ExitUsage
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
