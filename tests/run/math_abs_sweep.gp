// phase: 09
// math.Abs on about 100k inputs: bit-identical to Go (hash of the result bits and 200 samples).

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
		y := math.Abs(x)
		h = mix(h, math.Float64bits(y))
		if i%500 == 0 {
			fmt.Printf("%016x %016x\n", math.Float64bits(x), math.Float64bits(y))
		}
	}
	fmt.Printf("n=%d hash=%016x\n", len(xs), h)
}

// Output:
// 0000000000000000 0000000000000000
// 40862e42fefa39f0 40862e42fefa39f0
// 4008000000000000 4008000000000000
// 4040bfffffffffff 4040bfffffffffff
// 5b65923ac30ec1f4 5b65923ac30ec1f4
// c024bfdbf760976a 4024bfdbf760976a
// cc9f5dfc8a5c49d1 4c9f5dfc8a5c49d1
// 28faf9b36567288d 28faf9b36567288d
// 5c01af71ea368d0e 5c01af71ea368d0e
// 52bd26a497b45c45 52bd26a497b45c45
// 57a8503628a803dd 57a8503628a803dd
// 9146970ff4ce0971 1146970ff4ce0971
// f56a5815e599ff1f 756a5815e599ff1f
// 398243dbd8701b2c 398243dbd8701b2c
// 5ba4a48740cb04df 5ba4a48740cb04df
// da25dad4e83140c7 5a25dad4e83140c7
// 9ba45dbea8b9d076 1ba45dbea8b9d076
// f762bc1ba29bdb35 7762bc1ba29bdb35
// 662b00cc72923f60 662b00cc72923f60
// 482a3a7945c16ef0 482a3a7945c16ef0
// a8698fe6f1d1b6d7 28698fe6f1d1b6d7
// e006469c13f2521e 6006469c13f2521e
// e1dd8f8aae0c18d4 61dd8f8aae0c18d4
// e11d793466eefc2b 611d793466eefc2b
// 4dcd1ad5a02370a4 4dcd1ad5a02370a4
// 0deb45aa655f885e 0deb45aa655f885e
// f6250f700d64e16e 76250f700d64e16e
// d8536e85cdee3a05 58536e85cdee3a05
// 4e76b4f39cf95384 4e76b4f39cf95384
// f88bbc29e3ba37f8 788bbc29e3ba37f8
// 73ea5c3d1e048b48 73ea5c3d1e048b48
// 5c7b45e3f515c5bb 5c7b45e3f515c5bb
// ab2567ceba64e421 2b2567ceba64e421
// eb8d783bcea270b8 6b8d783bcea270b8
// c3b9dee0ebf001a5 43b9dee0ebf001a5
// 0e7855fba5612c1d 0e7855fba5612c1d
// 836e10793726318c 036e10793726318c
// 3b34dda55a15d732 3b34dda55a15d732
// 2f4e9ffba969cc56 2f4e9ffba969cc56
// 8b1e87ad9104ded4 0b1e87ad9104ded4
// 17c82fc123e13faa 17c82fc123e13faa
// f3be5d2a6268c34c 73be5d2a6268c34c
// 4b335c2e086fec82 4b335c2e086fec82
// 304c21caf2bd8d77 304c21caf2bd8d77
// 3fe30bf498dd7094 3fe30bf498dd7094
// 3fb5aac89bff8058 3fb5aac89bff8058
// 3feb34272e81d3a3 3feb34272e81d3a3
// 3fe816820fd48922 3fe816820fd48922
// bfd0fe16d5fa012a 3fd0fe16d5fa012a
// bfd7e00241eae292 3fd7e00241eae292
// 3fd04afd572abb6e 3fd04afd572abb6e
// bfd3fac47071ae3a 3fd3fac47071ae3a
// 3fee624f10cecad8 3fee624f10cecad8
// 3fea5ab522610f73 3fea5ab522610f73
// bfefdb1385845498 3fefdb1385845498
// bfeab98dff3b972c 3feab98dff3b972c
// 3fe33b90cc1ec878 3fe33b90cc1ec878
// bfbd77b29473af38 3fbd77b29473af38
// bfc0d52264f94548 3fc0d52264f94548
// bfaca7d38cbe0a80 3faca7d38cbe0a80
// 3fd89e3ee22ebdcc 3fd89e3ee22ebdcc
// 3fdb17eda830b6f4 3fdb17eda830b6f4
// 3fdc928ac90fa0f4 3fdc928ac90fa0f4
// bfe1262476c0cd54 3fe1262476c0cd54
// bfd891a76658a7ee 3fd891a76658a7ee
// 3fe75dada7e86de4 3fe75dada7e86de4
// 3feb0ff8154e2eb2 3feb0ff8154e2eb2
// 3fdf06d705d2c98a 3fdf06d705d2c98a
// bfe3f76f15ef5ae3 3fe3f76f15ef5ae3
// bfd111a2c1ca78b2 3fd111a2c1ca78b2
// 3fde48a32b9caa20 3fde48a32b9caa20
// 3fef97a196284ef1 3fef97a196284ef1
// 3fede2e4caecfaa7 3fede2e4caecfaa7
// bfb8f8d3e2cf5e90 3fb8f8d3e2cf5e90
// 3fb5e89f9a291078 3fb5e89f9a291078
// bfed44e8558c6b55 3fed44e8558c6b55
// bfdb8a8438eb88d8 3fdb8a8438eb88d8
// bfef3966dac6296e 3fef3966dac6296e
// 3fe177a452bbd5c2 3fe177a452bbd5c2
// 3fd5fa7497c7977e 3fd5fa7497c7977e
// bfd0f1b0f3280d4c 3fd0f1b0f3280d4c
// 3fdce23d0b00be16 3fdce23d0b00be16
// 3f9b4fb30606ac40 3f9b4fb30606ac40
// 3febc6ef604f80de 3febc6ef604f80de
// bfd1425a423a5810 3fd1425a423a5810
// 3fc446f084e7184c 3fc446f084e7184c
// 3fc1dcd9164395c0 3fc1dcd9164395c0
// 3fed04e125e49e2c 3fed04e125e49e2c
// bfd9c1bae0d1834a 3fd9c1bae0d1834a
// 3fe9c5140195eae9 3fe9c5140195eae9
// bfcbf4cde2506070 3fcbf4cde2506070
// bfccc9fdcd96446c 3fccc9fdcd96446c
// 3fe908bfc5faf522 3fe908bfc5faf522
// bfe7529c40e5d1d8 3fe7529c40e5d1d8
// 3fddc5abc9eb9c18 3fddc5abc9eb9c18
// bfcd86ee3ae16e44 3fcd86ee3ae16e44
// bfc3b6e0915c3048 3fc3b6e0915c3048
// bfe8a5ac83066a77 3fe8a5ac83066a77
// 3fdb59c5446ed18c 3fdb59c5446ed18c
// bfd090d7606e9e78 3fd090d7606e9e78
// 3fe4d49c91a53133 3fe4d49c91a53133
// 3fe8ba0c569302d0 3fe8ba0c569302d0
// 3fedd712a427ca3e 3fedd712a427ca3e
// bfecd6ff4d77b8de 3fecd6ff4d77b8de
// 418b404fc3162484 418b404fc3162484
// 41d17b6ce028e50d 41d17b6ce028e50d
// 408655173bfff5d1 408655173bfff5d1
// bfea65e0e6524d11 3fea65e0e6524d11
// be22aed75aeb9d29 3e22aed75aeb9d29
// c10cb224405f8efc 410cb224405f8efc
// 3feb027bf746a552 3feb027bf746a552
// c0609ecd76ff47a0 40609ecd76ff47a0
// c016e544370168a9 4016e544370168a9
// c0a93a015cc53ccd 40a93a015cc53ccd
// 3e87c667839667c9 3e87c667839667c9
// 3e11d98eda6dcb94 3e11d98eda6dcb94
// 416b3a4c3d7d9238 416b3a4c3d7d9238
// c1656317acd288fe 41656317acd288fe
// 3fee7cfa81808980 3fee7cfa81808980
// bea144763594e4ae 3ea144763594e4ae
// c06274676f3acee9 406274676f3acee9
// bf87a2dd3a378270 3f87a2dd3a378270
// 3f0138f32b5b8be8 3f0138f32b5b8be8
// bf83230b4ef7d484 3f83230b4ef7d484
// 3e1422076fb38833 3e1422076fb38833
// 4017a2641f288700 4017a2641f288700
// be1c28c11f1e1e4e 3e1c28c11f1e1e4e
// 40bf4cf742ff052e 40bf4cf742ff052e
// c19940c466a73f26 419940c466a73f26
// bfc84b098455eb88 3fc84b098455eb88
// 3e73ccded0704d12 3e73ccded0704d12
// c19259d0233a17c8 419259d0233a17c8
// 3e387a09cc4c64a0 3e387a09cc4c64a0
// bf31ffcdf0faf143 3f31ffcdf0faf143
// bf2dc2f4ae75ee28 3f2dc2f4ae75ee28
// 413f1b588b0dde46 413f1b588b0dde46
// c149b81eb24a22ae 4149b81eb24a22ae
// c03a3ee2751880ae 403a3ee2751880ae
// c040825333089264 4040825333089264
// c069e40733ff2e61 4069e40733ff2e61
// 3fb3a5f40e85efb6 3fb3a5f40e85efb6
// 4190e24e797072b6 4190e24e797072b6
// 4158bac92af262e6 4158bac92af262e6
// bf98a2773cb47237 3f98a2773cb47237
// c080f5aaa4e0668e 4080f5aaa4e0668e
// c0c65c914e3bbb1a 40c65c914e3bbb1a
// 3fd71f13102b33ce 3fd71f13102b33ce
// 3e6924bd2c13f2a4 3e6924bd2c13f2a4
// 4035402d0ef88402 4035402d0ef88402
// be20d335ec1dfd0e 3e20d335ec1dfd0e
// bf2724e26d190e2c 3f2724e26d190e2c
// c1a6776763ade5a0 41a6776763ade5a0
// bf73d3e6f8b4d4aa 3f73d3e6f8b4d4aa
// 41668c332abcb596 41668c332abcb596
// beec4e02cfb706e8 3eec4e02cfb706e8
// 40dd70b10b863620 40dd70b10b863620
// 3eac444c99c0d1a1 3eac444c99c0d1a1
// c178b7b6d077fabf 4178b7b6d077fabf
// bf6b00e334851430 3f6b00e334851430
// befc5f503bf32744 3efc5f503bf32744
// c021901561e5f3bb 4021901561e5f3bb
// bf9fe1961bbabc9a 3f9fe1961bbabc9a
// c0b82ff8a46d7370 40b82ff8a46d7370
// bf441813cf8fabab 3f441813cf8fabab
// 4089045a47b8c079 4089045a47b8c079
// c07bf9e98e980d9f 407bf9e98e980d9f
// c08c2be7a4bbcb87 408c2be7a4bbcb87
// 40811f081d4297a7 40811f081d4297a7
// 405566d61ad0e231 405566d61ad0e231
// c08101b9da2c269e 408101b9da2c269e
// c08a31e27a1d835e 408a31e27a1d835e
// c08aa54ac269a045 408aa54ac269a045
// 4067ce94cf8091a0 4067ce94cf8091a0
// 408ba94277171cb2 408ba94277171cb2
// 407afeb0d39dbd69 407afeb0d39dbd69
// 406dddc7ccb3faa2 406dddc7ccb3faa2
// 4038ec6f580d899a 4038ec6f580d899a
// c05c9ef205a6bc34 405c9ef205a6bc34
// c088d420c5366bbf 4088d420c5366bbf
// 40863da363414e13 40863da363414e13
// c089a72a0990087b 4089a72a0990087b
// 406a4b15d13a7908 406a4b15d13a7908
// c065f8d41fe23f29 4065f8d41fe23f29
// 40867b1e8721c6a9 40867b1e8721c6a9
// 4077c8882b2981fe 4077c8882b2981fe
// c07460d4022d36f8 407460d4022d36f8
// c088043886eef719 4088043886eef719
// c07211561b55e493 407211561b55e493
// 408171fe0170a0b0 408171fe0170a0b0
// c053422327c05870 4053422327c05870
// c0889826c6d467f5 40889826c6d467f5
// 407baeb37ff376d8 407baeb37ff376d8
// 408d71fa5ceb8ced 408d71fa5ceb8ced
// c0863526f46028ca 40863526f46028ca
// c08a73755f0a012b 408a73755f0a012b
// c06077c19b5ceab4 406077c19b5ceab4
// c08db6fd3eb3d7af 408db6fd3eb3d7af
// c084931eb03021f8 4084931eb03021f8
// c080c58c2d474550 4080c58c2d474550
// 408ba17a7d743a80 408ba17a7d743a80
// c0825a6354c0c00a 40825a6354c0c00a
// c0653ddadc66b0d6 40653ddadc66b0d6
// 40898f904c7db4f1 40898f904c7db4f1
// 408c622861ab45a0 408c622861ab45a0
// n=101592 hash=15c2337576d90a24
