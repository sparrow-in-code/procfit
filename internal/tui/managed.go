package tui

// Panel selects which view the explorer shows. Both panels render the same tree
// (rows/columns/cursor/fold); only the data source, legend and drop action
// differ (PM-0311/0312).
type Panel int

const (
	PanelBrowser Panel = iota
	PanelManaged
)

// DropFunc unmanages the target(s) owning the given pids (stops tracking; it does
// not revert applied control — lift with N/X/Z first).
type DropFunc func(pids []int) (string, error)

// togglePanel switches between the browser and the managed panel. The driver
// re-queries the active panel's data source on the resulting dirty flag.
func (m *Model) togglePanel() {
	if !m.hasMgr {
		m.status = "managed panel unavailable"
		return
	}
	if m.panel == PanelBrowser {
		m.panel = PanelManaged
		m.status = "managed targets"
	} else {
		m.panel = PanelBrowser
		m.status = ""
	}
	m.cursor, m.scroll = 0, 0
	m.dirty = true
}

// dropSelected stops tracking the target(s) owning the selected row's processes.
func (m *Model) dropSelected() {
	if m.dropFn == nil || m.panel != PanelManaged {
		return
	}
	fr, ok := m.currentRow()
	if !ok {
		return
	}
	pids := collectPIDs(fr.row)
	if len(pids) == 0 {
		m.status = "nothing to drop under the selection"
		return
	}
	res, err := m.dropFn(pids)
	if err != nil {
		m.status = "drop: " + err.Error()
	} else {
		m.status = res
	}
	m.dirty = true
}
