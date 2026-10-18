// phase: 05

// Interface variables that are boxed (captured and changed by closures, address taken) and
// pointers to interfaces (step 5.4).
package main

import "fmt"

type Counter interface{ Next() int }

type up struct{ n int }
type down struct{ n int }

func (u *up) Next() int   { u.n++; return u.n }
func (d *down) Next() int { d.n--; return d.n }

func swap(p *Counter, q Counter) Counter {
	old := *p
	*p = q
	return old
}

func main() {
	var c Counter = &up{}
	set := func(x Counter) { c = x }
	fmt.Println(c.Next(), c.Next())
	set(&down{10})
	fmt.Println(c.Next(), c.Next())
	pc := &c
	fmt.Println((*pc).Next())
	old := swap(&c, &up{100})
	fmt.Println(old.Next(), c.Next())
	cs := []Counter{&up{}, &down{}}
	pp := &cs[1]
	*pp = &up{50}
	fmt.Println(cs[1].Next(), cs[0].Next())
	m := map[string]Counter{"a": &up{}}
	m["a"].Next()
	fmt.Println(m["a"].Next())
	var a any = 1
	pa := &a
	*pa = "now a string"
	fmt.Println(a.(string))
	f := func() any { return a }
	a = 2.5
	fmt.Println(f().(float64))
}

// Output:
// 1 2
// 9 8
// 7
// 6 101
// 51 1
// 2
// now a string
// 2.5
