// phase: 09
// Integer to float conversions: round to nearest even above 2^53, uint64 above 2^63, float32 without double rounding.

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

	ints := []int64{0, 1, -1, 1 << 53, 1<<53 + 1, 1<<53 + 2, 1<<53 + 3, -(1<<53 + 1), 1<<62 + 1<<9, 1<<62 + 1<<9 + 1, 1<<62 + 3<<8, math.MaxInt64, math.MinInt64, math.MaxInt64 - 1, 16777217, 16777216, 16777219, -16777217, 1<<24 + 1, 1<<40 + 1<<16 + 1}
	for _, v := range ints {
		fmt.Printf("%d %016x %08x\n", v, math.Float64bits(float64(v)), math.Float32bits(float32(v)))
	}
	us := []uint64{0, 1<<63 - 1, 1 << 63, 1<<63 + 1, 1<<63 + 1<<10, 1<<63 + 1<<10 + 1, 1<<63 + 3<<10, math.MaxUint64, math.MaxUint64 - 1, 1<<64 - 1<<11, 1<<64 - 1<<10, 1<<64 - 1<<10 - 1, 1<<53 + 1}
	for _, v := range us {
		fmt.Printf("%d %016x %08x\n", v, math.Float64bits(float64(v)), math.Float32bits(float32(v)))
	}
	h := uint64(14695981039346656037)
	for i := 0; i < 100000; i++ {
		v := next()
		h = mix(h, math.Float64bits(float64(v)))
		h = mix(h, uint64(math.Float32bits(float32(v))))
		h = mix(h, math.Float64bits(float64(int64(v))))
		h = mix(h, uint64(math.Float32bits(float32(int64(v)))))
		w := uint32(v)
		h = mix(h, math.Float64bits(float64(w)))
		h = mix(h, uint64(math.Float32bits(float32(w))))
		h = mix(h, math.Float64bits(float64(int32(w))))
		s := int16(v)
		h = mix(h, math.Float64bits(float64(s)))
	}
	fmt.Printf("%016x\n", h)

}

// Output:
// 0 0000000000000000 00000000
// 1 3ff0000000000000 3f800000
// -1 bff0000000000000 bf800000
// 9007199254740992 4340000000000000 5a000000
// 9007199254740993 4340000000000000 5a000000
// 9007199254740994 4340000000000001 5a000000
// 9007199254740995 4340000000000002 5a000000
// -9007199254740993 c340000000000000 da000000
// 4611686018427388416 43d0000000000000 5e800000
// 4611686018427388417 43d0000000000001 5e800000
// 4611686018427388672 43d0000000000001 5e800000
// 9223372036854775807 43e0000000000000 5f000000
// -9223372036854775808 c3e0000000000000 df000000
// 9223372036854775806 43e0000000000000 5f000000
// 16777217 4170000010000000 4b800000
// 16777216 4170000000000000 4b800000
// 16777219 4170000030000000 4b800002
// -16777217 c170000010000000 cb800000
// 16777217 4170000010000000 4b800000
// 1099511693313 4270000010001000 53800001
// 0 0000000000000000 00000000
// 9223372036854775807 43e0000000000000 5f000000
// 9223372036854775808 43e0000000000000 5f000000
// 9223372036854775809 43e0000000000000 5f000000
// 9223372036854776832 43e0000000000000 5f000000
// 9223372036854776833 43e0000000000001 5f000000
// 9223372036854778880 43e0000000000002 5f000000
// 18446744073709551615 43f0000000000000 5f800000
// 18446744073709551614 43f0000000000000 5f800000
// 18446744073709549568 43efffffffffffff 5f800000
// 18446744073709550592 43f0000000000000 5f800000
// 18446744073709550591 43efffffffffffff 5f800000
// 9007199254740993 4340000000000000 5a000000
// c5447443fa22c6fe
