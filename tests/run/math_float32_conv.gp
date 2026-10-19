// phase: 09
// float32(float64) and float64(float32): rounding to nearest even, subnormals, overflow to Inf, NaN payloads.

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
	var h uint64 = 14695981039346656037
	for _, x := range xs {
		f := float32(x)
		h = mix(h, uint64(math.Float32bits(f)))
		h = mix(h, math.Float64bits(float64(f)))
	}
	fmt.Printf("%016x\n", h)
	for _, x := range []float64{3.4028235677973366e38, 3.4028234663852886e38, 3.4028235677973362e38, 1.401298464324817e-45, 7.006492321624085e-46, 7.006492321624086e-46, 1e-46, 1.1754943508222875e-38, 1.1754942106924411e-38,
		math.Float64frombits(0x7ff0000000000001), math.Float64frombits(0xfff8000000000001), math.Float64frombits(0x7ffffffffffffff0)} {
		f := float32(x)
		fmt.Printf("%016x %08x %016x\n", math.Float64bits(x), math.Float32bits(f), math.Float64bits(float64(f)))
	}
	for _, b := range []uint32{0x7f800001, 0x7fc00000, 0xffc00001, 0x7fffffff, 0x00000001, 0x007fffff, 0x80000001} {
		f := math.Float32frombits(b)
		fmt.Printf("%08x %016x %08x\n", b, math.Float64bits(float64(f)), math.Float32bits(f*1))
	}

}

// Output:
// 5c95a4a4416a4374
// 47effffff0000000 7f800000 7ff0000000000000
// 47efffffe0000000 7f7fffff 47efffffe0000000
// 47efffffefffffff 7f7fffff 47efffffe0000000
// 36a0000000000000 00000001 36a0000000000000
// 3690000000000000 00000000 0000000000000000
// 3690000000000000 00000000 0000000000000000
// 366244ce242c5561 00000000 0000000000000000
// 3810000000000000 00800000 3810000000000000
// 380fffffc0000000 007fffff 380fffffc0000000
// 7ff0000000000001 7fc00000 7ff8000000000000
// fff8000000000001 ffc00000 fff8000000000000
// 7ffffffffffffff0 7fffffff 7fffffffe0000000
// 7f800001 7ff8000020000000 7f800001
// 7fc00000 7ff8000000000000 7fc00000
// ffc00001 fff8000020000000 ffc00001
// 7fffffff 7fffffffe0000000 7fffffff
// 00000001 36a0000000000000 00000001
// 007fffff 380fffffc0000000 007fffff
// 80000001 b6a0000000000000 80000001
