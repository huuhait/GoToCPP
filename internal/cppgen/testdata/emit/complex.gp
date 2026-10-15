// goarch: amd64
// Complex numbers (step 3.15c): gp::complex64 and gp::complex128 with Go's operators; the
// builtins complex, real and imag; constants with their parts (float32 parts for complex64,
// rounded); conversions; a named complex type; complex numbers in arrays, slices, maps (also
// as keys), struct fields and package variables; ++ and op=. Go's * and / on amd64 at
// GOAMD64=v1 (arm64, and v3 or higher, fuse a*b + c, which rounds differently): (Inf+0i)*2 is
// (+Inf+NaNi), and a division by zero gives infinities.
package main

import "fmt"

type Phasor complex128

type Pair struct {
	a complex64
	b complex128
}

var unit = 1i
var roots = [2]complex64{1, -1}
var third complex64 = 1.0 / 3

func scale(c complex128, f float64) complex128 {
	return c * complex(f, 0)
}

func main() {
	a, b := 1.5-2i, complex(3, 4)
	println(a, b, a+b, a-b, a*b, a/b, -a, +a)
	println(real(a), imag(b), a == b, a != b, real(1+2i), imag(3i))
	var z complex128
	println(z, -z, a/z, z/z, unit*unit, third)
	f := 0.5
	c := complex(f, -f)
	c64 := complex64(c)
	println(c, c64, complex128(c64)+1, real(c64), complex(float32(f), 2))
	c64++
	c *= c
	c -= 2i
	c /= 0.5
	println(c64, c, scale(c, 2))
	inf := 1 / f / 0
	println(complex(inf, 0)*2, complex(1, 1)/complex(inf, inf), complex(inf, -f)/0.5)
	var p Phasor = 1 + 1i
	p *= p
	p++
	println(p, real(p), Phasor(a)+p, p == 1+2i)
	m := map[complex128]int{1i: 1, 2: 2}
	m[1i] += 10
	m[complex(0, f-f)] = 3
	m[complex(0, -(f - f))] = 4
	nan := inf - inf
	m[complex(nan, 0)] = 5
	m[complex(nan, 0)] = 6
	println(len(m), m[0], m[1i], m[complex(nan, 0)])
	s := []complex64{roots[0], roots[1], third}
	s[2] += 1i
	pair := Pair{1, 2i}
	pair.b = pair.b / 2
	println(s[2], pair.a, pair.b, pair == Pair{1, 1i})
	arr := [3]complex128{1, unit, a}
	println(arr[1], arr == [3]complex128{1, 1i, a})
	fmt.Println(a, c64, p, pair, m[0])
	fmt.Printf("%v %.2f %d\n", a, c64, b)
}
