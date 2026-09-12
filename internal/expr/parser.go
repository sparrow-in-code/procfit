package expr

import (
	"fmt"
	"regexp"
)

type parser struct {
	toks []token
	i    int
}

func (p *parser) peek() token { return p.toks[p.i] }
func (p *parser) advance() token {
	t := p.toks[p.i]
	if p.i < len(p.toks)-1 {
		p.i++
	}
	return t
}

func (p *parser) parseExpr() (node, error) { return p.parseOr() }

func (p *parser) parseOr() (node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tOp && p.peek().val == "||" {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = orNode{l: left, r: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tOp && p.peek().val == "&&" {
		p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = andNode{l: left, r: right}
	}
	return left, nil
}

func (p *parser) parseUnary() (node, error) {
	if p.peek().kind == tOp && p.peek().val == "!" {
		p.advance()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return notNode{x: x}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (node, error) {
	if p.peek().kind == tLParen {
		p.advance()
		inner, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.peek().kind != tRParen {
			return nil, fmt.Errorf("expr: expected ')'")
		}
		p.advance()
		return inner, nil
	}
	// Function call?
	if p.peek().kind == tIdent && p.toks[p.i+1].kind == tLParen {
		return p.parseFunc()
	}
	left, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	return p.maybeComparison(left)
}

func (p *parser) maybeComparison(left node) (node, error) {
	op, ok := p.comparisonOp()
	if !ok {
		return boolNode{x: left}, nil
	}
	right, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	cmp := cmpNode{op: op, l: left, r: right}
	if op == "~=" || op == "!~" {
		lit, ok := right.(litNode)
		if !ok || lit.v.Kind != KindString {
			return nil, fmt.Errorf("expr: %s requires a string regex on the right", op)
		}
		re, err := regexp.Compile(lit.v.Str)
		if err != nil {
			return nil, fmt.Errorf("expr: invalid regex: %w", err)
		}
		cmp.re = re
	}
	return cmp, nil
}

// comparisonOp consumes a comparison operator if present, including the
// identifier-form operators in / not in / contains.
func (p *parser) comparisonOp() (string, bool) {
	t := p.peek()
	if t.kind == tOp {
		switch t.val {
		case "==", "!=", "<", "<=", ">", ">=", "~=", "!~":
			p.advance()
			return t.val, true
		}
		return "", false
	}
	if t.kind == tIdent {
		switch t.val {
		case "in", "contains":
			p.advance()
			return t.val, true
		case "not":
			if p.toks[p.i+1].kind == tIdent && p.toks[p.i+1].val == "in" {
				p.advance()
				p.advance()
				return "not in", true
			}
		}
	}
	return "", false
}

func (p *parser) parseValue() (node, error) {
	t := p.peek()
	switch t.kind {
	case tString:
		p.advance()
		return litNode{v: Str(t.str)}, nil
	case tNumber:
		p.advance()
		return litNode{v: Num(t.num)}, nil
	case tLBrack:
		return p.parseList()
	case tIdent:
		p.advance()
		switch t.val {
		case "true":
			return litNode{v: Bool(true)}, nil
		case "false":
			return litNode{v: Bool(false)}, nil
		}
		return fieldNode{name: t.val}, nil
	default:
		return nil, fmt.Errorf("expr: unexpected token %q", tokenText(t))
	}
}

func (p *parser) parseList() (node, error) {
	p.advance() // [
	var elems []node
	for p.peek().kind != tRBrack {
		v, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		elems = append(elems, v)
		if p.peek().kind == tComma {
			p.advance()
			continue
		}
		break
	}
	if p.peek().kind != tRBrack {
		return nil, fmt.Errorf("expr: expected ']' to close list")
	}
	p.advance()
	return listNode{elems: elems}, nil
}

func (p *parser) parseFunc() (node, error) {
	name := p.advance().val
	p.advance() // (
	if name != "exists" && name != "missing" {
		return nil, fmt.Errorf("expr: unknown or unsupported function %q", name)
	}
	arg := p.peek()
	if arg.kind != tIdent {
		return nil, fmt.Errorf("expr: %s() requires a field argument", name)
	}
	p.advance()
	if p.peek().kind != tRParen {
		return nil, fmt.Errorf("expr: %s() takes one field argument", name)
	}
	p.advance()
	return funcNode{name: name, field: arg.val}, nil
}

func tokenText(t token) string {
	switch t.kind {
	case tString:
		return t.str
	case tEOF:
		return "<eof>"
	default:
		return t.val
	}
}
