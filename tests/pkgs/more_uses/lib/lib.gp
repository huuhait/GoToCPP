package lib

import "fmt"

type Grid [3][3]int

func (g *Grid) Set(i, j, v int) { g[i][j] = v }
func (g Grid) Trace() (t int) {
	for i := range g {
		t += g[i][i]
	}
	return
}

type Padded struct {
	A int
	_ int
	B string
	_ [2]byte
}

type Stack []string

func (s *Stack) Push(x string) { *s = append(*s, x) }
func (s *Stack) Pop() (x string, ok bool) {
	if len(*s) == 0 {
		return "", false
	}
	x = (*s)[len(*s)-1]
	*s = (*s)[:len(*s)-1]
	return x, true
}

func Sum(xs ...int) (total int) {
	defer func() { total *= 1 }()
	for _, x := range xs {
		total += x
	}
	return
}

func Join(sep string, parts ...string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

func Counter() (inc func() int, get func() int) {
	n := 0
	return func() int { n++; return n }, func() int { return n }
}

type Button struct {
	Label   string
	OnClick func(string) string
	clicks  int
}

func (b *Button) Click() string {
	b.clicks++
	if b.OnClick == nil {
		return "nothing"
	}
	return b.OnClick(fmt.Sprint(b.Label, b.clicks))
}

func New(label string, f func(string) string) *Button { return &Button{Label: label, OnClick: f} }

func Delete(m map[string]int, k string) { delete(m, k) }

type Err struct{ Code int }

func (e *Err) Error() string { return fmt.Sprint("err ", e.Code) }

func Fail(n int) error {
	if n == 0 {
		return nil
	}
	return &Err{n}
}
