package model

import "testing"

func TestProcess_MetricUnknownIsNotZero(t *testing.T) {
	p := &Process{}
	v := p.Metric("cpu")
	if v.Present() {
		t.Fatal("missing metric must not be present")
	}
	if v.Availability != Disabled {
		t.Fatalf("missing metric reason = %q, want disabled", v.Availability)
	}
}

func TestProcess_SetGetMetric(t *testing.T) {
	p := &Process{}
	p.SetMetric("cpu", NewValue(12.5, Derived, "sampler"))
	v := p.Metric("cpu")
	got, ok := v.Get()
	if !ok || got != 12.5 {
		t.Fatalf("Metric(cpu) = (%v,%v), want (12.5,true)", got, ok)
	}
}

func TestProcessState(t *testing.T) {
	if (ProcessState{Code: StateRunning}).Description() != "running" {
		t.Error("R should be running")
	}
	if !(ProcessState{Code: StateStopped}).Stopped() {
		t.Error("T should be stopped")
	}
	if !(ProcessState{Code: StateTracingStop}).Stopped() {
		t.Error("t should be stopped")
	}
	if (ProcessState{Code: StateRunning}).Stopped() {
		t.Error("R should not be stopped")
	}
	if (ProcessState{}).Description() != "unknown" {
		t.Error("zero state should be unknown")
	}
}

func TestNamespaceSet_Key(t *testing.T) {
	a := NamespaceSet{NSPID: 100, NSNet: 200}
	b := NamespaceSet{NSNet: 200, NSPID: 100}
	if a.Key() != b.Key() {
		t.Fatalf("key must be order-independent: %q vs %q", a.Key(), b.Key())
	}
	if a.Key() == "" {
		t.Fatal("non-empty set should have non-empty key")
	}
	if (NamespaceSet{}).Key() != "" {
		t.Fatal("empty set key should be empty")
	}
	if _, ok := a.Get(NSPID); !ok {
		t.Fatal("Get should find present ns")
	}
	if _, ok := a.Get(NSUser); ok {
		t.Fatal("Get should miss absent ns")
	}
	var nilSet NamespaceSet
	if _, ok := nilSet.Get(NSPID); ok {
		t.Fatal("nil set Get should be false")
	}
}

func TestNamespaceID_String(t *testing.T) {
	if (NamespaceID{Type: NSPID, Inode: 42}).String() != "pid:42" {
		t.Fatal("unexpected NamespaceID string")
	}
}
