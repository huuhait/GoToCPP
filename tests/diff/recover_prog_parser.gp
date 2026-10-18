// phase: 05

// A recursive-descent parser that panics with an error value and recovers it in Parse, re-panicking everything else.
package main

import "fmt"

type Node interface{ Eval() int }
type Lit int
type BinOp struct {
	Op   byte
	L, R Node
}
type Group struct{ Inner Node }

func (l Lit) Eval() int { return int(l) }
func (b BinOp) Eval() int {
	l, r := b.L.Eval(), b.R.Eval()
	switch b.Op {
	case '+':
		return l + r
	case '-':
		return l - r
	case '*':
		return l * r
	default:
		return l / r
	}
}
func (g Group) Eval() int { return g.Inner.Eval() }

type parseError struct {
	pos int
	msg string
}

func (e parseError) Error() string { return fmt.Sprintf("parse error at %d: %s", e.pos, e.msg) }

type parser struct {
	src string
	pos int
}

func (p *parser) peek() byte {
	for p.pos < len(p.src) && p.src[p.pos] == ' ' {
		p.pos++
	}
	if p.pos >= len(p.src) {
		return 0
	}
	return p.src[p.pos]
}

func (p *parser) fail(format string, args ...any) {
	panic(parseError{p.pos, fmt.Sprintf(format, args...)})
}

func (p *parser) expr() Node {
	n := p.term()
	for c := p.peek(); c == '+' || c == '-'; c = p.peek() {
		p.pos++
		n = BinOp{c, n, p.term()}
	}
	return n
}

func (p *parser) term() Node {
	n := p.factor()
	for c := p.peek(); c == '*' || c == '/'; c = p.peek() {
		p.pos++
		n = BinOp{c, n, p.factor()}
	}
	return n
}

func (p *parser) factor() Node {
	switch c := p.peek(); {
	case c >= '0' && c <= '9':
		v := 0
		for p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
			v = v*10 + int(p.src[p.pos]-'0')
			p.pos++
		}
		return Lit(v)
	case c == '(':
		p.pos++
		n := p.expr()
		if p.peek() != ')' {
			p.fail("expected )")
		}
		p.pos++
		return Group{n}
	case c == 0:
		p.fail("unexpected end")
	default:
		p.fail("unexpected %q", c)
	}
	return nil
}

func Parse(s string) (n Node, err error) {
	defer func() {
		switch r := recover().(type) {
		case nil:
		case parseError:
			n, err = nil, r
		default:
			panic(r)
		}
	}()
	p := &parser{src: s}
	n = p.expr()
	if p.peek() != 0 {
		p.fail("trailing input")
	}
	return n, nil
}

func run(s string) (res string) {
	defer func() {
		if r := recover(); r != nil {
			res = fmt.Sprintf("%q: runtime panic: %v", s, r)
		}
	}()
	n, err := Parse(s)
	if err != nil {
		return fmt.Sprintf("%q: %v", s, err)
	}
	return fmt.Sprintf("%q = %d (%T)", s, n.Eval(), n)
}

func main() {
	for _, s := range []string{"1+2*3", "(1+2)*3", " 10 / (5 - 5)", "2 *", "(1", "7 $ 2", "1 2", "((((8))))/2", ""} {
		fmt.Println(run(s))
	}
}
