// phase: 05

// Unnamed interface types as a field, an element of a slice, a map and a channel, a pointer target,
// a parameter and a result, a type argument, and in an assertion; two spellings are one type.
package main

import "fmt"

type S struct{ n int }

func (s S) Get() int      { return s.n }
func (s S) Set(n int) int { return n }

type Box struct {
	a  interface{ Get() int }
	bs []interface{ Get() int }
	m  map[string]interface{ Get() int }
	c  chan interface{ Get() int }
	f  func(interface{ Get() int }) int
	p  *interface{ Get() int }
	ar [2]interface{ Get() int }
}

func take(v interface{ Get() int }) int { return v.Get() }
func takes(vs []interface{ Get() int }) int {
	t := 0
	for _, v := range vs {
		t += v.Get()
	}
	return t
}
func ret() interface{ Get() int } { return S{7} }
func rets() (interface{ Get() int }, error) { return S{8}, nil }

type Gen[T interface{ Get() int }] struct {
	v T
	a interface{ Get() int }
}

func (g Gen[T]) Sum() int { return g.v.Get() + g.a.Get() }

func Map[T any](xs []T, f func(T) interface{ Get() int }) []interface{ Get() int } {
	var out []interface{ Get() int }
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

func main() {
	var b Box
	fmt.Println(b.a == nil, len(b.bs), len(b.m), b.c == nil, b.f == nil, b.p == nil)
	b.a = S{1}
	b.bs = append(b.bs, S{2}, S{3})
	b.m = map[string]interface{ Get() int }{"x": S{4}}
	b.c = make(chan interface{ Get() int }, 1)
	b.c <- S{5}
	b.f = take
	b.ar[1] = S{6}
	var v interface{ Get() int } = S{9}
	b.p = &v
	fmt.Println(b.a.Get(), takes(b.bs), b.m["x"].Get(), (<-b.c).Get(), b.f(S{10}), (*b.p).Get(), b.ar[1].Get(), b.ar[0] == nil)
	fmt.Println(ret().Get())
	r, err := rets()
	fmt.Println(r.Get(), err)
	g := Gen[S]{S{1}, S{2}}
	fmt.Println(g.Sum())
	fmt.Println(takes(Map([]int{1, 2}, func(i int) interface{ Get() int } { return S{i} })))
	var anon struct{ x interface{ Get() int } }
	anon.x = S{11}
	fmt.Println(anon.x.Get())
	// identical anonymous interface types are assignable
	var e1 interface{ Get() int } = S{12}
	var e2 interface {
		Get() int
	} = e1
	fmt.Println(e2.Get())
	// assertion to an anonymous interface
	var any1 any = S{13}
	if q, ok := any1.(interface{ Get() int }); ok {
		fmt.Println(q.Get())
	}
	// two methods, different order
	var two interface {
		Set(int) int
		Get() int
	} = S{14}
	fmt.Println(two.Get(), two.Set(3))
	fmt.Printf("%T\n", e1)
}

// Output:
// true 0 0 true true true
// 1 5 4 5 10 9 6 true
// 7
// 8 <nil>
// 3
// 3
// 11
// 12
// 13
// 14 3
// main.S
