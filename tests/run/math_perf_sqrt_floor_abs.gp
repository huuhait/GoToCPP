// phase: 09
// A workload of 30 million Sqrt, Floor, Ceil, Trunc, Abs, FMA calls (the natives must inline in an optimized build): deterministic checksum equal to Go's.

package main

import (
	"fmt"
	"math"
)


func main() {

	var s, f float64
	x := 1.0
	for i := 0; i < 30000000; i++ {
		x = x*1.0000001 + 0.001
		s += math.Sqrt(x) + math.Abs(-x)
		f += math.Floor(x*0.37) - math.Ceil(x*0.11) + math.Trunc(-x*0.07) + math.FMA(x, 0.5, 1)
		if x > 1e6 {
			x = 1
		}
	}
	fmt.Printf("%016x %016x\n", math.Float64bits(s), math.Float64bits(f))

}

// Output:
// 42777f90721c4db6 42702740e602b826
