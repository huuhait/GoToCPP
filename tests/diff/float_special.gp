// phase: 02

// Infinities, NaN and negative zero: how fmt prints them (+Inf, -Inf, NaN,
// -0), and how they compare and compute.
package main

import "fmt"

func main() {
	zero := 0.0
	inf := 1 / zero
	ninf := -1 / zero
	nan := zero / zero
	negz := -zero
	fmt.Println(inf, ninf, nan, negz, zero)
	fmt.Println(inf > 1e308, ninf < -1e308, nan == nan, nan != nan, nan < 1, nan > 1, negz == zero)
	fmt.Println(inf+inf, inf-inf, inf*0, inf*-1, -inf, 1/inf, -1/inf, 1/negz)
	fmt.Println(nan+1, nan*0, -nan, inf/inf)
	fmt.Printf("%v %v %v %v\n", inf, ninf, nan, negz)
	fmt.Printf("%f %f %f %f\n", inf, ninf, nan, negz)
	big := 1e308
	fmt.Println(big*10, -big*10, big*10-big*10)
	var f32 float32 = 3e38
	fmt.Println(f32*10, -f32*10, f32*10-f32*10, float32(zero)*-1)
	small := 1e-320
	fmt.Println(small, small/1e10, -small/1e10)
	var t32 float32 = 1e-45
	fmt.Println(t32, t32/2, t32*2)
}
