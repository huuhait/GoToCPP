// phase: 03
// diff: stderr
//go:build amd64

// Complex * and / on every pair of complex numbers whose parts are special
// values (zeros of both signs, ones, a huge value, infinities and NaN), like
// Go's test/cmplxdivide.go: Go's complex128div (Smith's algorithm with C99's
// corrections) and gc's multiplication without corrections, for complex128 and
// complex64 (computed in float64 and rounded back). Go's results on amd64 at
// GOAMD64=v1, which the harness sets: arm64, and v3 or higher, fuse a*b + c.
package main

func main() {
	zero := 0.0
	inf := 1 / zero
	vals := []float64{zero, -zero, 1, -1, 2, 1e300, zero / zero, inf, -inf}
	var cs []complex128
	for _, r := range vals {
		for _, i := range vals {
			cs = append(cs, complex(r, i))
		}
	}
	for i, a := range cs {
		for j, b := range cs {
			a64, b64 := complex64(a), complex64(b)
			println(i, j, a*b, a/b, a64*b64, a64/b64)
		}
	}
}
