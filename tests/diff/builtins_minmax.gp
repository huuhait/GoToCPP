// phase: 03

// The builtins min and max (Go 1.21) of integers, floats, strings and named types, with Go's
// rules for floats (a NaN wins, -0 is less than +0), constants (folded by the compiler) and
// mixed constant and variable arguments, and Go's order of evaluation for their arguments.
package main

import "fmt"

type Celsius float64

func (c Celsius) String() string { return fmt.Sprintf("%.1f°C", float64(c)) }

type Name string

type Level int8

var n int

func next(v int) int {
	n++
	fmt.Print("next", n, " ")
	return v * n
}

func main() {
	a, b, c := 3, -7, 5
	fmt.Println(min(a, b, c), max(a, b, c), min(a), max(b, 0), min(a, 10), max(1, 2, a))
	var i8 int8 = -128
	var u8 uint8 = 200
	var u64 uint64 = 1 << 63
	fmt.Println(min(i8, 0), max(i8, 127), min(u8, 255), max(u8, 1), max(u64, 1), min(u64, 1))
	zero := 0.0
	nan, inf, negz := zero/zero, 1/zero, -zero
	fmt.Println(min(1.5, zero), max(-1.5, zero), min(inf, 1), max(-inf, 1), min(-inf, inf))
	fmt.Println(min(nan, 1), max(1, nan), min(1, 2, nan), max(nan, inf), min(inf, nan, -inf))
	fmt.Println(1/min(negz, zero), 1/min(zero, negz), 1/max(negz, zero), 1/max(zero, negz), 1/min(negz, negz), 1/max(zero, zero))
	var f32 float32 = 0.1
	fmt.Println(min(f32, 0.2), max(f32, 0.05), min(float32(nan), f32))
	s, t := "banana", "apple"
	fmt.Println(min(s, t), max(s, t), min(s, t, "cherry"), max(s, "", t), min("b", s), max(s, "z"))
	fmt.Println(min(Celsius(20), Celsius(-3.5)), max(Celsius(1), 2, Celsius(zero)), min(Celsius(nan), 1))
	fmt.Println(min(Name("bob"), "al"), max(Name("bob"), Name("carl")), min(Level(3), -4), max(Level(-1), 0))
	const k = 10
	fmt.Println(min(k, 3), max(k, 3.5), min("x", "y"), min(1, 2.5) == 1)
	fmt.Println(min(next(1), next(5), next(-1)), max(next(2), a, next(1)))
	m := map[string]int{"a": 4}
	fmt.Println(min(m["a"], len(m)), max(len(s), len(t)), max(min(a, b), min(c, 100)))
	fmt.Println(max(n, next(1)), min(next(1), n), n)
	fmt.Println(min(len(s), next(1)), max(s, t)+fmt.Sprint(next(1)), min(n, 100)+next(1))
	x := min(a, b)
	x = max(x, next(3))
	fmt.Println(x)
}
