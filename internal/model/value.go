// Package model defines the OS-agnostic core domain: normalized entities,
// stable identities, and metric/control values. It must not import any
// platform-specific package (no syscall, no x/sys, no /proc assumptions);
// platform specifics live behind ports (see internal/ports). See DEVELOPMENT.md
// §2.4 and RFC §11.
package model

import "fmt"

// Availability records whether a value is present and, if not, why. It exists to
// enforce the invariant "unknown is not zero" (RFC §6.4): a missing metric must
// never be silently rendered as 0.
type Availability string

const (
	// Available means the value was read successfully.
	Available Availability = "available"
	// WarmingUp means a rate/delta has no prior sample yet (RFC §13.6).
	WarmingUp Availability = "warming_up"
	// Disabled means the producing collector is not enabled.
	Disabled Availability = "disabled"
	// Unsupported means the kernel/platform/build cannot provide it.
	Unsupported Availability = "unsupported"
	// PermissionDenied means access was refused (hidepid, caps, paranoid, ...).
	PermissionDenied Availability = "permission_denied"
	// Vanished means the entity disappeared mid-read (ENOENT/ESRCH).
	Vanished Availability = "vanished"
	// ReadError means an unexpected read/parse failure occurred.
	ReadError Availability = "read_error"
)

// Valid reports whether a is a known availability constant.
func (a Availability) Valid() bool {
	switch a {
	case Available, WarmingUp, Disabled, Unsupported, PermissionDenied, Vanished, ReadError:
		return true
	default:
		return false
	}
}

// Quality describes how exact an available value is (RFC §11).
type Quality string

const (
	// Exact is a precise reading.
	Exact Quality = "exact"
	// Sampled is subject to between-scan blind spots (e.g. churn).
	Sampled Quality = "sampled"
	// Estimated is an approximation.
	Estimated Quality = "estimated"
	// Derived is computed from other values.
	Derived Quality = "derived"
)

// Value is a metric/attribute reading carrying availability and quality
// metadata alongside the payload. The zero Value is deliberately NOT
// "available zero": callers must set Availability explicitly, and readers must
// check Present before trusting T.
type Value[T any] struct {
	V            T
	Availability Availability
	Quality      Quality
	// Source names the collector/backend that produced the value.
	Source string
}

// Present reports whether the value is available for use.
func (v Value[T]) Present() bool { return v.Availability == Available }

// Get returns the payload and whether it is present. Callers that care about
// availability must use this rather than reading V directly.
func (v Value[T]) Get() (T, bool) { return v.V, v.Present() }

// NewValue constructs an available value.
func NewValue[T any](v T, q Quality, source string) Value[T] {
	return Value[T]{V: v, Availability: Available, Quality: q, Source: source}
}

// Unavailable constructs a value that is not present, with a reason. The payload
// is the type's zero and must not be interpreted as a real reading.
func Unavailable[T any](reason Availability, source string) Value[T] {
	var zero T
	return Value[T]{V: zero, Availability: reason, Quality: "", Source: source}
}

// String renders the value for debugging (not a stable output format).
func (v Value[T]) String() string {
	if !v.Present() {
		return string(v.Availability)
	}
	return fmt.Sprintf("%v", v.V)
}
