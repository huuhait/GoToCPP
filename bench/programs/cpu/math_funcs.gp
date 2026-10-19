// bench: math_funcs
// category: cpu
// size: 6000000
// smoke: 3000
// twin: no
// about: math.Sin, Cos, Exp, Log, Pow and Atan in a loop (the ported math package, bit-exact with Go)
package main

import (
	"fmt"
	"math"
)

func main() {
	var s float64
	for i := 0; i < benchN; i++ {
		x := float64(i%1000) / 100
		s += math.Sin(x) + math.Cos(x) + math.Exp(x/10) + math.Log(x+1) + math.Pow(x, 1.5) + math.Atan(x)
	}
	fmt.Printf("%.3f\n", s/float64(benchN))
}
