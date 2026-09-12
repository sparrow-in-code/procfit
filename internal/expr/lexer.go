package expr

import (
	"fmt"
	"strconv"
	"strings"
)

type tokKind int

const (
	tEOF tokKind = iota
	tIdent
	tString
	tNumber
	tOp
	tLParen
	tRParen
	tLBrack
	tRBrack
	tComma
)

type token struct {
	kind tokKind
	val  string  // operator text or identifier/keyword
	str  string  // string literal content
	num  float64 // numeric/size/duration value
	pos  int
}

type lexer struct {
	src string
	pos int
	tok []token
}

func lex(src string) ([]token, error) {
	l := &lexer{src: src}
	for {
		t, err := l.next()
		if err != nil {
			return nil, err
		}
		l.tok = append(l.tok, t)
		if t.kind == tEOF {
			return l.tok, nil
		}
	}
}

func (l *lexer) next() (token, error) {
	l.skipSpace()
	if l.pos >= len(l.src) {
		return token{kind: tEOF, pos: l.pos}, nil
	}
	c := l.src[l.pos]
	switch {
	case c == '"':
		return l.lexString()
	case c == '(':
		return l.single(tLParen, "(")
	case c == ')':
		return l.single(tRParen, ")")
	case c == '[':
		return l.single(tLBrack, "[")
	case c == ']':
		return l.single(tRBrack, "]")
	case c == ',':
		return l.single(tComma, ",")
	case isDigit(c):
		return l.lexNumber()
	case isIdentStart(c):
		return l.lexIdent()
	default:
		return l.lexOp()
	}
}

func (l *lexer) single(k tokKind, v string) (token, error) {
	t := token{kind: k, val: v, pos: l.pos}
	l.pos++
	return t, nil
}

func (l *lexer) skipSpace() {
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t' || l.src[l.pos] == '\n') {
		l.pos++
	}
}

func (l *lexer) lexString() (token, error) {
	start := l.pos
	l.pos++ // opening quote
	var b strings.Builder
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == '\\' && l.pos+1 < len(l.src) {
			b.WriteByte(unescape(l.src[l.pos+1]))
			l.pos += 2
			continue
		}
		if c == '"' {
			l.pos++
			return token{kind: tString, str: b.String(), pos: start}, nil
		}
		b.WriteByte(c)
		l.pos++
	}
	return token{}, fmt.Errorf("expr: unterminated string at %d", start)
}

func unescape(c byte) byte {
	switch c {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	case 'r':
		return '\r'
	default:
		return c
	}
}

func (l *lexer) lexIdent() (token, error) {
	start := l.pos
	for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
		l.pos++
	}
	return token{kind: tIdent, val: l.src[start:l.pos], pos: start}, nil
}

func (l *lexer) lexNumber() (token, error) {
	start := l.pos
	for l.pos < len(l.src) && (isDigit(l.src[l.pos]) || l.src[l.pos] == '.') {
		l.pos++
	}
	mantissa := l.src[start:l.pos]
	sufStart := l.pos
	for l.pos < len(l.src) && isLetter(l.src[l.pos]) {
		l.pos++
	}
	suffix := l.src[sufStart:l.pos]
	base, err := strconv.ParseFloat(mantissa, 64)
	if err != nil {
		return token{}, fmt.Errorf("expr: bad number %q", mantissa)
	}
	val, err := applySuffix(base, suffix)
	if err != nil {
		return token{}, err
	}
	return token{kind: tNumber, num: val, pos: start}, nil
}

func (l *lexer) lexOp() (token, error) {
	two := ""
	if l.pos+1 < len(l.src) {
		two = l.src[l.pos : l.pos+2]
	}
	switch two {
	case "==", "!=", "<=", ">=", "~=", "!~", "&&", "||":
		t := token{kind: tOp, val: two, pos: l.pos}
		l.pos += 2
		return t, nil
	}
	one := l.src[l.pos : l.pos+1]
	switch one {
	case "<", ">", "!":
		t := token{kind: tOp, val: one, pos: l.pos}
		l.pos++
		return t, nil
	}
	return token{}, fmt.Errorf("expr: unexpected character %q at %d", one, l.pos)
}

func isDigit(c byte) bool      { return c >= '0' && c <= '9' }
func isLetter(c byte) bool     { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isIdentStart(c byte) bool { return isLetter(c) || c == '_' }
func isIdentPart(c byte) bool  { return isIdentStart(c) || isDigit(c) || c == '-' || c == '.' }
