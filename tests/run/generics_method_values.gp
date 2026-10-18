// phase: 07

// Method values and method expressions on type arguments, stored, called later, and bound to copies.
package main

import "fmt"

type Counter struct{ n int }

func (c *Counter) Inc() int  { c.n++; return c.n }
func (c Counter) Value() int { return c.n }

type Temp float64

func (t Temp) Double() Temp { return t * 2 }

type Valuer interface{ Value() int }

func Bind[T Valuer](x T) func() int { return x.Value }

func Express[T Valuer](x T) int {
	f := T.Value
	return f(x)
}

func Collect[T Valuer](xs []T) []func() int {
	var out []func() int
	for _, x := range xs {
		out = append(out, x.Value)
	}
	return out
}

func Apply[T interface{ Double() T }](x T, n int) T {
	f := T.Double
	for range n {
		x = f(x)
	}
	return x
}

func main() {
	c := Counter{3}
	get := Bind(c)
	c.n = 10
	fmt.Println(get(), c.Value(), Express(c), Express(&c))
	cs := []Counter{{1}, {2}}
	fs := Collect(cs)
	cs[0].n = 99
	fmt.Println(fs[0](), fs[1]())
	pc := &Counter{5}
	pget := Bind(pc)
	fmt.Println(pget(), pc.Inc(), Apply(Temp(1.5), 3))
}

// Output:
// 3 10 10 10
// 1 2
// 5 6 12
