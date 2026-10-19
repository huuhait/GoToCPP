// The hand-written Go twin of umacro_generic.gp.

package main

import "fmt"

type Point struct{ X, Y int }

func Dbg[T any](gp_file string, gp_line int, gp_func string, gp_text_v string, v T) T {
	fmt.Printf("[%s:%d] %s = %v (%T)\n", gp_file, gp_line, gp_text_v, v, v)
	return v
}

func Max[T int | float64 | string](gp_file string, gp_line int, gp_func string, a, b T) T {
	fmt.Printf("max at %s:%d in %s\n", gp_file, gp_line, gp_func)
	if a > b {
		return a
	}
	return b
}

func Pair[A, B any](gp_file string, gp_line int, gp_func string, gp_text_a string, gp_text_b string, a A, b B) []any {
	return []any{a, b, gp_text_a, gp_text_b}
}

func double(n int) int { return n * 2 }

func main() {
	n := Dbg("umacro_generic.gp", 37, "main.main", "double(21)", double(21))
	var f float64 = Dbg("umacro_generic.gp", 38, "main.main", "1.5", 1.5)
	s := Dbg("umacro_generic.gp", 39, "main.main", `"x" + "y"`, "x"+"y")
	p := Dbg("umacro_generic.gp", 40, "main.main", "Point{1, 2}", Point{1, 2})
	q := Dbg("umacro_generic.gp", 41, "main.main", "&p", &p)
	fmt.Println(n, f, s, p, q.X)

	g := Dbg[float64]("umacro_generic.gp", 45, "main.main", "1", 1)
	h := Dbg[int8]("umacro_generic.gp", 46, "main.main", "100", 100)
	fmt.Println(g/3, h+27)

	total := Dbg("umacro_generic.gp", 50, "main.main", "#Dbg(2) + #Dbg(3)",
		Dbg("umacro_generic.gp", 50, "main.main", "2", 2)+Dbg("umacro_generic.gp", 50, "main.main", "3", 3))
	fmt.Println(total)

	if Dbg("umacro_generic.gp", 54, "main.main", "n", n) > 40 {
		fmt.Println("big")
	}
	for i := 0; i < Dbg("umacro_generic.gp", 57, "main.main", "2", 2); i++ {
		fmt.Println("i =", i)
	}
	fmt.Println(double(Dbg("umacro_generic.gp", 60, "main.main", "4", 4)), Dbg("umacro_generic.gp", 60, "main.main", `"a"`, "a"), Dbg("umacro_generic.gp", 60, "main.main", "true", true))

	fmt.Println(Max("umacro_generic.gp", 63, "main.main", 3, 7), Max("umacro_generic.gp", 63, "main.main", 2.5, 1.5), Max("umacro_generic.gp", 63, "main.main", "pear", "apple"))
	fmt.Println(Max[float64]("umacro_generic.gp", 64, "main.main", 1, 2.5))

	fmt.Println(Pair("umacro_generic.gp", 67, "main.main", "1", `"one"`, 1, "one"))
	fmt.Println(Pair[int, any]("umacro_generic.gp", 68, "main.main", "n", "nil", n, nil))

	pts := []Point{Dbg("umacro_generic.gp", 71, "main.main", "Point{3, 4}", Point{3, 4}), {5, 6}}
	m := map[string]int{"k": Dbg("umacro_generic.gp", 72, "main.main", "9", 9)}
	fmt.Println(pts, m)
}
