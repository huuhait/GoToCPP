// phase: 09
// Exp at the places where Go's assembly changes branch: the overflow limit, the denormal limit, +-Inf, NaN, integers, and neighbours.

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
	for _, c := range []float64{709.782712893384, 709.7827128933841, 709.78271289338397, 709.78271289338404, -745.1332191019411, -745.13321910194122, -745.1332191019413, -708.3964185322641, -708.39641853226420, 0, 1, -1, 0.5, 1e-300, -1e-300, 5e-324, 710, 711, -750, -1000, 1e10, -1e10, 1e300, -1e300, math.Inf(1), math.Inf(-1), math.NaN(), 88.72283905206835, -87.33654475055310} {
		xs = append(xs, c)
		for k := 0; k < 40; k++ {
			c = math.Nextafter(c, math.Inf(1))
			xs = append(xs, c)
		}
		for k := 0; k < 80; k++ {
			c = math.Nextafter(c, math.Inf(-1))
			xs = append(xs, c)
		}
	}
	for i := -800; i <= 800; i++ {
		xs = append(xs, float64(i), float64(i)+0.5, float64(i)*math.Ln2, float64(i)*math.Ln2+1e-13)
	}

	h := uint64(14695981039346656037)
	for i, x := range xs {
		a := math.Exp(x)
		b := math.Exp2(x)
		h = mix(h, math.Float64bits(a))
		h = mix(h, math.Float64bits(b))
		if i%400 == 0 {
			fmt.Printf("%016x %016x %016x\n", math.Float64bits(x), math.Float64bits(a), math.Float64bits(b))
		}
	}
	fmt.Printf("n=%d hash=%016x\n", len(xs), h)
}

// Output:
// 40862e42fefa39ef 7ff0000000000000 6c4b869b3585f2af
// 40862e42fefa3a15 7ff0000000000000 6c4b869b35864d4f
// c0874910d52d304d 0000000000000001 115d2d6c09dc8d5a
// 800000000000001f 3ff0000000000000 3ff0000000000000
// 01a56e1fc2f8f374 3ff0000000000000 3ff0000000000000
// 4086300000000010 7ff0000000000000 6c5000000000162e
// c08f400000000015 0000000000000000 016fffffffffc5c7
// fe37e43c8800758b 0000000000000000 0000000000000000
// 7ff8000000000001 7ff8000000000001 7ff8000000000001
// c080da25e6b91d03 0f5000000000027c 1e3a90d28fcdf122
// c07d5f42b7a55ab5 15900000000002c4 2290855319d7b6c0
// c0790a39a1d87b65 1bd000000000020b 26e48c4d0daf09a6
// c074b5308c0b9c14 2210000000000253 2b398e98d4afff0b
// c0706027763ebcc3 285000000000029a 2f8fc979f8d760d0
// c068163cc0e3bae6 2e900000000001e2 33e3c49195bee067
// c05ed8552a93f889 34d00000000001ea 3838962d1e26a93c
// c04b0861a6c0f68d 3b100000000001d1 3c8e947f9f9f0f76
// 402e7f9c1e980fe1 41500000000001c1 40e3046b90bc0cb7
// 40552417db067f3f 47900000000001c0 4537a7301e2e2d7e
// 40633c1e191cfe41 4dd00000000001c8 498d6b409e8aabcd
// 406be63044b6bce3 541000000000020f 4de24b9147457778
// 4072482138283dc2 5a500000000001d7 5236c1462525b76b
// 40769d2a4df51d13 609000000000021e 568c4d4aec543eca
// 407af23363c1fc63 66d0000000000166 5ae199bbce84a7dc
// 407f473c798edbb4 6d100000000001ae 5f35e416fea2aa17
// n=9913 hash=b318d85e9ac22cec
