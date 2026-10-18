// phase: 05

// Boxing copies values: structs, arrays, slices and maps in interfaces, value and pointer receivers, method sets.
package main

import "fmt"

type P struct{ X, Y int }
type Big struct {
	A [8]int
	S string
	P P
}

func (p P) Sum() int   { return p.X + p.Y }
func (p *P) Inc()      { p.X++; p.Y++ }
func (b Big) First() int { return b.A[0] }

type Summer interface{ Sum() int }
type Incer interface{ Inc() }

func main() {
	p := P{1, 2}
	var s Summer = p
	p.Inc()
	fmt.Println(p, s.Sum(), s)
	pp := &p
	var s2 Summer = pp
	pp.Inc()
	fmt.Println(p, s2.Sum(), s2)
	var i Incer = pp
	i.Inc()
	fmt.Println(p)
	arr := [3]int{1, 2, 3}
	var a any = arr
	arr[0] = 99
	fmt.Println(a, arr, a.([3]int)[0])
	b := Big{A: [8]int{5, 6}, S: "hi", P: P{7, 8}}
	var ba any = b
	b.A[0] = 50
	b.P.X = 70
	fmt.Println(ba.(Big).First(), ba.(Big).P, b.First(), b.P)
	var ba2 any = ba
	fmt.Println(ba2 == ba, ba == any(b), ba2 == any(Big{A: [8]int{5, 6}, S: "hi", P: P{7, 8}}))
	sl := []P{{1, 1}, {2, 2}}
	var as any = sl
	sl[0].X = 100
	fmt.Println(as.([]P)[0].X)
	m := map[string]P{"a": {1, 2}}
	var am any = m
	m["a"] = P{3, 4}
	fmt.Println(am.(map[string]P)["a"])
	var ss []Summer
	for i := 0; i < 3; i++ {
		ss = append(ss, P{i, i * 2})
	}
	for _, x := range ss {
		fmt.Print(x.Sum(), " ")
	}
	fmt.Println()
	if _, ok := any(p).(Incer); !ok {
		fmt.Println("P value does not implement Incer")
	}
	if _, ok := any(&p).(Incer); ok {
		fmt.Println("*P implements Incer")
	}
	defer func() {
		r := recover()
		fmt.Println(r)
		err := r.(error)
		fmt.Println(err.Error())
	}()
	var x any = p
	_ = x.(Incer)
}
