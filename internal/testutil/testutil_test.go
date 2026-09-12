package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/netikras/procfit/internal/ports"
)

func TestFakeClock(t *testing.T) {
	clk := NewFakeClock(time.Unix(100, 0))
	if clk.Now().Unix() != 100 || clk.NowMono() != 0 {
		t.Fatal("initial clock wrong")
	}
	clk.Advance(2 * time.Second)
	if clk.Now().Unix() != 102 || clk.NowMono() != 2*time.Second {
		t.Fatal("advance wrong")
	}
}

func TestFakeSource(t *testing.T) {
	g1 := []ports.ProcStat{{PID: 1}}
	g2 := []ports.ProcStat{{PID: 2}}
	src := NewFakeSource(g1, g2)
	if b, _ := src.BootID(); b != "boot-test" {
		t.Fatal("boot id")
	}
	if src.ClockTicksPerSec() != 100 || src.OnlineCPUs() != 4 || src.BootTimeUnix() == 0 || src.ID() != "fake" {
		t.Fatal("static info wrong")
	}
	s1, _ := src.List(context.Background())
	s2, _ := src.List(context.Background())
	s3, _ := src.List(context.Background())
	if s1[0].PID != 1 || s2[0].PID != 2 || s3[0].PID != 2 {
		t.Fatalf("generation advance wrong: %d %d %d", s1[0].PID, s2[0].PID, s3[0].PID)
	}
	// Error path.
	src.ListErr = context.Canceled
	if _, err := src.List(context.Background()); err == nil {
		t.Fatal("ListErr should surface")
	}
	// Empty source.
	empty := NewFakeSource()
	if out, err := empty.List(context.Background()); err != nil || out != nil {
		t.Fatal("empty source should return nil,nil")
	}
}

func TestFakeController(t *testing.T) {
	fc := NewFakeController()
	id := ports.ProcessInstanceIdentity{PID: 5, StartTime: 50}
	fc.Add(5, 3, id)
	if n, err := fc.GetNice(5); err != nil || n != 3 {
		t.Fatal("get nice")
	}
	if err := fc.SetNice(5, 9); err != nil || fc.Nice[5] != 9 {
		t.Fatal("set nice")
	}
	if _, err := fc.GetNice(404); err == nil {
		t.Fatal("missing pid get should error")
	}
	if err := fc.SetNice(404, 1); err == nil {
		t.Fatal("missing pid set should error")
	}
	if err := fc.SendSignal(5, ports.SigHUP); err != nil || len(fc.Signals[5]) != 1 {
		t.Fatal("signal")
	}
	if err := fc.SendSignal(404, ports.SigHUP); err == nil {
		t.Fatal("signal missing pid should error")
	}
	if got, ok := fc.ReadIdentity(5); !ok || !got.SameProcess(id) {
		t.Fatal("read identity")
	}
	fc.Remove(5)
	if _, ok := fc.ReadIdentity(5); ok {
		t.Fatal("removed pid should be gone")
	}
	if fc.Capabilities().Nice != true {
		t.Fatal("caps")
	}
}
