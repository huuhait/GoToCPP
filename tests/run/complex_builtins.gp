// phase: 03

// complex, real and imag (step 3.15c): complex(r, i) builds a complex128 from
// float64 parts and a complex64 from float32 parts; real and imag give the
// parts back with their float type. Constants fold at compile time. (fmt prints
// the float parts here: printing a complex number with fmt comes after the fmt
// shim's step 3.15b.)
package main

import "fmt"

type Volt float64

func parts(c complex128) (float64, float64) {
	return real(c), imag(c)
}

func main() {
	re, im := 3.0, -4.0
	c := complex(re, im)
	fmt.Println(real(c), imag(c), real(c*c), imag(c*c))
	r, i := parts(c / (1 + 1i))
	fmt.Println(r, i)

	var f32 float32 = 0.1
	c64 := complex(f32, f32*2)
	fmt.Println(real(c64), imag(c64), real(c64) == f32, real(complex128(c64)))

	v := Volt(1.5)
	fmt.Println(real(complex(v, v)), imag(complex(v, 0.25)))

	const k = 2 + 3i
	fmt.Println(real(k), imag(k), real(k*k), imag(1/k))
	var k64 complex64 = 1.0 / 3i
	fmt.Println(real(k64), imag(k64))

	sum := 0i
	for n := 1; n <= 4; n++ {
		sum += complex(float64(n), float64(-n))
	}
	fmt.Println(real(sum), imag(sum), sum == 10-10i)
}

// Output:
// 3 -4 -7 -24
// -0.5 -3.5
// 0.1 0.2 true 0.10000000149011612
// 1.5 0.25
// 2 3 -5 -0.23076923076923078
// 0 -0.33333334
// 10 -10 true
