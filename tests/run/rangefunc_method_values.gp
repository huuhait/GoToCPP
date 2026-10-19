// phase: 07

package main

import "fmt"

type List struct{ items []string }

func (l *List) All(yield func(int, string) bool) {
	for i, s := range l.items {
		if !yield(i, s) {
			return
		}
	}
}

func (l List) Values(yield func(string) bool) {
	for _, s := range l.items {
		if !yield(s) {
			return
		}
	}
}

func (l *List) Backward() func(yield func(int, string) bool) {
	return func(yield func(int, string) bool) {
		for i := len(l.items) - 1; i >= 0; i-- {
			if !yield(i, l.items[i]) {
				return
			}
		}
	}
}

type Tree struct {
	Left, Right *Tree
	Val         int
}

func (t *Tree) Walk(yield func(int) bool) bool {
	if t == nil {
		return true
	}
	return t.Left.Walk(yield) && yield(t.Val) && t.Right.Walk(yield)
}

func (t *Tree) All() func(yield func(int) bool) {
	return func(yield func(int) bool) { t.Walk(yield) }
}

type Counter struct{ n int }

func (c *Counter) Up(yield func(int) bool) {
	for c.n < 5 {
		c.n++
		if !yield(c.n) {
			return
		}
	}
}

func main() {
	l := &List{items: []string{"a", "b", "c"}}
	for i, s := range l.All {
		fmt.Println(i, s)
	}
	for s := range l.Values {
		fmt.Println(s)
	}
	for i, s := range l.Backward() {
		fmt.Println(i, s)
	}
	var v List = *l
	for s := range v.Values {
		fmt.Println("v", s)
	}
	f := l.All
	for i := range f {
		fmt.Println("f", i)
	}
	t := &Tree{&Tree{nil, nil, 1}, &Tree{&Tree{nil, nil, 3}, nil, 4}, 2}
	for x := range t.All() {
		fmt.Println("tree", x)
		if x == 3 {
			break
		}
	}
	var c Counter
	for x := range c.Up {
		if x == 2 {
			break
		}
	}
	fmt.Println("counter", c.n)
	for x := range c.Up {
		fmt.Println("up", x)
	}
	fmt.Println("counter", c.n)
}

// Output:
// 0 a
// 1 b
// 2 c
// a
// b
// c
// 2 c
// 1 b
// 0 a
// v a
// v b
// v c
// f 0
// f 1
// f 2
// tree 1
// tree 2
// tree 3
// counter 2
// up 3
// up 4
// up 5
// counter 5
