// phase: 09
// math.Frexp: fraction and exponent, bit-identical to Go.

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
		a, e := math.Frexp(x)
		h = mix(h, math.Float64bits(a))
		h = mix(h, uint64(e))
		if i%500 == 0 {
			fmt.Printf("%016x %016x %016x\n", math.Float64bits(x), math.Float64bits(a), uint64(e))
		}
	}
	fmt.Printf("n=%d hash=%016x\n", len(xs), h)
}

// Output:
// 0000000000000000 0000000000000000 0000000000000000
// 40862e42fefa39f0 3fe62e42fefa39f0 000000000000000a
// 4008000000000000 3fe8000000000000 0000000000000002
// 4040bfffffffffff 3fe0bfffffffffff 0000000000000006
// 5b65923ac30ec1f4 3fe5923ac30ec1f4 00000000000001b8
// c024bfdbf760976a bfe4bfdbf760976a 0000000000000004
// cc9f5dfc8a5c49d1 bfef5dfc8a5c49d1 00000000000000cb
// 28faf9b36567288d 3feaf9b36567288d fffffffffffffe91
// 5c01af71ea368d0e 3fe1af71ea368d0e 00000000000001c2
// 52bd26a497b45c45 3fed26a497b45c45 000000000000012d
// 57a8503628a803dd 3fe8503628a803dd 000000000000017c
// 9146970ff4ce0971 bfe6970ff4ce0971 fffffffffffffd16
// f56a5815e599ff1f bfea5815e599ff1f 0000000000000358
// 398243dbd8701b2c 3fe243dbd8701b2c ffffffffffffff9a
// 5ba4a48740cb04df 3fe4a48740cb04df 00000000000001bc
// da25dad4e83140c7 bfe5dad4e83140c7 00000000000001a4
// 9ba45dbea8b9d076 bfe45dbea8b9d076 fffffffffffffdbc
// f762bc1ba29bdb35 bfe2bc1ba29bdb35 0000000000000378
// 662b00cc72923f60 3feb00cc72923f60 0000000000000264
// 482a3a7945c16ef0 3fea3a7945c16ef0 0000000000000084
// a8698fe6f1d1b6d7 bfe98fe6f1d1b6d7 fffffffffffffe88
// e006469c13f2521e bfe6469c13f2521e 0000000000000202
// e1dd8f8aae0c18d4 bfed8f8aae0c18d4 000000000000021f
// e11d793466eefc2b bfed793466eefc2b 0000000000000213
// 4dcd1ad5a02370a4 3fed1ad5a02370a4 00000000000000de
// 0deb45aa655f885e 3feb45aa655f885e fffffffffffffce0
// f6250f700d64e16e bfe50f700d64e16e 0000000000000364
// d8536e85cdee3a05 bfe36e85cdee3a05 0000000000000187
// 4e76b4f39cf95384 3fe6b4f39cf95384 00000000000000e9
// f88bbc29e3ba37f8 bfebbc29e3ba37f8 000000000000038a
// 73ea5c3d1e048b48 3fea5c3d1e048b48 0000000000000340
// 5c7b45e3f515c5bb 3feb45e3f515c5bb 00000000000001c9
// ab2567ceba64e421 bfe567ceba64e421 fffffffffffffeb4
// eb8d783bcea270b8 bfed783bcea270b8 00000000000002ba
// c3b9dee0ebf001a5 bfe9dee0ebf001a5 000000000000003d
// 0e7855fba5612c1d 3fe855fba5612c1d fffffffffffffce9
// 836e10793726318c bfee10793726318c fffffffffffffc38
// 3b34dda55a15d732 3fe4dda55a15d732 ffffffffffffffb5
// 2f4e9ffba969cc56 3fee9ffba969cc56 fffffffffffffef6
// 8b1e87ad9104ded4 bfee87ad9104ded4 fffffffffffffcb3
// 17c82fc123e13faa 3fe82fc123e13faa fffffffffffffd7e
// f3be5d2a6268c34c bfee5d2a6268c34c 000000000000033d
// 4b335c2e086fec82 3fe35c2e086fec82 00000000000000b5
// 304c21caf2bd8d77 3fec21caf2bd8d77 ffffffffffffff06
// 3fe30bf498dd7094 3fe30bf498dd7094 0000000000000000
// 3fb5aac89bff8058 3fe5aac89bff8058 fffffffffffffffd
// 3feb34272e81d3a3 3feb34272e81d3a3 0000000000000000
// 3fe816820fd48922 3fe816820fd48922 0000000000000000
// bfd0fe16d5fa012a bfe0fe16d5fa012a ffffffffffffffff
// bfd7e00241eae292 bfe7e00241eae292 ffffffffffffffff
// 3fd04afd572abb6e 3fe04afd572abb6e ffffffffffffffff
// bfd3fac47071ae3a bfe3fac47071ae3a ffffffffffffffff
// 3fee624f10cecad8 3fee624f10cecad8 0000000000000000
// 3fea5ab522610f73 3fea5ab522610f73 0000000000000000
// bfefdb1385845498 bfefdb1385845498 0000000000000000
// bfeab98dff3b972c bfeab98dff3b972c 0000000000000000
// 3fe33b90cc1ec878 3fe33b90cc1ec878 0000000000000000
// bfbd77b29473af38 bfed77b29473af38 fffffffffffffffd
// bfc0d52264f94548 bfe0d52264f94548 fffffffffffffffe
// bfaca7d38cbe0a80 bfeca7d38cbe0a80 fffffffffffffffc
// 3fd89e3ee22ebdcc 3fe89e3ee22ebdcc ffffffffffffffff
// 3fdb17eda830b6f4 3feb17eda830b6f4 ffffffffffffffff
// 3fdc928ac90fa0f4 3fec928ac90fa0f4 ffffffffffffffff
// bfe1262476c0cd54 bfe1262476c0cd54 0000000000000000
// bfd891a76658a7ee bfe891a76658a7ee ffffffffffffffff
// 3fe75dada7e86de4 3fe75dada7e86de4 0000000000000000
// 3feb0ff8154e2eb2 3feb0ff8154e2eb2 0000000000000000
// 3fdf06d705d2c98a 3fef06d705d2c98a ffffffffffffffff
// bfe3f76f15ef5ae3 bfe3f76f15ef5ae3 0000000000000000
// bfd111a2c1ca78b2 bfe111a2c1ca78b2 ffffffffffffffff
// 3fde48a32b9caa20 3fee48a32b9caa20 ffffffffffffffff
// 3fef97a196284ef1 3fef97a196284ef1 0000000000000000
// 3fede2e4caecfaa7 3fede2e4caecfaa7 0000000000000000
// bfb8f8d3e2cf5e90 bfe8f8d3e2cf5e90 fffffffffffffffd
// 3fb5e89f9a291078 3fe5e89f9a291078 fffffffffffffffd
// bfed44e8558c6b55 bfed44e8558c6b55 0000000000000000
// bfdb8a8438eb88d8 bfeb8a8438eb88d8 ffffffffffffffff
// bfef3966dac6296e bfef3966dac6296e 0000000000000000
// 3fe177a452bbd5c2 3fe177a452bbd5c2 0000000000000000
// 3fd5fa7497c7977e 3fe5fa7497c7977e ffffffffffffffff
// bfd0f1b0f3280d4c bfe0f1b0f3280d4c ffffffffffffffff
// 3fdce23d0b00be16 3fece23d0b00be16 ffffffffffffffff
// 3f9b4fb30606ac40 3feb4fb30606ac40 fffffffffffffffb
// 3febc6ef604f80de 3febc6ef604f80de 0000000000000000
// bfd1425a423a5810 bfe1425a423a5810 ffffffffffffffff
// 3fc446f084e7184c 3fe446f084e7184c fffffffffffffffe
// 3fc1dcd9164395c0 3fe1dcd9164395c0 fffffffffffffffe
// 3fed04e125e49e2c 3fed04e125e49e2c 0000000000000000
// bfd9c1bae0d1834a bfe9c1bae0d1834a ffffffffffffffff
// 3fe9c5140195eae9 3fe9c5140195eae9 0000000000000000
// bfcbf4cde2506070 bfebf4cde2506070 fffffffffffffffe
// bfccc9fdcd96446c bfecc9fdcd96446c fffffffffffffffe
// 3fe908bfc5faf522 3fe908bfc5faf522 0000000000000000
// bfe7529c40e5d1d8 bfe7529c40e5d1d8 0000000000000000
// 3fddc5abc9eb9c18 3fedc5abc9eb9c18 ffffffffffffffff
// bfcd86ee3ae16e44 bfed86ee3ae16e44 fffffffffffffffe
// bfc3b6e0915c3048 bfe3b6e0915c3048 fffffffffffffffe
// bfe8a5ac83066a77 bfe8a5ac83066a77 0000000000000000
// 3fdb59c5446ed18c 3feb59c5446ed18c ffffffffffffffff
// bfd090d7606e9e78 bfe090d7606e9e78 ffffffffffffffff
// 3fe4d49c91a53133 3fe4d49c91a53133 0000000000000000
// 3fe8ba0c569302d0 3fe8ba0c569302d0 0000000000000000
// 3fedd712a427ca3e 3fedd712a427ca3e 0000000000000000
// bfecd6ff4d77b8de bfecd6ff4d77b8de 0000000000000000
// 418b404fc3162484 3feb404fc3162484 000000000000001a
// 41d17b6ce028e50d 3fe17b6ce028e50d 000000000000001f
// 408655173bfff5d1 3fe655173bfff5d1 000000000000000a
// bfea65e0e6524d11 bfea65e0e6524d11 0000000000000000
// be22aed75aeb9d29 bfe2aed75aeb9d29 ffffffffffffffe4
// c10cb224405f8efc bfecb224405f8efc 0000000000000012
// 3feb027bf746a552 3feb027bf746a552 0000000000000000
// c0609ecd76ff47a0 bfe09ecd76ff47a0 0000000000000008
// c016e544370168a9 bfe6e544370168a9 0000000000000003
// c0a93a015cc53ccd bfe93a015cc53ccd 000000000000000c
// 3e87c667839667c9 3fe7c667839667c9 ffffffffffffffea
// 3e11d98eda6dcb94 3fe1d98eda6dcb94 ffffffffffffffe3
// 416b3a4c3d7d9238 3feb3a4c3d7d9238 0000000000000018
// c1656317acd288fe bfe56317acd288fe 0000000000000018
// 3fee7cfa81808980 3fee7cfa81808980 0000000000000000
// bea144763594e4ae bfe144763594e4ae ffffffffffffffec
// c06274676f3acee9 bfe274676f3acee9 0000000000000008
// bf87a2dd3a378270 bfe7a2dd3a378270 fffffffffffffffa
// 3f0138f32b5b8be8 3fe138f32b5b8be8 fffffffffffffff2
// bf83230b4ef7d484 bfe3230b4ef7d484 fffffffffffffffa
// 3e1422076fb38833 3fe422076fb38833 ffffffffffffffe3
// 4017a2641f288700 3fe7a2641f288700 0000000000000003
// be1c28c11f1e1e4e bfec28c11f1e1e4e ffffffffffffffe3
// 40bf4cf742ff052e 3fef4cf742ff052e 000000000000000d
// c19940c466a73f26 bfe940c466a73f26 000000000000001b
// bfc84b098455eb88 bfe84b098455eb88 fffffffffffffffe
// 3e73ccded0704d12 3fe3ccded0704d12 ffffffffffffffe9
// c19259d0233a17c8 bfe259d0233a17c8 000000000000001b
// 3e387a09cc4c64a0 3fe87a09cc4c64a0 ffffffffffffffe5
// bf31ffcdf0faf143 bfe1ffcdf0faf143 fffffffffffffff5
// bf2dc2f4ae75ee28 bfedc2f4ae75ee28 fffffffffffffff4
// 413f1b588b0dde46 3fef1b588b0dde46 0000000000000015
// c149b81eb24a22ae bfe9b81eb24a22ae 0000000000000016
// c03a3ee2751880ae bfea3ee2751880ae 0000000000000005
// c040825333089264 bfe0825333089264 0000000000000006
// c069e40733ff2e61 bfe9e40733ff2e61 0000000000000008
// 3fb3a5f40e85efb6 3fe3a5f40e85efb6 fffffffffffffffd
// 4190e24e797072b6 3fe0e24e797072b6 000000000000001b
// 4158bac92af262e6 3fe8bac92af262e6 0000000000000017
// bf98a2773cb47237 bfe8a2773cb47237 fffffffffffffffb
// c080f5aaa4e0668e bfe0f5aaa4e0668e 000000000000000a
// c0c65c914e3bbb1a bfe65c914e3bbb1a 000000000000000e
// 3fd71f13102b33ce 3fe71f13102b33ce ffffffffffffffff
// 3e6924bd2c13f2a4 3fe924bd2c13f2a4 ffffffffffffffe8
// 4035402d0ef88402 3fe5402d0ef88402 0000000000000005
// be20d335ec1dfd0e bfe0d335ec1dfd0e ffffffffffffffe4
// bf2724e26d190e2c bfe724e26d190e2c fffffffffffffff4
// c1a6776763ade5a0 bfe6776763ade5a0 000000000000001c
// bf73d3e6f8b4d4aa bfe3d3e6f8b4d4aa fffffffffffffff9
// 41668c332abcb596 3fe68c332abcb596 0000000000000018
// beec4e02cfb706e8 bfec4e02cfb706e8 fffffffffffffff0
// 40dd70b10b863620 3fed70b10b863620 000000000000000f
// 3eac444c99c0d1a1 3fec444c99c0d1a1 ffffffffffffffec
// c178b7b6d077fabf bfe8b7b6d077fabf 0000000000000019
// bf6b00e334851430 bfeb00e334851430 fffffffffffffff8
// befc5f503bf32744 bfec5f503bf32744 fffffffffffffff1
// c021901561e5f3bb bfe1901561e5f3bb 0000000000000004
// bf9fe1961bbabc9a bfefe1961bbabc9a fffffffffffffffb
// c0b82ff8a46d7370 bfe82ff8a46d7370 000000000000000d
// bf441813cf8fabab bfe41813cf8fabab fffffffffffffff6
// 4089045a47b8c079 3fe9045a47b8c079 000000000000000a
// c07bf9e98e980d9f bfebf9e98e980d9f 0000000000000009
// c08c2be7a4bbcb87 bfec2be7a4bbcb87 000000000000000a
// 40811f081d4297a7 3fe11f081d4297a7 000000000000000a
// 405566d61ad0e231 3fe566d61ad0e231 0000000000000007
// c08101b9da2c269e bfe101b9da2c269e 000000000000000a
// c08a31e27a1d835e bfea31e27a1d835e 000000000000000a
// c08aa54ac269a045 bfeaa54ac269a045 000000000000000a
// 4067ce94cf8091a0 3fe7ce94cf8091a0 0000000000000008
// 408ba94277171cb2 3feba94277171cb2 000000000000000a
// 407afeb0d39dbd69 3feafeb0d39dbd69 0000000000000009
// 406dddc7ccb3faa2 3fedddc7ccb3faa2 0000000000000008
// 4038ec6f580d899a 3fe8ec6f580d899a 0000000000000005
// c05c9ef205a6bc34 bfec9ef205a6bc34 0000000000000007
// c088d420c5366bbf bfe8d420c5366bbf 000000000000000a
// 40863da363414e13 3fe63da363414e13 000000000000000a
// c089a72a0990087b bfe9a72a0990087b 000000000000000a
// 406a4b15d13a7908 3fea4b15d13a7908 0000000000000008
// c065f8d41fe23f29 bfe5f8d41fe23f29 0000000000000008
// 40867b1e8721c6a9 3fe67b1e8721c6a9 000000000000000a
// 4077c8882b2981fe 3fe7c8882b2981fe 0000000000000009
// c07460d4022d36f8 bfe460d4022d36f8 0000000000000009
// c088043886eef719 bfe8043886eef719 000000000000000a
// c07211561b55e493 bfe211561b55e493 0000000000000009
// 408171fe0170a0b0 3fe171fe0170a0b0 000000000000000a
// c053422327c05870 bfe3422327c05870 0000000000000007
// c0889826c6d467f5 bfe89826c6d467f5 000000000000000a
// 407baeb37ff376d8 3febaeb37ff376d8 0000000000000009
// 408d71fa5ceb8ced 3fed71fa5ceb8ced 000000000000000a
// c0863526f46028ca bfe63526f46028ca 000000000000000a
// c08a73755f0a012b bfea73755f0a012b 000000000000000a
// c06077c19b5ceab4 bfe077c19b5ceab4 0000000000000008
// c08db6fd3eb3d7af bfedb6fd3eb3d7af 000000000000000a
// c084931eb03021f8 bfe4931eb03021f8 000000000000000a
// c080c58c2d474550 bfe0c58c2d474550 000000000000000a
// 408ba17a7d743a80 3feba17a7d743a80 000000000000000a
// c0825a6354c0c00a bfe25a6354c0c00a 000000000000000a
// c0653ddadc66b0d6 bfe53ddadc66b0d6 0000000000000008
// 40898f904c7db4f1 3fe98f904c7db4f1 000000000000000a
// 408c622861ab45a0 3fec622861ab45a0 000000000000000a
// n=101592 hash=5a6f0cbf01fb8f04
