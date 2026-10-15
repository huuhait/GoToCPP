// phase: 03
// diff: stderr
//go:build amd64

// Complex arithmetic (step 3.15c): + - * / and unary - on complex128 and
// complex64, with zeros of both signs, infinities and NaN; division by zero
// (no panic); == and != (a NaN part is never equal); ++, -- and op=, which add
// (1+0i), so a -0 imaginary part becomes +0; conversions between the two sizes
// (each part rounded); the builtins complex, real and imag; constants, which
// go/types folds exactly (also with float32 rounding for complex64); a named
// complex type. Go's * and / on amd64 at GOAMD64=v1, which the harness sets:
// arm64, and v3 or higher, fuse a*b + c.
package main

type Phasor complex128

type Small complex64

func main() {
	zero := 0.0
	negz := -zero
	inf := 1 / zero
	nan := zero / zero

	a, b := complex(1.5, -2), complex(3, 4)
	println(a+b, a-b, a*b, a/b, -a, +a, a == b, a != b)
	println(real(a), imag(a), real(a*b), imag(a/b))

	// Zeros: -(0+0i) is (-0-0i); (-0) + 0 is +0.
	z := complex(zero, zero)
	nz := complex(negz, negz)
	println(z, -z, nz, -nz, nz+z, nz-z, z-nz, nz*z, z*nz, nz*nz, z == nz)
	println(complex(negz, 1)*complex(1, negz), complex(1, negz)/complex(2, 0))

	// Infinities and NaN: no recovery after *, C99's corrections after /.
	println(complex(inf, 0)*2, complex(inf, 0)*complex(0, 1), complex(inf, inf)*complex(inf, inf))
	println(complex(nan, 0)*1, complex(nan, 1) == complex(nan, 1), complex(1, nan) != complex(1, nan))
	println(complex(1, 1)/complex(inf, inf), complex(inf, 1)/complex(2, 3), complex(inf, nan)/complex(1, 1))
	println(complex(1, 2)/complex(nan, 0), complex(inf, inf)/complex(inf, inf))

	// Division by zero: infinities or NaN, never a panic.
	println(a/z, a/nz, -a/z, z/z, complex(inf, 0)/z, complex(0, 1)/z, complex(nan, 1)/z)
	var c64 complex64 = 1 - 1i
	var z64 complex64
	println(c64/z64, z64/z64, -c64/z64)

	// complex64: * in float64 and rounded back, / as a complex128.
	e := float32(1.0 / 8192)
	p := complex(1+e, 1) * complex(1-e, 1)
	println(p, real(p) == 0, complex64(1+1i)/3, complex64(1)/complex(0, e))
	var big32 complex64 = complex(3e38, 3e38)
	println(big32*big32, big32+big32, big32/complex64(complex(1e-38, 0)))

	// Conversions round each part; complex128 of a complex64 is exact.
	w := complex(0.1, 1e40)
	w64 := complex64(w)
	println(w64, complex128(w64), complex64(complex(1e-46, -1e-46)), complex64(complex(nan, inf)))
	println(complex128(complex64(complex(16777217, 0.3))))

	// ++, -- and op=.
	c := complex(negz, negz)
	c++
	println(c)
	c = complex(negz, negz)
	c--
	println(c)
	c = 2 + 3i
	c += 1i
	c -= complex(0.5, 0)
	c *= c
	c /= 1 - 1i
	println(c)
	c64++
	c64 *= 2i
	c64 /= 3
	println(c64)

	// Constants: exact folding, complex64 parts rounded to float32.
	const k = (0.1 + 0.2i) * 3
	var k64 complex64 = k
	println(k, k64, real(k), imag(k64), k == 0.3+0.6i)
	const third = 1 / 3i
	println(third, complex64(third), complex(1e-45, 0)*1i, complex64(1e300*1e-300+1i))
	var untyped = 2i * 2i
	println(untyped, real(untyped) == -4, 1i*1i == -1)
	r := 0.25
	println(complex(r, 2), complex(-1, r), complex(float32(r), 1.5))

	// A named complex type.
	ph := Phasor(complex(1, 1))
	ph *= ph
	ph++
	ph = -ph / 2
	println(ph, real(ph), imag(ph), ph == -1-1i, Phasor(a)+ph)
	var sm Small = 1i
	sm--
	sm *= sm
	println(sm, real(sm), complex128(sm), Small(c64)+sm)
}
