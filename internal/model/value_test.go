package model

import "testing"

func TestValue_UnknownIsNotZero(t *testing.T) {
	// An unavailable numeric value must not be usable as a real zero.
	v := Unavailable[float64](PermissionDenied, "test")
	if v.Present() {
		t.Fatal("unavailable value reported as present")
	}
	if _, ok := v.Get(); ok {
		t.Fatal("Get() reported ok for an unavailable value")
	}
	if v.Availability != PermissionDenied {
		t.Fatalf("reason lost: got %q", v.Availability)
	}
	// The payload is the zero value but callers are told not to trust it.
	if v.V != 0 {
		t.Fatalf("expected zero payload, got %v", v.V)
	}
}

func TestValue_Available(t *testing.T) {
	v := NewValue(42.0, Exact, "proc")
	if !v.Present() {
		t.Fatal("available value not present")
	}
	got, ok := v.Get()
	if !ok || got != 42.0 {
		t.Fatalf("Get() = (%v,%v), want (42,true)", got, ok)
	}
	if v.Quality != Exact || v.Source != "proc" {
		t.Fatalf("metadata lost: %+v", v)
	}
}

func TestValue_String(t *testing.T) {
	if s := Unavailable[int](Vanished, "x").String(); s != "vanished" {
		t.Fatalf("unavailable String() = %q", s)
	}
	if s := NewValue(7, Exact, "x").String(); s != "7" {
		t.Fatalf("available String() = %q", s)
	}
}

func TestAvailability_Valid(t *testing.T) {
	valid := []Availability{Available, WarmingUp, Disabled, Unsupported, PermissionDenied, Vanished, ReadError}
	for _, a := range valid {
		if !a.Valid() {
			t.Errorf("%q should be valid", a)
		}
	}
	if Availability("bogus").Valid() {
		t.Error("bogus availability reported valid")
	}
}
