// phase: 09
// lsan: cycle
// container/list and container/ring: the elements of a list and the nodes of a ring point at each other (a cycle that the program drops is not freed, D1).

package main

import (
	"container/list"
	"container/ring"
	"fmt"
	"strings"
)

func main() {
	// container/list
	l := list.New()
	e4 := l.PushBack(4)
	e1 := l.PushFront(1)
	l.InsertBefore(3, e4)
	l.InsertAfter(2, e1)
	l.MoveToBack(e1)
	var vals []string
	for e := l.Front(); e != nil; e = e.Next() {
		vals = append(vals, fmt.Sprint(e.Value))
	}
	fmt.Println(strings.Join(vals, " "), l.Len(), l.Front().Value, l.Back().Value)
	l.Remove(e4)
	vals = nil
	for e := l.Back(); e != nil; e = e.Prev() {
		vals = append(vals, fmt.Sprint(e.Value))
	}
	fmt.Println(strings.Join(vals, " "))
	l.Init()

	// container/ring
	r := ring.New(5)
	for i := 0; i < r.Len(); i++ {
		r.Value = i
		r = r.Next()
	}
	sum := 0
	r.Do(func(v any) { sum += v.(int) })
	r = r.Move(2)
	r.Unlink(2)
	var parts []string
	r.Do(func(v any) { parts = append(parts, fmt.Sprint(v)) })
	fmt.Println(sum, r.Len(), strings.Join(parts, ","))
}

// Output:
// 2 3 4 1 4 2 1
// 1 3 2
// 10 3 2,0,1
