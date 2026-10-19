// phase: 07
// lsan: cycle

package main

import "fmt"

func repeat(s string, n int) string {
	r := ""
	for range n {
		r += s
	}
	return r
}

type Node struct {
	Name     string
	Children []*Node
}

func (n *Node) Walk(depth int) func(yield func(string, int) bool) {
	return func(yield func(string, int) bool) {
		if !yield(n.Name, depth) {
			return
		}
		for _, c := range n.Children {
			for name, d := range c.Walk(depth + 1) {
				if !yield(name, d) {
					return
				}
			}
		}
	}
}

func Cnt(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

func findDeep(n *Node, want string) (depth int, found bool) {
	for name, d := range n.Walk(0) {
		if name == want {
			return d, true
		}
	}
	return -1, false
}

var pkgLit = func() (s []int) {
	for x := range Cnt(4) {
		if x == 3 {
			return s
		}
		s = append(s, x*x)
	}
	return nil
}

var table = func() map[int]string {
	m := map[int]string{}
	for i := range Cnt(3) {
		m[i] = repeat("*", i)
	}
	return m
}()

func init() {
	for x := range Cnt(2) {
		fmt.Println("init", x)
	}
}

func main() {
	tree := &Node{"root", []*Node{{"a", []*Node{{"a1", nil}, {"a2", nil}}}, {"b", nil}}}
	for name, d := range tree.Walk(0) {
		fmt.Println(repeat("  ", d) + name)
	}
	fmt.Println(findDeep(tree, "a2"))
	fmt.Println(findDeep(tree, "zz"))
	fmt.Println(pkgLit(), table)
	f := func(n int) (r int) {
		for x := range Cnt(n) {
			for y := range Cnt(n) {
				if x+y == 3 {
					return x*10 + y
				}
			}
		}
		return -1
	}
	fmt.Println(f(3), f(5))
	g := func() func() int {
		total := 0
		for x := range Cnt(4) {
			total += x
		}
		return func() int { total++; return total }
	}()
	fmt.Println(g(), g())
	var rec func(n int) int
	rec = func(n int) int {
		if n == 0 {
			return 0
		}
		sum := 0
		for x := range Cnt(n) {
			sum += x + rec(x)
		}
		return sum
	}
	fmt.Println(rec(5))
}

// Output:
// init 0
// init 1
// root
//   a
//     a1
//     a2
//   b
// 2 true
// -1 false
// [0 1 4] map[0: 1:* 2:**]
// 12 3
// 7 8
// 26
