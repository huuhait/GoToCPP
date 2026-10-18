// phase: 07

// Instances of generic functions: one function for each list of type arguments, named
// gp_inst_<hash>_<name>, written before the functions of the unit with a prototype first.
package main

import "fmt"

type Number interface{ ~int | ~int8 | ~float64 }

type Celsius float64

func (c Celsius) String() string { return fmt.Sprintf("%.1f C", float64(c)) }

func Sum[T Number](xs ...T) T {
	var s T
	for _, x := range xs {
		s += x
	}
	return s
}

func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

func Show[T fmt.Stringer](x T) string { return "<" + x.String() + ">" }

func Kind[T any](v T) string {
	switch any(v).(type) {
	case int:
		return "int"
	case T:
		return "T"
	}
	return "?"
}

func main() {
	fmt.Println(Sum(1, 2, 3), Sum[int8](100, 100), Sum(Celsius(1.5), 2))
	fmt.Println(Map([]int{1, 2}, func(i int) string { return fmt.Sprint(i) }))
	fmt.Println(Show(Celsius(36.6)), Kind(1), Kind("s"))
	f := Map[string, int]
	fmt.Println(f([]string{"ab"}, func(s string) int { return len(s) }))
}
