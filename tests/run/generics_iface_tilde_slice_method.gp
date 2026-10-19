// phase: 07

// Constraints that combine a core slice type with methods: interface{ ~[]E; Len() int }.
package main

import "fmt"

type Stack []int
type Names []string

func (s Stack) Len() int  { return len(s) }
func (n Names) Len() int  { return len(n) * 100 }
func (s *Stack) Push(x int) { *s = append(*s, x) }

func Info[S interface {
	~[]E
	Len() int
}, E any](s S) (int, int, E) {
	var last E
	if len(s) > 0 {
		last = s[len(s)-1]
	}
	return s.Len(), len(s), last
}

func Map[S interface {
	~[]E
	Len() int
}, E, R any](s S, f func(E) R) []R {
	out := make([]R, 0, s.Len())
	for _, e := range s {
		out = append(out, f(e))
	}
	return out
}

func Fill[S ~[]int, PS interface {
	*S
	Push(int)
}](n int) S {
	var s S
	ps := PS(&s)
	for i := range n {
		ps.Push(i * i)
	}
	return s
}

func Grow[S interface {
	~[]E
	Len() int
}, E any](s S, e E) S {
	if s.Len() > 3 {
		return s[:0]
	}
	return append(s, e)
}

func main() {
	fmt.Println(Info(Stack{1, 2, 3}))
	fmt.Println(Info(Names{"a", "b"}))
	fmt.Println(Info(Stack(nil)))
	fmt.Println(Map(Stack{1, 2, 3}, func(i int) string { return fmt.Sprint(i * 2) }), Map(Names{"x"}, func(s string) int { return len(s) }))
	fmt.Println(Fill[Stack](5), len(Fill[Stack](0)))
	s := Stack{}
	for i := 0; i < 6; i++ {
		s = Grow(s, i)
	}
	fmt.Printf("%v %T\n", s, Grow(Names{}, "z"))
}

// Output:
// 3 3 3
// 200 2 b
// 0 0 0
// [2 4 6] [1]
// [0 1 4 9 16] 0
// [5] main.Names
