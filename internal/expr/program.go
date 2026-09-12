package expr

import (
	"fmt"
	"sort"
)

// Program is a compiled, reusable expression. It is safe for concurrent
// evaluation (it holds no mutable state).
type Program struct {
	root   node
	fields []string
}

// Compile parses and type-checks an expression. The expression must consume all
// input. Field names are collected for optional validation via Validate.
func Compile(input string) (*Program, error) {
	toks, err := lex(input)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	root, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tEOF {
		return nil, fmt.Errorf("expr: unexpected trailing input near %q", tokenText(p.peek()))
	}
	set := map[string]bool{}
	collectFields(root, set)
	fields := make([]string, 0, len(set))
	for f := range set {
		fields = append(fields, f)
	}
	sort.Strings(fields)
	return &Program{root: root, fields: fields}, nil
}

// Fields returns the sorted field names referenced by the program.
func (p *Program) Fields() []string { return p.fields }

// Validate checks that every referenced field is allowed for this stage,
// enforcing the entity-vs-aggregate field discipline (RFC §10.5).
func (p *Program) Validate(allowed map[string]bool) error {
	for _, f := range p.fields {
		if !allowed[f] {
			return fmt.Errorf("expr: field %q is not valid in this context", f)
		}
	}
	return nil
}

// Eval evaluates the program against an environment, returning its truth value.
func (p *Program) Eval(env Env) bool { return p.root.eval(env).truthy() }

func collectFields(n node, set map[string]bool) {
	switch t := n.(type) {
	case fieldNode:
		set[t.name] = true
	case funcNode:
		set[t.field] = true
	case boolNode:
		collectFields(t.x, set)
	case notNode:
		collectFields(t.x, set)
	case andNode:
		collectFields(t.l, set)
		collectFields(t.r, set)
	case orNode:
		collectFields(t.l, set)
		collectFields(t.r, set)
	case cmpNode:
		collectFields(t.l, set)
		collectFields(t.r, set)
	case listNode:
		for _, e := range t.elems {
			collectFields(e, set)
		}
	}
}
