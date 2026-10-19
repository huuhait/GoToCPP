// phase: 05

// Generic macros (step 10.9): Dbg returns its argument with the type it has, a macro can be
// instantiated by hand (#Dbg[float64](1)), and uses can be nested and sit in conditions, loop
// headers, arguments and composite literals. The twin (testdata/macro_twin/umacro_generic.go)
// is the same program with the macros written out by hand.

package main

import "fmt"

type Point struct{ X, Y int }

// Dbg prints where it is used, the text of v and its value, and returns v.
macro Dbg[T any](v T) T {
	fmt.Printf("[%s:%d] %s = %v (%T)\n", #file(), #line(), #text(v), v, v)
	return v
}

// Max returns the larger of a and b and says where it was asked.
macro Max[T int | float64 | string](a, b T) T {
	fmt.Printf("max at %s:%d in %s\n", #file(), #line(), #func())
	if a > b {
		return a
	}
	return b
}

// Pair makes a two-element slice; its type parameters are inferred or given.
macro Pair[A, B any](a A, b B) []any {
	return []any{a, b, #text(a), #text(b)}
}

func double(n int) int { return n * 2 }

func main() {
	n := #Dbg(double(21))
	var f float64 = #Dbg(1.5)
	s := #Dbg("x" + "y")
	p := #Dbg(Point{1, 2})
	q := #Dbg(&p)
	fmt.Println(n, f, s, p, q.X)

	// Explicit type arguments: the constant takes the type given.
	g := #Dbg[float64](1)
	h := #Dbg[int8](100)
	fmt.Println(g/3, h+27)

	// Nested uses: the inner one runs first.
	total := #Dbg(#Dbg(2) + #Dbg(3))
	fmt.Println(total)

	// In a condition, a loop header and an argument.
	if #Dbg(n) > 40 {
		fmt.Println("big")
	}
	for i := 0; i < #Dbg(2); i++ {
		fmt.Println("i =", i)
	}
	fmt.Println(double(#Dbg(4)), #Dbg("a"), #Dbg(true))

	// Max for each type; the macro knows the function that uses it.
	fmt.Println(#Max(3, 7), #Max(2.5, 1.5), #Max("pear", "apple"))
	fmt.Println(#Max[float64](1, 2.5))

	// Pair with the texts of its arguments.
	fmt.Println(#Pair(1, "one"))
	fmt.Println(#Pair[int, any](n, nil))

	// A composite literal and a map.
	pts := []Point{#Dbg(Point{3, 4}), {5, 6}}
	m := map[string]int{"k": #Dbg(9)}
	fmt.Println(pts, m)
}

// Output:
// [umacro_generic.gp:37] double(21) = 42 (int)
// [umacro_generic.gp:38] 1.5 = 1.5 (float64)
// [umacro_generic.gp:39] "x" + "y" = xy (string)
// [umacro_generic.gp:40] Point{1, 2} = {1 2} (main.Point)
// [umacro_generic.gp:41] &p = &{1 2} (*main.Point)
// 42 1.5 xy {1 2} 1
// [umacro_generic.gp:45] 1 = 1 (float64)
// [umacro_generic.gp:46] 100 = 100 (int8)
// 0.3333333333333333 127
// [umacro_generic.gp:50] 2 = 2 (int)
// [umacro_generic.gp:50] 3 = 3 (int)
// [umacro_generic.gp:50] #Dbg(2) + #Dbg(3) = 5 (int)
// 5
// [umacro_generic.gp:54] n = 42 (int)
// big
// [umacro_generic.gp:57] 2 = 2 (int)
// i = 0
// [umacro_generic.gp:57] 2 = 2 (int)
// i = 1
// [umacro_generic.gp:57] 2 = 2 (int)
// [umacro_generic.gp:60] 4 = 4 (int)
// [umacro_generic.gp:60] "a" = a (string)
// [umacro_generic.gp:60] true = true (bool)
// 8 a true
// max at umacro_generic.gp:63 in main.main
// max at umacro_generic.gp:63 in main.main
// max at umacro_generic.gp:63 in main.main
// 7 2.5 pear
// max at umacro_generic.gp:64 in main.main
// 2.5
// [1 one 1 "one"]
// [42 <nil> n nil]
// [umacro_generic.gp:71] Point{3, 4} = {3 4} (main.Point)
// [umacro_generic.gp:72] 9 = 9 (int)
// [{3 4} {5 6}] map[k:9]
