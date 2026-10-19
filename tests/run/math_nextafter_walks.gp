// phase: 09
// Nextafter and Nextafter32 walking across zero, the subnormal boundary, powers of two and the largest finite number, in both directions.

package main

import (
	"fmt"
	"math"
)

var state uint64 = 0x9E3779B97F4A7C15

func next() uint64 { // splitmix64
	state += 0x9E3779B97F4A7C15
	z := state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

func unit() float64 { return float64(next()>>11) / (1 << 53) }

func sgn() float64 {
	if next()&1 == 0 {
		return 1
	}
	return -1
}

func mix(h, w uint64) uint64 {
	h ^= w
	h *= 1099511628211
	h ^= h >> 29
	return h
}

func b2w(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

func specials() []float64 {
	s := []float64{0, math.Copysign(0, -1), 1, -1, 0.5, -0.5, 2, -2, 3, 10, math.Inf(1), math.Inf(-1), math.NaN(),
		math.MaxFloat64, -math.MaxFloat64, math.SmallestNonzeroFloat64, -math.SmallestNonzeroFloat64,
		2.2250738585072014e-308, 2.225073858507201e-308, math.MaxFloat32, 1e-10, 1e10, 1e15, 1e300, 1e-300,
		math.Pi, math.Pi / 2, math.Pi / 4, 3 * math.Pi / 2, 2 * math.Pi, 1e6 * math.Pi, math.E, math.Ln2, math.Ln10,
		709.782712893384, 709.79, -745.13321910194122, -745.2, -708.3964185322641, 1 << 52, 1 << 53, 1 << 62, 1 << 63,
		-1 << 63, 1 << 64, 0.7071067811865476, 0.7071067811865475, 1.4142135623730951, 0.9999999999999999, 1.0000000000000002,
		171.62437695630272, 171.7, 172, 1.0e22, 4503599627370496.5, 6.5, 7.5, 0.4999999999999999, 1e-5, 8.0e-5, 9.5367431640625e-07, 0.1, 0.2, 0.3, 1.0 / 3, 2.0 / 3,
		math.Float64frombits(0x7ff0000000000001), math.Float64frombits(0x7ff8000000000000), math.Float64frombits(0xfff8000000000000),
		math.Float64frombits(0x7ff4479c6aacc07a), math.Float64frombits(0xfff8000000000001), math.Float64frombits(0x7fffffffffffffff),
		math.Float64frombits(0xffffffffffffffff), math.Float64frombits(0xfff0000000000001)}
	for i := -40; i <= 40; i++ {
		s = append(s, float64(i), float64(i)+0.5, float64(i)/3, float64(i)*math.Pi/4)
	}
	n := len(s)
	for i := 0; i < n; i++ {
		x := s[i]
		s = append(s, math.Nextafter(x, math.Inf(1)), math.Nextafter(x, math.Inf(-1)), -x)
	}
	return s
}

func inputs() []float64 {
	xs := specials()
	for i := 0; i < 20000; i++ {
		xs = append(xs, math.Float64frombits(next()))
	}
	for i := 0; i < 30000; i++ {
		xs = append(xs, sgn()*unit())
	}
	for i := 0; i < 30000; i++ {
		xs = append(xs, sgn()*(1+unit())*math.Ldexp(1, int(next()%61)-30))
	}
	for i := 0; i < 20000; i++ {
		xs = append(xs, (unit()*2-1)*1000)
	}
	return xs
}

func pairs() ([]float64, []float64) {
	sp := specials()
	var as, bs []float64
	for i := 0; i < len(sp); i += 3 {
		for j := 0; j < len(sp); j += 3 {
			as = append(as, sp[i])
			bs = append(bs, sp[j])
		}
	}
	for i := 0; i < 30000; i++ {
		as = append(as, sgn()*unit()*20)
		bs = append(bs, sgn()*unit()*20)
	}
	for i := 0; i < 30000; i++ {
		as = append(as, sgn()*(1+unit())*math.Ldexp(1, int(next()%41)-20))
		bs = append(bs, float64(int(next()%61)-30)+[]float64{0, 0.5, 0.25, 0.1}[next()%4])
	}
	for i := 0; i < 20000; i++ {
		as = append(as, math.Float64frombits(next()))
		bs = append(bs, math.Float64frombits(next()))
	}
	for i := 0; i < 20000; i++ {
		as = append(as, (unit()*2-1)*1000)
		bs = append(bs, (unit()*2-1)*1000)
	}
	return as, bs
}

func main() {
	var xs []float64
	for _, c := range []float64{0, math.Copysign(0, -1), 5e-324, 2.2250738585072014e-308, 1, -1, 2, 0.5, math.MaxFloat64, -math.MaxFloat64, math.Inf(1), math.Inf(-1), math.NaN()} {
		a, b := c, c
		for k := 0; k < 300; k++ {
			a = math.Nextafter(a, math.Inf(1))
			b = math.Nextafter(b, math.Inf(-1))
			xs = append(xs, a, b)
		}
		xs = append(xs, math.Nextafter(c, c), math.Nextafter(c, math.NaN()), math.Nextafter(math.NaN(), c))
	}

	h := uint64(14695981039346656037)
	for i, x := range xs {
		f := float32(x)
		a := math.Nextafter32(f, float32(math.Inf(1)))
		b := math.Nextafter32(f, float32(math.Inf(-1)))
		c := math.Nextafter32(f, f)
		h = mix(h, uint64(math.Float32bits(a)))
		h = mix(h, uint64(math.Float32bits(b)))
		h = mix(h, uint64(math.Float32bits(c)))
		if i%400 == 0 {
			fmt.Printf("%016x %016x %016x %016x\n", math.Float64bits(x), uint64(math.Float32bits(a)), uint64(math.Float32bits(b)), uint64(math.Float32bits(c)))
		}
	}
	fmt.Printf("n=%d hash=%016x\n", len(xs), h)
}

// Output:
// 0000000000000001 0000000000000001 0000000080000001 0000000000000000
// 00000000000000c9 0000000000000001 0000000080000001 0000000000000000
// 8000000000000063 0000000000000001 0000000080000001 0000000080000000
// 800000000000012b 0000000000000001 0000000080000001 0000000080000000
// 00000000000000c7 0000000000000001 0000000080000001 0000000000000000
// 000fffffffffffa0 0000000000000001 0000000080000001 0000000000000000
// 000ffffffffffed8 0000000000000001 0000000080000001 0000000000000000
// 3ff00000000000c3 000000003f800001 000000003f7fffff 000000003f800000
// bff000000000005d 00000000bf7fffff 00000000bf800001 00000000bf800000
// bff0000000000125 00000000bf7fffff 00000000bf800001 00000000bf800000
// 40000000000000c0 0000000040000001 000000003fffffff 0000000040000000
// 3fdfffffffffffa6 000000003f000001 000000003effffff 000000003f000000
// 3fdffffffffffede 000000003f000001 000000003effffff 000000003f000000
// 7ff0000000000000 000000007f800000 000000007f7fffff 000000007f800000
// fff0000000000000 00000000ff7fffff 00000000ff800000 00000000ff800000
// fff0000000000000 00000000ff7fffff 00000000ff800000 00000000ff800000
// 7ff0000000000000 000000007f800000 000000007f7fffff 000000007f800000
// fff0000000000000 00000000ff7fffff 00000000ff800000 00000000ff800000
// fff0000000000000 00000000ff7fffff 00000000ff800000 00000000ff800000
// 7ff8000000000001 000000007fc00000 000000007fc00000 000000007fc00000
// n=7839 hash=35f8e3d82ef0e7f9
