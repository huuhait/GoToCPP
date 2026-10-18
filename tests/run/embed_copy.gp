// phase: 05

// A struct that embeds a value is copied with it (assignment, arguments, results, range, map
// and slice elements); one that embeds a pointer shares what the pointer points to.
package main

import "fmt"

type Inner struct{ N int }

func (i *Inner) Inc()    { i.N++ }
func (i Inner) Get() int { return i.N }

type ByValue struct {
	Inner
	S []int
}

type ByPointer struct {
	*Inner
}

func pass(b ByValue) ByValue {
	b.Inc()
	b.S[0] = 99
	return b
}

func main() {
	a := ByValue{Inner{1}, []int{1, 2}}
	b := a
	b.Inc()
	fmt.Println(a.N, b.N)
	c := pass(a)
	fmt.Println(a.N, c.N, a.S, c.S) // the slice is shared, the Inner is not

	p := ByPointer{&Inner{1}}
	q := p
	q.Inc()
	fmt.Println(p.N, q.N, p.Inner == q.Inner)

	arr := [2]ByValue{{Inner{1}, nil}, {Inner{2}, nil}}
	arr2 := arr
	arr2[0].Inc()
	for _, e := range arr {
		e.Inc()
	}
	for i := range arr {
		arr[i].Inc()
	}
	fmt.Println(arr[0].N, arr[1].N, arr2[0].N)

	s := []ByValue{{Inner{5}, nil}}
	t := append([]ByValue(nil), s...)
	t[0].Inc()
	s[0].Inc()
	s[0].Inc()
	fmt.Println(s[0].N, t[0].N)

	m := map[string]ByValue{"k": {Inner{7}, nil}}
	v := m["k"]
	v.Inc()
	fmt.Println(m["k"].N, v.N, m["k"].Get())
	m["k"] = v
	fmt.Println(m["k"].N)
}

// Output:
// 1 2
// 1 2 [99 2] [99 2]
// 2 2 true
// 2 3 2
// 7 6
// 7 8 7
// 8
