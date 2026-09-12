package expr

import (
	"regexp"
	"strings"
)

// Env resolves a field name to a Value for evaluation. Unknown or unavailable
// fields return Missing (RFC §10.6). Entities and aggregated rows each provide
// their own Env implementation.
type Env interface {
	Lookup(field string) Value
}

type node interface {
	eval(Env) Value
}

type litNode struct{ v Value }

func (n litNode) eval(Env) Value { return n.v }

type listNode struct{ elems []node }

func (n listNode) eval(env Env) Value {
	vals := make([]Value, len(n.elems))
	for i, e := range n.elems {
		vals[i] = e.eval(env)
	}
	return Value{Kind: KindList, List: vals}
}

type fieldNode struct{ name string }

func (n fieldNode) eval(env Env) Value { return env.Lookup(n.name) }

type notNode struct{ x node }

func (n notNode) eval(env Env) Value { return Bool(!n.x.eval(env).truthy()) }

type andNode struct{ l, r node }

func (n andNode) eval(env Env) Value {
	if !n.l.eval(env).truthy() { // short-circuit (RFC §10.6)
		return Bool(false)
	}
	return Bool(n.r.eval(env).truthy())
}

type orNode struct{ l, r node }

func (n orNode) eval(env Env) Value {
	if n.l.eval(env).truthy() { // short-circuit
		return Bool(true)
	}
	return Bool(n.r.eval(env).truthy())
}

// boolNode coerces a bare value/field into a boolean context.
type boolNode struct{ x node }

func (n boolNode) eval(env Env) Value { return Bool(n.x.eval(env).truthy()) }

type cmpNode struct {
	op   string
	l, r node
	re   *regexp.Regexp // precompiled for ~= / !~ with a literal pattern
}

func (n cmpNode) eval(env Env) Value {
	lv := n.l.eval(env)
	rv := n.r.eval(env)
	switch n.op {
	case "in", "not in":
		return Bool(evalIn(lv, rv) == (n.op == "in"))
	}
	// For all other operators a missing operand yields false (RFC §10.6),
	// including "!=".
	if lv.IsMissing() || rv.IsMissing() {
		return Bool(false)
	}
	switch n.op {
	case "==":
		return Bool(equalVal(lv, rv))
	case "!=":
		return Bool(!equalVal(lv, rv))
	case "<", "<=", ">", ">=":
		return Bool(compareOrder(n.op, lv, rv))
	case "~=", "!~":
		return Bool(n.matchRegex(lv) == (n.op == "~="))
	case "contains":
		return Bool(evalContains(lv, rv))
	}
	return Bool(false)
}

func (n cmpNode) matchRegex(lv Value) bool {
	if lv.Kind != KindString {
		return false
	}
	re := n.re
	if re == nil {
		return false
	}
	return re.MatchString(lv.Str)
}

func equalVal(a, b Value) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case KindNumber:
		return a.Num == b.Num
	case KindString:
		return a.Str == b.Str
	case KindBool:
		return a.Bool == b.Bool
	default:
		return false
	}
}

func compareOrder(op string, a, b Value) bool {
	if a.Kind == KindNumber && b.Kind == KindNumber {
		return orderNum(op, a.Num, b.Num)
	}
	if a.Kind == KindString && b.Kind == KindString {
		return orderStr(op, a.Str, b.Str)
	}
	return false
}

func orderNum(op string, a, b float64) bool {
	switch op {
	case "<":
		return a < b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case ">=":
		return a >= b
	}
	return false
}

func orderStr(op, a, b string) bool {
	switch op {
	case "<":
		return a < b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case ">=":
		return a >= b
	}
	return false
}

func evalIn(needle, list Value) bool {
	if list.Kind != KindList {
		return false
	}
	for _, e := range list.List {
		if equalVal(needle, e) {
			return true
		}
	}
	return false
}

func evalContains(container, item Value) bool {
	switch container.Kind {
	case KindString:
		return item.Kind == KindString && strings.Contains(container.Str, item.Str)
	case KindList:
		return evalIn(item, container)
	default:
		return false
	}
}

// funcNode implements the built-in functions exists()/missing().
type funcNode struct {
	name  string
	field string
}

func (n funcNode) eval(env Env) Value {
	missing := env.Lookup(n.field).IsMissing()
	switch n.name {
	case "exists":
		return Bool(!missing)
	case "missing":
		return Bool(missing)
	}
	return Bool(false)
}
