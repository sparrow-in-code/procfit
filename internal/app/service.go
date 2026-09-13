package app

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/netikras/procfit/internal/meta"
)

// cmdService implements `procfit service install|uninstall`: it writes/removes a
// user systemd unit but never enables or starts it (RFC §17.2 — persistence is
// opt-in and explicit).
func cmdService(env Env, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "usage: service install|uninstall")
		return ExitUsage
	}
	switch args[0] {
	case "install":
		return serviceInstall(env, args[1:])
	case "uninstall":
		return serviceUninstall(env, args[1:])
	default:
		fmt.Fprintf(env.Stderr, "unknown service subcommand %q\n", args[0])
		return ExitUsage
	}
}

func userUnitPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "systemd", "user", meta.Name+".service"), nil
}

func serviceInstall(env Env, args []string) int {
	fs := flag.NewFlagSet("service install", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	setupUsage(env, fs, "service install", "write a user systemd unit (does NOT enable/start it)",
		"procfit service install   # writes ~/.config/systemd/user/procfit.service + prints enable cmd")
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
	}
	unitPath, err := userUnitPath()
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	exe, err := os.Executable()
	if err != nil {
		exe = meta.Name
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	if err := os.WriteFile(unitPath, []byte(unitContents(exe)), 0o644); err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	fmt.Fprintf(env.Stdout, "wrote %s\n", unitPath)
	fmt.Fprintf(env.Stdout, "to enable it (not done automatically):\n  systemctl --user daemon-reload && systemctl --user enable --now %s\n", meta.Name)
	return ExitOK
}

func serviceUninstall(env Env, args []string) int {
	fs := flag.NewFlagSet("service uninstall", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	setupUsage(env, fs, "service uninstall", "remove the user systemd unit",
		"procfit service uninstall   # removes the unit file (disable it separately if enabled)")
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
	}
	unitPath, err := userUnitPath()
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	fmt.Fprintf(env.Stdout, "removed %s (disable it with: systemctl --user disable %s)\n", unitPath, meta.Name)
	return ExitOK
}

func unitContents(exe string) string {
	return fmt.Sprintf(`[Unit]
Description=%s per-user process observer and workload controller daemon
After=default.target

[Service]
Type=simple
ExecStart=%s daemon run
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure

[Install]
WantedBy=default.target
`, meta.Name, exe)
}
