// phase: 07

// Pointer types as type arguments: *T instances compare by identity, method sets differ from T's.
package main

import "fmt"

type Counter struct{ n int }

func (c *Counter) Inc() int  { c.n++; return c.n }
func (c Counter) Value() int { return c.n }

type Incer interface{ Inc() int }
type Valuer interface{ Value() int }

func Bump[T Incer](x T) int { return x.Inc() + x.Inc() }

func Read[T Valuer](x T) int { return x.Value() }

func Same[T comparable](a, b T) bool { return a == b }

func Deref[T any](pp **T) T { return **pp }

func Set[T any](pp **T, v *T) { *pp = v }

func Nil[T any]() *T { return nil }

func IsNil[T any](p *T) bool { return p == nil }

func Chain[T any](v T) ***T {
	p := &v
	pp := &p
	return &pp
}

func main() {
	c := &Counter{}
	fmt.Println(Bump(c), Read(c), Read(*c), c.n)
	fmt.Println(Same(c, c), Same(c, &Counter{}), Same(*c, Counter{4}), Same[*Counter](nil, nil))
	a, b := 1, 2
	pa, pb := &a, &b
	fmt.Println(Deref(&pa), Deref(&pb))
	Set(&pa, pb)
	*pa = 20
	fmt.Println(a, b, pa == pb)
	fmt.Println(IsNil(Nil[int]()), IsNil(&a), IsNil[Counter](nil), Nil[string]() == nil)
	ppp := Chain("deep")
	***ppp += "er"
	fmt.Println(***ppp)
}

// Output:
// 3 2 2 2
// true false false true
// 1 2
// 1 20 true
// true false true true
// deeper
