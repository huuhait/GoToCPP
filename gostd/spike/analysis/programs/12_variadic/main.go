// Variadic functions: no arguments, one, several, a slice spread with ..., an empty spread, and
// a variadic function value.
package main

func sum(nums ...int) int {
	t := 0
	for _, n := range nums {
		t += n
	}
	return t
}

func join(sep string, parts ...string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

func main() {
	println(sum(), sum(1), sum(1, 2, 3))
	xs := []int{4, 5, 6}
	println(sum(xs...))
	println(join("-", "a", "b", "c"))
	println(join("+", []string{"x", "y"}...))
	f := sum
	println(f(7, 8))
}
