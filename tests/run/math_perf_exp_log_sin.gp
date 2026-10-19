// phase: 09
// A workload of 3 million Exp, Log, Sin, Cos, Pow, Atan calls: deterministic checksum equal to Go's.

package main

import (
	"fmt"
	"math"
)


func main() {

	var s float64
	x := 0.001
	for i := 0; i < 3000000; i++ {
		x += 0.00137
		if x > 700 {
			x = 0.001
		}
		s += math.Exp(x/10) + math.Log(x) + math.Sin(x) + math.Cos(x*3) + math.Pow(x, 0.25) + math.Atan(x)
	}
	fmt.Printf("%016x\n", math.Float64bits(s))

}

// Output:
// 4731ae9dbab8844c
