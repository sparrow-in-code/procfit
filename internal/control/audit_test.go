package control

import (
	"testing"

	"github.com/netikras/procfit/internal/history"
)

type capRecorder struct{ events []history.Event }

func (c *capRecorder) Record(e history.Event) { c.events = append(c.events, e) }

func TestManager_AuditsApplyAndRestore(t *testing.T) {
	m, fc, _ := newMgr(t)
	rec := &capRecorder{}
	m.SetRecorder(rec)
	fc.Add(7, 0, idFor(7, 70))
	m.Manage("t", ModeFollow, "", []Instance{{ID: idFor(7, 70), PID: 7}})
	_, _ = m.SetNice("t", 10, false)
	_, _ = m.Restore("t", false)

	var applied, restored bool
	for _, e := range rec.events {
		if e.Action == "applied" && e.Field == "nice" && e.To == "10" {
			applied = true
		}
		if e.Action == "restored" && e.Field == "nice" {
			restored = true
		}
	}
	if !applied || !restored {
		t.Fatalf("expected applied+restored audit events, got %+v", rec.events)
	}
}

func TestManager_SetRecorderNilKeepsNop(t *testing.T) {
	m, fc, _ := newMgr(t)
	m.SetRecorder(nil) // must not replace the Nop recorder / must not panic
	fc.Add(7, 0, idFor(7, 70))
	m.Manage("t", ModeFollow, "", []Instance{{ID: idFor(7, 70), PID: 7}})
	if _, err := m.SetNice("t", 3, false); err != nil {
		t.Fatal(err)
	}
}
