package main

import "fmt"

func Sum(gp_func string, gp_line int, gp_text_xs string, xs ...int) int {
	total := 0
	for _, x := range xs {
		total += x
	}
	fmt.Printf("sum %s %d %q %d %d\n", gp_func, gp_line, gp_text_xs, len(xs), total)
	return total
}

func Join(gp_file string, gp_line int, gp_text_parts string, sep string, parts ...string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	fmt.Printf("join %s %d %q\n", gp_file, gp_line, gp_text_parts)
	return out
}

func main() {
	fmt.Println(Sum("main.main", 33, ""))
	fmt.Println(Sum("main.main", 34, "1", 1))
	fmt.Println(Sum("main.main", 35, "1, 2, 3", 1, 2, 3))
	xs := []int{4, 5}
	fmt.Println(Sum("main.main", 37, "xs", xs...))
	fmt.Println(Join("umacro_variadic.gp", 38, "\"a\", \"b\"", "-", "a", "b"))
	fmt.Println(Join("umacro_variadic.gp", 39, "", "+"))
}
