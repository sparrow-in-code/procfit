//go:build linux

package procfs

import (
	"fmt"
	"syscall"

	"github.com/netikras/procfit/internal/ports"
)

// Controller is the Linux implementation of ports.Controller. It reuses the
// Source for identity re-reads and uses setpriority/kill for mutations. It never
// runs setuid; permission is whatever the kernel already grants (RFC §20.1).
type Controller struct {
	s *Source
}

// NewController builds a controller backed by the given Source.
func NewController(s *Source) *Controller { return &Controller{s: s} }

// Capabilities reports supported operations (permission is checked per call).
func (c *Controller) Capabilities() ports.ControlCaps {
	return ports.ControlCaps{Nice: true, Signal: true}
}

// GetNice reads the observed nice value from /proc (the actual value, not the
// getpriority-mapped one).
func (c *Controller) GetNice(pid int) (int, error) {
	st, ok := c.s.readProcess(pid)
	if !ok {
		return 0, fmt.Errorf("procfs: pid %d not found", pid)
	}
	return st.Nice, nil
}

// SetNice sets the nice value via setpriority(PRIO_PROCESS).
func (c *Controller) SetNice(pid, nice int) error {
	if err := syscall.Setpriority(syscall.PRIO_PROCESS, pid, nice); err != nil {
		return fmt.Errorf("setpriority pid %d nice %d: %w", pid, nice, err)
	}
	return nil
}

var sigMap = map[ports.Signal]syscall.Signal{
	ports.SigStop: syscall.SIGSTOP,
	ports.SigCont: syscall.SIGCONT,
	ports.SigTerm: syscall.SIGTERM,
	ports.SigKill: syscall.SIGKILL,
	ports.SigHUP:  syscall.SIGHUP,
	ports.SigINT:  syscall.SIGINT,
}

// SendSignal delivers a signal via kill(2).
func (c *Controller) SendSignal(pid int, sig ports.Signal) error {
	s, ok := sigMap[sig]
	if !ok {
		return fmt.Errorf("procfs: unsupported signal %q", sig)
	}
	if err := syscall.Kill(pid, s); err != nil {
		return fmt.Errorf("kill pid %d %s: %w", pid, sig, err)
	}
	return nil
}

// ReadIdentity re-reads canonical identity for revalidation before acting.
func (c *Controller) ReadIdentity(pid int) (ports.ProcessInstanceIdentity, bool) {
	st, ok := c.s.readProcess(pid)
	if !ok {
		return ports.ProcessInstanceIdentity{}, false
	}
	return st.ID, true
}
