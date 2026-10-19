// phase: 09
// math.Ilogb, bit-identical to Go.

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
	h := uint64(14695981039346656037)
	for i, x := range xs {
		a := math.Ilogb(x)
		h = mix(h, uint64(a))
		if i%500 == 0 {
			fmt.Printf("%016x %016x\n", math.Float64bits(x), uint64(a))
		}
	}
	fmt.Printf("n=%d hash=%016x\n", len(xs), h)
}

// Output:
// 0000000000000000 ffffffff80000000
// 40862e42fefa39f0 0000000000000009
// 4008000000000000 0000000000000001
// 4040bfffffffffff 0000000000000005
// 5b65923ac30ec1f4 00000000000001b7
// c024bfdbf760976a 0000000000000003
// cc9f5dfc8a5c49d1 00000000000000ca
// 28faf9b36567288d fffffffffffffe90
// 5c01af71ea368d0e 00000000000001c1
// 52bd26a497b45c45 000000000000012c
// 57a8503628a803dd 000000000000017b
// 9146970ff4ce0971 fffffffffffffd15
// f56a5815e599ff1f 0000000000000357
// 398243dbd8701b2c ffffffffffffff99
// 5ba4a48740cb04df 00000000000001bb
// da25dad4e83140c7 00000000000001a3
// 9ba45dbea8b9d076 fffffffffffffdbb
// f762bc1ba29bdb35 0000000000000377
// 662b00cc72923f60 0000000000000263
// 482a3a7945c16ef0 0000000000000083
// a8698fe6f1d1b6d7 fffffffffffffe87
// e006469c13f2521e 0000000000000201
// e1dd8f8aae0c18d4 000000000000021e
// e11d793466eefc2b 0000000000000212
// 4dcd1ad5a02370a4 00000000000000dd
// 0deb45aa655f885e fffffffffffffcdf
// f6250f700d64e16e 0000000000000363
// d8536e85cdee3a05 0000000000000186
// 4e76b4f39cf95384 00000000000000e8
// f88bbc29e3ba37f8 0000000000000389
// 73ea5c3d1e048b48 000000000000033f
// 5c7b45e3f515c5bb 00000000000001c8
// ab2567ceba64e421 fffffffffffffeb3
// eb8d783bcea270b8 00000000000002b9
// c3b9dee0ebf001a5 000000000000003c
// 0e7855fba5612c1d fffffffffffffce8
// 836e10793726318c fffffffffffffc37
// 3b34dda55a15d732 ffffffffffffffb4
// 2f4e9ffba969cc56 fffffffffffffef5
// 8b1e87ad9104ded4 fffffffffffffcb2
// 17c82fc123e13faa fffffffffffffd7d
// f3be5d2a6268c34c 000000000000033c
// 4b335c2e086fec82 00000000000000b4
// 304c21caf2bd8d77 ffffffffffffff05
// 3fe30bf498dd7094 ffffffffffffffff
// 3fb5aac89bff8058 fffffffffffffffc
// 3feb34272e81d3a3 ffffffffffffffff
// 3fe816820fd48922 ffffffffffffffff
// bfd0fe16d5fa012a fffffffffffffffe
// bfd7e00241eae292 fffffffffffffffe
// 3fd04afd572abb6e fffffffffffffffe
// bfd3fac47071ae3a fffffffffffffffe
// 3fee624f10cecad8 ffffffffffffffff
// 3fea5ab522610f73 ffffffffffffffff
// bfefdb1385845498 ffffffffffffffff
// bfeab98dff3b972c ffffffffffffffff
// 3fe33b90cc1ec878 ffffffffffffffff
// bfbd77b29473af38 fffffffffffffffc
// bfc0d52264f94548 fffffffffffffffd
// bfaca7d38cbe0a80 fffffffffffffffb
// 3fd89e3ee22ebdcc fffffffffffffffe
// 3fdb17eda830b6f4 fffffffffffffffe
// 3fdc928ac90fa0f4 fffffffffffffffe
// bfe1262476c0cd54 ffffffffffffffff
// bfd891a76658a7ee fffffffffffffffe
// 3fe75dada7e86de4 ffffffffffffffff
// 3feb0ff8154e2eb2 ffffffffffffffff
// 3fdf06d705d2c98a fffffffffffffffe
// bfe3f76f15ef5ae3 ffffffffffffffff
// bfd111a2c1ca78b2 fffffffffffffffe
// 3fde48a32b9caa20 fffffffffffffffe
// 3fef97a196284ef1 ffffffffffffffff
// 3fede2e4caecfaa7 ffffffffffffffff
// bfb8f8d3e2cf5e90 fffffffffffffffc
// 3fb5e89f9a291078 fffffffffffffffc
// bfed44e8558c6b55 ffffffffffffffff
// bfdb8a8438eb88d8 fffffffffffffffe
// bfef3966dac6296e ffffffffffffffff
// 3fe177a452bbd5c2 ffffffffffffffff
// 3fd5fa7497c7977e fffffffffffffffe
// bfd0f1b0f3280d4c fffffffffffffffe
// 3fdce23d0b00be16 fffffffffffffffe
// 3f9b4fb30606ac40 fffffffffffffffa
// 3febc6ef604f80de ffffffffffffffff
// bfd1425a423a5810 fffffffffffffffe
// 3fc446f084e7184c fffffffffffffffd
// 3fc1dcd9164395c0 fffffffffffffffd
// 3fed04e125e49e2c ffffffffffffffff
// bfd9c1bae0d1834a fffffffffffffffe
// 3fe9c5140195eae9 ffffffffffffffff
// bfcbf4cde2506070 fffffffffffffffd
// bfccc9fdcd96446c fffffffffffffffd
// 3fe908bfc5faf522 ffffffffffffffff
// bfe7529c40e5d1d8 ffffffffffffffff
// 3fddc5abc9eb9c18 fffffffffffffffe
// bfcd86ee3ae16e44 fffffffffffffffd
// bfc3b6e0915c3048 fffffffffffffffd
// bfe8a5ac83066a77 ffffffffffffffff
// 3fdb59c5446ed18c fffffffffffffffe
// bfd090d7606e9e78 fffffffffffffffe
// 3fe4d49c91a53133 ffffffffffffffff
// 3fe8ba0c569302d0 ffffffffffffffff
// 3fedd712a427ca3e ffffffffffffffff
// bfecd6ff4d77b8de ffffffffffffffff
// 418b404fc3162484 0000000000000019
// 41d17b6ce028e50d 000000000000001e
// 408655173bfff5d1 0000000000000009
// bfea65e0e6524d11 ffffffffffffffff
// be22aed75aeb9d29 ffffffffffffffe3
// c10cb224405f8efc 0000000000000011
// 3feb027bf746a552 ffffffffffffffff
// c0609ecd76ff47a0 0000000000000007
// c016e544370168a9 0000000000000002
// c0a93a015cc53ccd 000000000000000b
// 3e87c667839667c9 ffffffffffffffe9
// 3e11d98eda6dcb94 ffffffffffffffe2
// 416b3a4c3d7d9238 0000000000000017
// c1656317acd288fe 0000000000000017
// 3fee7cfa81808980 ffffffffffffffff
// bea144763594e4ae ffffffffffffffeb
// c06274676f3acee9 0000000000000007
// bf87a2dd3a378270 fffffffffffffff9
// 3f0138f32b5b8be8 fffffffffffffff1
// bf83230b4ef7d484 fffffffffffffff9
// 3e1422076fb38833 ffffffffffffffe2
// 4017a2641f288700 0000000000000002
// be1c28c11f1e1e4e ffffffffffffffe2
// 40bf4cf742ff052e 000000000000000c
// c19940c466a73f26 000000000000001a
// bfc84b098455eb88 fffffffffffffffd
// 3e73ccded0704d12 ffffffffffffffe8
// c19259d0233a17c8 000000000000001a
// 3e387a09cc4c64a0 ffffffffffffffe4
// bf31ffcdf0faf143 fffffffffffffff4
// bf2dc2f4ae75ee28 fffffffffffffff3
// 413f1b588b0dde46 0000000000000014
// c149b81eb24a22ae 0000000000000015
// c03a3ee2751880ae 0000000000000004
// c040825333089264 0000000000000005
// c069e40733ff2e61 0000000000000007
// 3fb3a5f40e85efb6 fffffffffffffffc
// 4190e24e797072b6 000000000000001a
// 4158bac92af262e6 0000000000000016
// bf98a2773cb47237 fffffffffffffffa
// c080f5aaa4e0668e 0000000000000009
// c0c65c914e3bbb1a 000000000000000d
// 3fd71f13102b33ce fffffffffffffffe
// 3e6924bd2c13f2a4 ffffffffffffffe7
// 4035402d0ef88402 0000000000000004
// be20d335ec1dfd0e ffffffffffffffe3
// bf2724e26d190e2c fffffffffffffff3
// c1a6776763ade5a0 000000000000001b
// bf73d3e6f8b4d4aa fffffffffffffff8
// 41668c332abcb596 0000000000000017
// beec4e02cfb706e8 ffffffffffffffef
// 40dd70b10b863620 000000000000000e
// 3eac444c99c0d1a1 ffffffffffffffeb
// c178b7b6d077fabf 0000000000000018
// bf6b00e334851430 fffffffffffffff7
// befc5f503bf32744 fffffffffffffff0
// c021901561e5f3bb 0000000000000003
// bf9fe1961bbabc9a fffffffffffffffa
// c0b82ff8a46d7370 000000000000000c
// bf441813cf8fabab fffffffffffffff5
// 4089045a47b8c079 0000000000000009
// c07bf9e98e980d9f 0000000000000008
// c08c2be7a4bbcb87 0000000000000009
// 40811f081d4297a7 0000000000000009
// 405566d61ad0e231 0000000000000006
// c08101b9da2c269e 0000000000000009
// c08a31e27a1d835e 0000000000000009
// c08aa54ac269a045 0000000000000009
// 4067ce94cf8091a0 0000000000000007
// 408ba94277171cb2 0000000000000009
// 407afeb0d39dbd69 0000000000000008
// 406dddc7ccb3faa2 0000000000000007
// 4038ec6f580d899a 0000000000000004
// c05c9ef205a6bc34 0000000000000006
// c088d420c5366bbf 0000000000000009
// 40863da363414e13 0000000000000009
// c089a72a0990087b 0000000000000009
// 406a4b15d13a7908 0000000000000007
// c065f8d41fe23f29 0000000000000007
// 40867b1e8721c6a9 0000000000000009
// 4077c8882b2981fe 0000000000000008
// c07460d4022d36f8 0000000000000008
// c088043886eef719 0000000000000009
// c07211561b55e493 0000000000000008
// 408171fe0170a0b0 0000000000000009
// c053422327c05870 0000000000000006
// c0889826c6d467f5 0000000000000009
// 407baeb37ff376d8 0000000000000008
// 408d71fa5ceb8ced 0000000000000009
// c0863526f46028ca 0000000000000009
// c08a73755f0a012b 0000000000000009
// c06077c19b5ceab4 0000000000000007
// c08db6fd3eb3d7af 0000000000000009
// c084931eb03021f8 0000000000000009
// c080c58c2d474550 0000000000000009
// 408ba17a7d743a80 0000000000000009
// c0825a6354c0c00a 0000000000000009
// c0653ddadc66b0d6 0000000000000007
// 40898f904c7db4f1 0000000000000009
// 408c622861ab45a0 0000000000000009
// n=101592 hash=66d41be9e34e572c
