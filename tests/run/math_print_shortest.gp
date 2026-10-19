// phase: 09
// Println of floats: the shortest representation that round-trips, for random bit patterns, float32 and float64, as Go prints it.

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
	h := uint64(14695981039346656037)
	for i := 0; i < 60000; i++ {
		var x float64
		switch i % 4 {
		case 0:
			x = math.Float64frombits(next())
		case 1:
			x = float64(int64(next()>>20)) / 1000
		case 2:
			x = math.Sqrt(float64(i))
		default:
			x = math.Ldexp(unit(), int(next()%200)-100)
		}
		s := fmt.Sprint(x) + fmt.Sprint(float32(x)) + fmt.Sprintf("%v%g", -x, x*3)
		for j := 0; j < len(s); j++ {
			h = mix(h, uint64(s[j]))
		}
		if i%3000 == 0 {
			fmt.Println(x, float32(x), -x)
		}
	}
	fmt.Printf("%016x\n", h)
}

// Output:
// 1.4238489803937894e+224 +Inf -1.4238489803937894e+224
// 3.1142996951463786e-233 0 -3.1142996951463786e-233
// -2.8819122208956302e-24 -2.8819122e-24 2.8819122208956302e-24
// 7.762591762228172e+80 +Inf -7.762591762228172e+80
// 7.046658043849322e+142 +Inf -7.046658043849322e+142
// -3.987262092731745e+20 -3.987262e+20 3.987262092731745e+20
// -4.9627642366736706e+172 -Inf 4.9627642366736706e+172
// -4.631290669643859e+220 -Inf 4.631290669643859e+220
// 4.432232536253618e-259 0 -4.432232536253618e-259
// -1.9225976792591706e-102 -0 1.9225976792591706e-102
// 3.184118914288325e-161 0 -3.184118914288325e-161
// -1.8834784118971277e+07 -1.8834784e+07 1.8834784118971277e+07
// -1.3278984082874164e-93 -0 1.3278984082874164e-93
// 4.614076197782873e-223 0 -4.614076197782873e-223
// -6.608444414445377e+261 -Inf 6.608444414445377e+261
// -6.821037660362662e-42 -6.822e-42 6.821037660362662e-42
// -3.1285149529164724e+48 -Inf 3.1285149529164724e+48
// 1.0001399362691859e+186 +Inf -1.0001399362691859e+186
// 1.9330772465413e-101 0 -1.9330772465413e-101
// 4.606836493274651e+113 +Inf -4.606836493274651e+113
// e445bcfd457cb7df
