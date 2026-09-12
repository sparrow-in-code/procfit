//go:build !linux

package procfs

import (
	"fmt"

	"github.com/netikras/procfit/internal/ports"
)

// Controller on non-Linux platforms is a stub: control is unsupported until a
// platform adapter implements it (DEVELOPMENT.md §2.4). Observation still works.
type Controller struct{ s *Source }

// NewController returns a stub controller.
func NewController(s *Source) *Controller { return &Controller{s: s} }

func (c *Controller) Capabilities() ports.ControlCaps { return ports.ControlCaps{} }

func (c *Controller) GetNice(pid int) (int, error) {
	return 0, fmt.Errorf("procfs: control unsupported on this platform")
}

func (c *Controller) SetNice(pid, nice int) error {
	return fmt.Errorf("procfs: control unsupported on this platform")
}

func (c *Controller) SendSignal(pid int, sig ports.Signal) error {
	return fmt.Errorf("procfs: control unsupported on this platform")
}

func (c *Controller) ReadIdentity(pid int) (ports.ProcessInstanceIdentity, bool) {
	st, ok := c.s.readProcess(pid)
	if !ok {
		return ports.ProcessInstanceIdentity{}, false
	}
	return st.ID, true
}
