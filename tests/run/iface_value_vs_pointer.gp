// phase: 05

// A struct value in an interface is a copy; a pointer shares the object (step 5.4). Method sets:
// the pointer has the methods of the value too.
package main

import "fmt"

type Counter interface {
	Inc() int
	Get() int
}

type Cnt struct{ n int }

func (c *Cnt) Inc() int { c.n++; return c.n }
func (c *Cnt) Get() int { return c.n }

type V struct{ n int }

func (v V) Get() int { return v.n }

type Getter interface{ Get() int }

func main() {
	c := Cnt{}
	var cnt Counter = &c
	cnt.Inc()
	cnt.Inc()
	fmt.Println(c.n, cnt.Get())
	v := V{1}
	var g Getter = v
	v.n = 2
	fmt.Println(v.Get(), g.Get())
	var gp Getter = &v
	v.n = 3
	fmt.Println(gp.Get(), g.Get())
	g2 := g
	fmt.Println(g2.Get())
	pv := gp.(*V)
	pv.n = 9
	fmt.Println(v.n, gp.Get())
	vv := g.(V)
	vv.n = 100
	fmt.Println(g.Get(), vv.Get())
	var anyc any = Cnt{}
	_, ok := anyc.(Counter)
	_, ok2 := any(&c).(Counter)
	_, ok3 := any(&v).(Getter)
	_, ok4 := any(v).(Getter)
	fmt.Println(ok, ok2, ok3, ok4)
}

// Output:
// 2 2
// 2 1
// 3 1
// 1
// 9 9
// 1 100
// false true true true
