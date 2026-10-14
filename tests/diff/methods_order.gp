// phase: 03

// Go's order for method calls (step 3.10): the receiver is evaluated with the arguments, its
// calls first (h().M(f()) calls h, then f) and its reads after the calls of the arguments (g.M(f()),
// where f changes g, calls M on the new g; the same for a value, an element and a field), and a
// deferred method call copies a value receiver (and *p) at the defer statement, while a pointer
// receiver sees the changes made after it.
package main

import "fmt"

type T struct{ v int }

func (t T) M(a int) int   { fmt.Println("M", t.v, a); return t.v }
func (t *T) PM(a int) int { fmt.Println("PM", t.v, a); return t.v }

var g = &T{1}
var gv = T{10}
var gs = []T{{100}, {200}}
var gps = []*T{{1000}}

func f() int { fmt.Println("f"); g = &T{2}; gv = T{20}; gs[0].v++; gps[0] = &T{2000}; return 0 }
func h() *T  { fmt.Println("h"); return &T{3} }
func hv() T  { fmt.Println("hv"); return T{4} }

type W struct {
	t  T
	pt *T
}

var gw = W{T{7}, &T{8}}

func fw() int { fmt.Println("fw"); gw.t.v = 70; gw.pt = &T{80}; return 1 }

func main() {
	g.M(f())
	gv.M(f())
	g = &T{1}
	g.PM(f())
	gv = T{10}
	gv.PM(f())
	h().M(f())
	h().PM(f())
	hv().M(f())
	gs[0].M(f())
	gs[1].PM(f())
	gps[0].M(f())
	gps[0].PM(f())
	gw.t.M(fw())
	gw.pt.M(fw())
	gw.t.PM(fw())
	gw.pt.PM(fw())
	x := T{5}
	x.M(x.PM(1))
	px := &x
	px.M(px.PM(2) + x.PM(3))
	fmt.Println(x.M(1) + px.M(2) + gv.M(3))
	deferred()
}

func deferred() {
	{
		x := T{5}
		defer x.M(1)
		x.v = 6
		q := &T{7}
		defer q.M(2)
		q.v = 8
		defer q.PM(3)
		q.v = 9
		defer x.PM(4)
		x.v = 11
		defer gw.pt.M(fw())
		defer gs[0].PM(f())
	}
}
