// phase: 09
// lsan: cycle

package main

import (
	"fmt"
	"reflect"
)

type Node struct {
	Val  int
	Next *Node
}
type Tree struct {
	Kids   []*Tree
	Parent *Tree
}

func main() {
	a := &Node{1, nil}
	a.Next = a
	b := &Node{1, nil}
	b.Next = b
	fmt.Println(reflect.DeepEqual(a, b))
	c := &Node{2, nil}
	c.Next = c
	fmt.Println(reflect.DeepEqual(a, c))
	x1 := &Node{1, &Node{2, nil}}
	x1.Next.Next = x1
	x2 := &Node{1, &Node{2, nil}}
	x2.Next.Next = x2
	fmt.Println(reflect.DeepEqual(x1, x2))
	x2.Next.Val = 3
	fmt.Println(reflect.DeepEqual(x1, x2))
	t1 := &Tree{}
	t1.Kids = []*Tree{{Parent: t1}, {Parent: t1}}
	t2 := &Tree{}
	t2.Kids = []*Tree{{Parent: t2}, {Parent: t2}}
	fmt.Println(reflect.DeepEqual(t1, t2))
	t2.Kids = t2.Kids[:1]
	fmt.Println(reflect.DeepEqual(t1, t2))
	m1 := map[string]any{}
	m1["self"] = m1
	m2 := map[string]any{}
	m2["self"] = m2
	fmt.Println(reflect.DeepEqual(m1, m2))
	s1 := make([]any, 1)
	s1[0] = s1
	s2 := make([]any, 1)
	s2[0] = s2
	fmt.Println(reflect.DeepEqual(s1, s2))
	fmt.Println(reflect.DeepEqual(a, a), reflect.DeepEqual(m1, m1))
}

// Output:
// true
// false
// true
// false
// true
// false
// true
// true
// true true
