package sysclock

import (
	"testing"
	"time"
)

func TestClock(t *testing.T) {
	c := New()
	if c.Now().IsZero() {
		t.Fatal("Now should return a real time")
	}
	m1 := c.NowMono()
	time.Sleep(2 * time.Millisecond)
	m2 := c.NowMono()
	if m2 <= m1 {
		t.Fatalf("NowMono should advance: %v -> %v", m1, m2)
	}
	if m1 < 0 {
		t.Fatal("mono reading should be non-negative")
	}
}
