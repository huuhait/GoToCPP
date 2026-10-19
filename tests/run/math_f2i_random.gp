// phase: 09
// float64 to integer conversion on random values of all magnitudes (including out of range): hash identical to Go's.

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

	xs := inputs()
	var h [10]uint64
	for i := range h {
		h[i] = 14695981039346656037
	}
	for _, x := range xs {
		h[0] = mix(h[0], uint64(int64(x)))
		h[1] = mix(h[1], uint64(int32(x)))
		h[2] = mix(h[2], uint64(int16(x)))
		h[3] = mix(h[3], uint64(int8(x)))
		h[4] = mix(h[4], uint64(x))
		h[5] = mix(h[5], uint64(uint32(x)))
		h[6] = mix(h[6], uint64(uint16(x)))
		h[7] = mix(h[7], uint64(uint8(x)))
		h[8] = mix(h[8], uint64(int(x)))
		h[9] = mix(h[9], uint64(uint(x)))
	}
	for i, v := range h {
		fmt.Printf("%d %016x\n", i, v)
	}
	var f32 [4]uint64
	for i := range f32 {
		f32[i] = 14695981039346656037
	}
	for _, x := range xs {
		f := float32(x)
		f32[0] = mix(f32[0], uint64(int64(f)))
		f32[1] = mix(f32[1], uint64(int32(f)))
		f32[2] = mix(f32[2], uint64(uint64(f)))
		f32[3] = mix(f32[3], uint64(uint32(f)))
	}
	fmt.Printf("%016x %016x %016x %016x\n", f32[0], f32[1], f32[2], f32[3])

}

// Output:
// 0 90182fd39b60a285
// 1 71fcfe9eae39dea6
// 2 e139ae84cff3b491
// 3 57e0f823118fecc0
// 4 e0bd5fba1d9485ce
// 5 f8e385cee80b8721
// 6 f062e104d86b829a
// 7 799836d73b6b6c62
// 8 90182fd39b60a285
// 9 e0bd5fba1d9485ce
// 6de21a6faaaf546a b6af98b1b1e65bf8 56296bcf211453b5 9b61fed90a3137e8
