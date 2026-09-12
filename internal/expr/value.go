// Package expr implements the selector/filter DSL used by `select` (per-entity,
// pre-group) and `having` (per-row, post-aggregation) and by persistent managed
// selectors (RFC §10). It is independent of the model and query packages: it
// evaluates against an Env abstraction, so the same compiled program can be
// applied to entities or aggregated rows (Liskov / DIP, DEVELOPMENT.md §2.2).
package expr

import "fmt"

// Kind is the dynamic type of a Value.
type Kind int

const (
	KindMissing Kind = iota // value not available (RFC §10.6)
	KindNumber
	KindString
	KindBool
	KindList
)

// Value is a dynamically-typed evaluation value.
type Value struct {
	Kind Kind
	Num  float64
	Str  string
	Bool bool
	List []Value
}

// Missing is the canonical unavailable value.
var Missing = Value{Kind: KindMissing}

// Num builds a number value.
func Num(f float64) Value { return Value{Kind: KindNumber, Num: f} }

// Str builds a string value.
func Str(s string) Value { return Value{Kind: KindString, Str: s} }

// Bool builds a boolean value.
func Bool(b bool) Value { return Value{Kind: KindBool, Bool: b} }

// IsMissing reports whether the value is unavailable.
func (v Value) IsMissing() bool { return v.Kind == KindMissing }

// truthy reports the boolean interpretation for logical operators. Missing and
// non-bool values are false.
func (v Value) truthy() bool { return v.Kind == KindBool && v.Bool }

func (v Value) String() string {
	switch v.Kind {
	case KindNumber:
		return fmt.Sprintf("%g", v.Num)
	case KindString:
		return v.Str
	case KindBool:
		return fmt.Sprintf("%t", v.Bool)
	case KindList:
		return fmt.Sprintf("%v", v.List)
	default:
		return "<missing>"
	}
}
