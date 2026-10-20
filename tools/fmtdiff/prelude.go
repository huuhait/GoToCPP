package main

// prelude is the fixed part of every generated program: the types, the methods and the helper
// functions. The values (vN) and main are appended by the generator.
const prelude = `package main

import (
	"errors"
	"fmt"
	"io"
	"math"
)

type MyInt int
type MyU8 uint8
type MyStr string
type MyF float64
type MyBool bool
type MyBytes []byte
type MyArr [3]int
type MyMap map[string]int
type MySlice []string
type Pt struct{ X, Y int }
type Inner struct {
	A int
	B string
}
type Outer struct {
	I   Inner
	P   *Inner
	S   []int
	M   map[string]int
	E   error
	Any any
}
type Emb struct {
	Inner
	N int
}
type Unexp struct {
	a int
	b string
	C float64
}
type Tagged struct {
	A int    ` + "`json:\"a\"`" + `
	B string ` + "`x:\"y\"`" + `
}
type Empty struct{}
type Pair[K comparable, V any] struct {
	Key K
	Val V
}
type Box[T any] struct{ V T }
type StrV struct{ n int }

func (s StrV) String() string { return fmt.Sprint("S", s.n) }

type StrP struct{ n int }

func (s *StrP) String() string { return fmt.Sprint("P", s.n) }

type ErrV struct{ code int }

func (e ErrV) Error() string { return fmt.Sprint("err", e.code) }

type GoS struct{ n int }

func (g GoS) GoString() string { return "GoS!" }

type Fmtr struct{ n int }

func (f Fmtr) Format(s fmt.State, verb rune) {
	w, wok := s.Width()
	p, pok := s.Precision()
	fmt.Fprintf(s, "F[%c w=%d,%v p=%d,%v", verb, w, wok, p, pok)
	for _, c := range "+-# 0" {
		if s.Flag(int(c)) {
			s.Write([]byte{byte(c)})
		}
	}
	fmt.Fprint(s, "]")
}

type StrErr struct{ s string }

func (e *StrErr) Error() string { return e.s }

type PanicS struct{}

func (PanicS) String() string { panic("boom") }

type NilS struct{ p *int }

func (n *NilS) String() string { return fmt.Sprint(*n.p) }

type Color int

func (c Color) String() string {
	switch c {
	case 0:
		return "red"
	case 1:
		return "green"
	}
	return fmt.Sprintf("Color(%d)", int(c))
}

type HasStrField struct {
	S StrV
	s StrV
	P *StrP
}

type EmbS struct {
	StrV
	X int
}
type EmbP struct{ *StrP }
type EmbE struct {
	ErrV
	Y int
}
type Sn string

func (s Sn) String() string { return "<" + string(s) + ">" }

type PanicE struct{}

func (PanicE) String() string { panic(ErrV{9}) }

type PanicI struct{}

func (PanicI) Error() string { panic(42) }

type PanicG struct{}

func (PanicG) GoString() string { panic("gs") }

type PanicF struct{}

func (PanicF) Format(f fmt.State, v rune) { panic("fmt") }

type FmtStr struct{}

func (FmtStr) Format(f fmt.State, v rune) { f.Write([]byte(fmt.FormatString(f, v))) }

type Both struct{}

func (Both) Error() string  { return "E" }
func (Both) String() string { return "S" }

type GoAndStr struct{}

func (GoAndStr) GoString() string { return "G" }
func (GoAndStr) String() string   { return "S" }

type NilFmt struct{ p *int }

func (n *NilFmt) Format(f fmt.State, v rune) { fmt.Fprint(f, *n.p) }

type MyPtr uintptr
type NamedC complex128
type Tree struct {
	L, R *Tree
	V    int
}

var _ = errors.New
var _ io.Writer = (*lim)(nil) // gocpp needs package io in the program to pass a *lim to Fprint (a gap, see README)
var _ = math.Pi

type lim struct {
	left int
	buf  []byte
}

func (l *lim) Write(p []byte) (int, error) {
	if len(p) > l.left {
		n := l.left
		l.left = 0
		l.buf = append(l.buf, p[:n]...)
		return n, errors.New("short")
	}
	l.left -= len(p)
	l.buf = append(l.buf, p...)
	return len(p), nil
}

func fp(l *lim, n int, err error) string { return fmt.Sprintf("%d %v %q", n, err, l.buf) }

func id(x any) any { return x }

func c(id int, s string) { fmt.Printf("%d %q\n", id, s) }

func ef(e error) string {
	s := e.Error()
	if u, ok := e.(interface{ Unwrap() []error }); ok {
		s += fmt.Sprint(" multi=", len(u.Unwrap()))
	} else if u, ok := e.(interface{ Unwrap() error }); ok {
		s += " single=" + fmt.Sprint(u.Unwrap())
	}
	return s + fmt.Sprintf(" %T", e)
}
`
