// phase: 02

// float64 and float32 arithmetic printed with fmt: sums that don't round
// well, mixing with integers by conversion, comparisons and a series.
package main

import "fmt"

func harmonic(n int) float64 {
	s := 0.0
	for i := 1; i <= n; i++ {
		s += 1 / float64(i)
	}
	return s
}

func main() {
	fmt.Println(0.1+0.2, 0.1+0.2 == 0.3, 1.0/3.0)
	a, b := 0.1, 0.2
	fmt.Println(a+b, a+b == 0.3, a*b)
	var f32 float32 = 0.1
	fmt.Println(f32+0.2, f32*3, float64(f32))
	fmt.Println(harmonic(10), harmonic(1000))
	x := 7
	fmt.Println(float64(x)/2, x/2, float64(x/2))
	c := 100.0
	f := c*9/5 + 32
	fmt.Println(f, (f-32)*5/9)
	fmt.Println(1e6, 1e-4, 1e-5, 123456.0, 1234567.0, 100000000.0)
	fmt.Printf("%f %f %v\n", 3.0, 2.5e-7, 2.5e-7)
	pi := 0.0
	sign := 1.0
	for k := range 100000 {
		pi += sign * 4 / float64(2*k+1)
		sign = -sign
	}
	fmt.Println(pi > 3.1415 && pi < 3.1417)
	big := 1e300
	fmt.Println(big*10, big*1e10 > big)
	var tiny float32 = 1e-30
	fmt.Println(tiny*tiny, tiny/1e10)
}

// Output:
// 0.3 true 0.3333333333333333
// 0.30000000000000004 false 0.020000000000000004
// 0.3 0.3 0.10000000149011612
// 2.9289682539682538 7.485470860550343
// 3.5 3 3
// 212 100
// 1e+06 0.0001 1e-05 123456 1.234567e+06 1e+08
// 3.000000 0.000000 2.5e-07
// true
// 1e+301 true
// 0 1e-40
