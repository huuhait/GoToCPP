// phase: 07

// Method values of type parameters: the receiver is evaluated and copied when the value is made.
// (Not tested: a value method bound through a type argument that is a pointer, `x.Get` with T = *Counter:
// the spec says *x is copied when the method value is evaluated, gc reads it at the call.)
package main

import "fmt"

type Counter struct{ n int }

func (c Counter) Get() int   { return c.n }
func (c *Counter) Inc() int  { c.n++; return c.n }

type Getter interface{ Get() int }
type Incer interface{ Inc() int }
type Both interface {
	Getter
	Incer
}

func Bind[T Getter](x T) func() int { return x.Get }

func BindInc[T Incer](x T) func() int { return x.Inc }

func Snapshot[T Both](x T) func() int {
	return x.Inc
}

func Table[T Getter](xs []T) (fs []func() int) {
	for _, x := range xs {
		fs = append(fs, x.Get)
	}
	return
}

func main() {
	c := Counter{1}
	get := Bind(c)
	cp := &c
	inc := BindInc(cp)
	c.n = 10
	fmt.Println(get(), inc(), inc(), c.n)
	i := Snapshot(cp)
	i()
	i()
	fmt.Println(c.Get(), c.n)
	fs := Table([]Counter{{1}, {2}, {3}})
	fmt.Println(fs[0](), fs[1](), fs[2]())
	var gi Getter = Counter{42}
	fmt.Println(Bind(gi)(), Table([]Getter{Counter{5}, Counter{6}})[1]())
	h := Counter.Get
	k := (*Counter).Inc
	fmt.Println(h(Counter{8}), k(&c))
}

// Output:
// 1 11 12 12
// 14 14
// 1 2 3
// 42 6
// 8 15
