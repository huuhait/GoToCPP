// phase: 09
// Long float sums and products: the order of operations is Go's (no reassociation, no vectorized reductions).

package main

import (
	"fmt"
	"math"
)


func main() {

	s, sgn := 0.0, 1.0
	for i := 0; i < 5000000; i++ {
		s += sgn / float64(2*i+1)
		sgn = -sgn
	}
	fmt.Printf("%016x %v\n", math.Float64bits(4*s), 4*s)
	h := 0.0
	for i := 1; i <= 3000000; i++ {
		h += 1 / float64(i)
	}
	fmt.Printf("%016x %v\n", math.Float64bits(h), h)
	p := 1.0
	for i := 1; i <= 200; i++ {
		p *= 1 + 1/float64(i*i)
	}
	fmt.Printf("%016x %v\n", math.Float64bits(p), p)
	var a [1000]float64
	for i := range a {
		a[i] = math.Sin(float64(i)) * 1e8
	}
	t := 0.0
	for k := 0; k < 100; k++ {
		for _, v := range a {
			t += v
		}
	}
	fmt.Printf("%016x %v\n", math.Float64bits(t), t)

}

// Output:
// 400921fb396c3a5f 3.1415924535897797
// 402efb90be4441d6 15.491338678199934
// 400d4326f0c31c72 3.657789116815871
// c19ec794e25a77c4 -1.290990645883475e+08
