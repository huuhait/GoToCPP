// phase: 07

// Methods promoted from embedded fields satisfy constraints: value embedding, pointer embedding, interface embedding.
package main

import "fmt"

type IDer interface{ ID() int }
type Incer interface{ Inc() int }

type Base struct{ id int }

func (b Base) ID() int { return b.id }

type Derived struct {
	Base
	extra string
}

type Deeper struct{ Derived }

type Counter struct{ n int }

func (c *Counter) Inc() int { c.n++; return c.n }

type WithCounter struct {
	*Counter
	name string
}

type Wrapper struct{ IDer }

func IDs[T IDer](xs ...T) []int {
	var out []int
	for _, x := range xs {
		out = append(out, x.ID())
	}
	return out
}

func Bump[T Incer](x T, times int) int {
	r := 0
	for range times {
		r = x.Inc()
	}
	return r
}

func main() {
	fmt.Println(IDs(Base{1}), IDs(Derived{Base{2}, "x"}, Derived{Base{3}, "y"}), IDs(Deeper{Derived{Base{4}, ""}}))
	fmt.Println(IDs(&Derived{Base{5}, ""}), IDs[IDer](Base{6}, Derived{Base{7}, ""}))
	fmt.Println(IDs(Wrapper{Base{8}}), IDs(Wrapper{Derived{Base{9}, ""}}))
	w := WithCounter{&Counter{}, "w"}
	fmt.Println(Bump(w, 3), Bump(&w, 2), w.n)
	fmt.Println(Bump(w.Counter, 1))
}

// Output:
// [1] [2 3] [4]
// [5] [6 7]
// [8] [9]
// 3 5 5
// 6
