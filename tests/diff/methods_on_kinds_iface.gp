// phase: 08

// Methods on slice, map, func, bool, string, array and chan types, through interfaces and as method values.
package main

import "fmt"

type IntSlice []int
type Counter map[string]int
type Op func(int, int) int
type Flag bool
type Str string
type Arr [3]int
type Ch chan int

func (s IntSlice) Sum() (t int) {
	for _, x := range s {
		t += x
	}
	return
}
func (s *IntSlice) Push(x int)            { *s = append(*s, x) }
func (c Counter) Inc(k string) Counter    { c[k]++; return c }
func (o Op) Apply(a, b int) int           { return o(a, b) }
func (f Flag) Not() Flag                  { return !f }
func (f *Flag) Toggle()                   { *f = !*f }
func (s Str) Len() int                    { return len(s) }
func (s Str) String() string              { return "Str(" + string(s) + ")" }
func (a Arr) Sum() int                    { return a[0] + a[1] + a[2] }
func (a *Arr) Set(i, v int)               { a[i] = v }
func (c Ch) Send(v int)                   { c <- v }

type Summer interface{ Sum() int }
type Applier interface{ Apply(int, int) int }
type Lener interface{ Len() int }

func main() {
	s := IntSlice{1, 2}
	s.Push(3)
	var sm Summer = s
	s.Push(4)
	fmt.Println(sm.Sum(), s.Sum(), len(s))
	ps := &s
	var sm2 Summer = ps
	ps.Push(5)
	fmt.Println(sm2.Sum())
	c := Counter{}
	c.Inc("a").Inc("a").Inc("b")
	fmt.Println(c, len(c))
	var ap Applier = Op(func(a, b int) int { return a*10 + b })
	fmt.Println(ap.Apply(4, 2), Op(func(a, b int) int { return a - b }).Apply(5, 3))
	f := Flag(true)
	f.Toggle()
	fmt.Println(f, f.Not())
	var l Lener = Str("hello")
	fmt.Println(l.Len(), l, Str("x"))
	fmt.Printf("%v %s %d\n", l, l, l)
	a := Arr{1, 2, 3}
	var as Summer = a
	a.Set(0, 10)
	fmt.Println(as.Sum(), a.Sum(), a)
	var as2 Summer = &a
	a.Set(1, 20)
	fmt.Println(as2.Sum())
	ch := make(Ch, 2)
	ch.Send(1)
	ch.Send(2)
	fmt.Println(<-ch, <-ch, len(ch))
	fmt.Printf("%T %T %T %T %T\n", s, c, ap, f, ch)
	var x any = s
	switch v := x.(type) {
	case []int:
		fmt.Println("plain slice", v)
	case IntSlice:
		fmt.Println("IntSlice", v.Sum())
	}
	m := map[Str]IntSlice{"a": {1, 2}, "b": nil}
	fmt.Println(m["a"].Sum(), m["b"].Sum(), m["z"].Sum(), m)
	fn := IntSlice.Sum
	fn2 := (*IntSlice).Push
	fn2(&s, 100)
	fmt.Println(fn(s))
}
