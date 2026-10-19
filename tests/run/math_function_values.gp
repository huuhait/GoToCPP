// phase: 09
// math functions as function values: assigned to variables, stored in slices and maps, passed as arguments, returned from functions.

package main

import (
	"fmt"
	"math"
)


func main() {

	f := math.Sqrt
	g := math.Sin
	fs := []func(float64) float64{math.Sqrt, math.Floor, math.Exp, math.Log, math.Abs, math.Cbrt, math.Trunc, math.Ceil, math.RoundToEven, math.Sin}
	for _, fn := range fs {
		fmt.Printf("%016x %016x\n", math.Float64bits(fn(2.5)), math.Float64bits(fn(-0.5)))
	}
	m := map[string]func(float64, float64) float64{"pow": math.Pow, "hypot": math.Hypot, "max": math.Max, "mod": math.Mod}
	for _, k := range []string{"hypot", "max", "mod", "pow"} {
		fmt.Println(k, m[k](7, 3))
	}
	apply := func(fn func(float64) float64, v float64) float64 { return fn(v) }
	fmt.Println(f(16), g(0), apply(math.Floor, -2.5), apply(f, 2))
	bits := math.Float64bits
	fb := math.Float64frombits
	fmt.Println(bits(1.5), fb(0x4000000000000000))
	var fma = math.FMA
	fmt.Println(fma(2, 3, 4))

}

// Output:
// 3ff94c583ada5b53 fff8000000000000
// 4000000000000000 bff0000000000000
// 40285d6fd931e0bb 3fe368b2fc6f960a
// 3fed5240f0e0e078 7ff8000000000001
// 4004000000000000 3fe0000000000000
// 3ff5b7209557b0ee bfe965fea53d6e3c
// 4000000000000000 8000000000000000
// 4008000000000000 8000000000000000
// 4000000000000000 8000000000000000
// 3fe326af0dcfcab0 bfdeaee8744b05f0
// hypot 7.615773105863909
// max 7
// mod 1
// pow 343
// 4 0 -3 1.4142135623730951
// 4609434218613702656 2
// 10
