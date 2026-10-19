// phase: 09
// math.Exp2 on about 100k inputs: bit-identical to Go (hash of the result bits and 200 samples).

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
		y := math.Exp2(x)
		h = mix(h, math.Float64bits(y))
		if i%500 == 0 {
			fmt.Printf("%016x %016x\n", math.Float64bits(x), math.Float64bits(y))
		}
	}
	fmt.Printf("n=%d hash=%016x\n", len(xs), h)
}

// Output:
// 0000000000000000 3ff0000000000000
// 40862e42fefa39f0 6c4b869b3585f511
// 4008000000000000 4020000000000000
// 4040bfffffffffff 4206a09e667f3bad
// 5b65923ac30ec1f4 7ff0000000000000
// c024bfdbf760976a 3f48ae19711cf21f
// cc9f5dfc8a5c49d1 0000000000000000
// 28faf9b36567288d 3ff0000000000000
// 5c01af71ea368d0e 7ff0000000000000
// 52bd26a497b45c45 7ff0000000000000
// 57a8503628a803dd 7ff0000000000000
// 9146970ff4ce0971 3ff0000000000000
// f56a5815e599ff1f 0000000000000000
// 398243dbd8701b2c 3ff0000000000000
// 5ba4a48740cb04df 7ff0000000000000
// da25dad4e83140c7 0000000000000000
// 9ba45dbea8b9d076 3ff0000000000000
// f762bc1ba29bdb35 0000000000000000
// 662b00cc72923f60 7ff0000000000000
// 482a3a7945c16ef0 7ff0000000000000
// a8698fe6f1d1b6d7 3ff0000000000000
// e006469c13f2521e 0000000000000000
// e1dd8f8aae0c18d4 0000000000000000
// e11d793466eefc2b 0000000000000000
// 4dcd1ad5a02370a4 7ff0000000000000
// 0deb45aa655f885e 3ff0000000000000
// f6250f700d64e16e 0000000000000000
// d8536e85cdee3a05 0000000000000000
// 4e76b4f39cf95384 7ff0000000000000
// f88bbc29e3ba37f8 0000000000000000
// 73ea5c3d1e048b48 7ff0000000000000
// 5c7b45e3f515c5bb 7ff0000000000000
// ab2567ceba64e421 3ff0000000000000
// eb8d783bcea270b8 0000000000000000
// c3b9dee0ebf001a5 0000000000000000
// 0e7855fba5612c1d 3ff0000000000000
// 836e10793726318c 3ff0000000000000
// 3b34dda55a15d732 3ff0000000000000
// 2f4e9ffba969cc56 3ff0000000000000
// 8b1e87ad9104ded4 3ff0000000000000
// 17c82fc123e13faa 3ff0000000000000
// f3be5d2a6268c34c 0000000000000000
// 4b335c2e086fec82 7ff0000000000000
// 304c21caf2bd8d77 3ff0000000000000
// 3fe30bf498dd7094 3ff82bcb367fe514
// 3fb5aac89bff8058 3ff0f77c0da055f5
// 3feb34272e81d3a3 3ffcd7a3a1a7e4e3
// 3fe816820fd48922 3ffaf5c14d9efc5e
// bfd0fe16d5fa012a 3fea9ef85cbea08d
// bfd7e00241eae292 3fe8b57367f619d6
// 3fd04afd572abb6e 3ff3167864799236
// bfd3fac47071ae3a 3fe9c6076453e851
// 3fee624f10cecad8 3ffee6363284e4d4
// 3fea5ab522610f73 3ffc5108440f5cc6
// bfefdb1385845498 3fe00cd11a27560c
// bfeab98dff3b972c 3fe1efd107e8f46b
// 3fe33b90cc1ec878 3ff844c568d3f944
// bfbd77b29473af38 3fed8bc88454943e
// bfc0d52264f94548 3fed364e287a00ee
// bfaca7d38cbe0a80 3feec848f3bc8ba9
// 3fd89e3ee22ebdcc 3ff4e38930b4ff12
// 3fdb17eda830b6f4 3ff574d5fc17b56d
// 3fdc928ac90fa0f4 3ff5cd86fa7baf0e
// bfe1262476c0cd54 3fe6123af0d6c7a2
// bfd891a76658a7ee 3fe8861708f79e1e
// 3fe75dada7e86de4 3ffa8aa898bd55a8
// 3feb0ff8154e2eb2 3ffcc1115d4a8ab8
// 3fdf06d705d2c98a 3ff663e10fcc01d2
// bfe3f76f15ef5ae3 3fe4c3b4cbd49e97
// bfd111a2c1ca78b2 3fea99563ee3cf10
// 3fde48a32b9caa20 3ff635f0eaa6a81f
// 3fef97a196284ef1 3fffb7f9bd85def6
// 3fede2e4caecfaa7 3ffe9163e832f55e
// bfb8f8d3e2cf5e90 3fede86a1253fb6b
// 3fb5e89f9a291078 3ff0fa538e0e6eb5
// bfed44e8558c6b55 3fe0f998a59322c3
// bfdb8a8438eb88d8 3fe7bf3870618663
// bfef3966dac6296e 3fe045691348f20d
// 3fe177a452bbd5c2 3ff75bb05e4ba43c
// 3fd5fa7497c7977e 3ff44cd045878476
// bfd0f1b0f3280d4c 3feaa28baf9cbbb4
// 3fdce23d0b00be16 3ff5e060c6cdd32e
// 3f9b4fb30606ac40 3ff04c6d672a367a
// 3febc6ef604f80de 3ffd33e9ba9a04dd
// bfd1425a423a5810 3fea8b512d9d649a
// 3fc446f084e7184c 3ff1db61fc12e17a
// 3fc1dcd9164395c0 3ff1a000b7261a9a
// 3fed04e125e49e2c 3ffdffc2bc8a5146
// bfd9c1bae0d1834a 3fe835d7fd501d80
// 3fe9c5140195eae9 3ffbf5d587b35e96
// bfcbf4cde2506070 3feb8121c8d27ad5
// bfccc9fdcd96446c 3feb61736c5fe50a
// 3fe908bfc5faf522 3ffb84ad83d274ef
// bfe7529c40e5d1d8 3fe34efb350e0571
// 3fddc5abc9eb9c18 3ff6168628fc3cf5
// bfcd86ee3ae16e44 3feb457e0bcaa3e6
// bfc3b6e0915c3048 3fecc281fb3a9094
// bfe8a5ac83066a77 3fe2c32fc44154f6
// 3fdb59c5446ed18c 3ff5842861f04874
// bfd090d7606e9e78 3feabe8a813f1b28
// 3fe4d49c91a53133 3ff91f900226fad3
// 3fe8ba0c569302d0 3ffb55ec25a242cf
// 3fedd712a427ca3e 3ffe8991360c307e
// bfecd6ff4d77b8de 3fe12232ae76b96c
// 418b404fc3162484 7ff0000000000000
// 41d17b6ce028e50d 7ff0000000000000
// 408655173bfff5d1 6c98dec3f6d9fb56
// bfea65e0e6524d11 3fe210713727ca42
// be22aed75aeb9d29 3fefffffff30cc98
// c10cb224405f8efc 0000000000000000
// 3feb027bf746a552 3ffcb8ac7981282b
// c0609ecd76ff47a0 37a06ba06fb1ee32
// c016e544370168a9 3f935ff0a7455b51
// c0a93a015cc53ccd 0000000000000000
// 3e87c667839667c9 3ff0000020f58c6e
// 3e11d98eda6dcb94 3ff0000000317d81
// 416b3a4c3d7d9238 7ff0000000000000
// c1656317acd288fe 0000000000000000
// 3fee7cfa81808980 3ffef814fe087a11
// bea144763594e4ae 3fefffff407f84e4
// c06274676f3acee9 36b49191a92ffcad
// bf87a2dd3a378270 3fefbeba5e04f5bd
// 3f0138f32b5b8be8 3ff00017e02c046b
// bf83230b4ef7d484 3fefcb1cdbc730d4
// 3e1422076fb38833 3ff000000037d201
// 4017a2641f288700 404e09068acf33bb
// be1c28c11f1e1e4e 3fefffffff63da2e
// 40bf4cf742ff052e 7ff0000000000000
// c19940c466a73f26 0000000000000000
// bfc84b098455eb88 3fec0e331de77fd6
// 3e73ccded0704d12 3ff000000db9792a
// c19259d0233a17c8 0000000000000000
// 3e387a09cc4c64a0 3ff00000010f7497
// bf31ffcdf0faf143 3feffe70cd5ab530
// bf2dc2f4ae75ee28 3feffeb5f5b9b923
// 413f1b588b0dde46 7ff0000000000000
// c149b81eb24a22ae 0000000000000000
// c03a3ee2751880ae 3e4afd757f8e9354
// c040825333089264 3ddf9981a23ac619
// c069e40733ff2e61 32fd5385242f2f86
// 3fb3a5f40e85efb6 3ff0dfce7b250791
// 4190e24e797072b6 7ff0000000000000
// 4158bac92af262e6 7ff0000000000000
// bf98a2773cb47237 3fef78877c075998
// c080f5aaa4e0668e 1e0395c2fe5482eb
// c0c65c914e3bbb1a 0000000000000000
// 3fd71f13102b33ce 3ff48d8c51990060
// 3e6924bd2c13f2a4 3ff0000008b6cdc4
// 4035402d0ef88402 41430950720d9248
// be20d335ec1dfd0e 3fefffffff456781
// bf2724e26d190e2c 3feffeff56f30ff9
// c1a6776763ade5a0 0000000000000000
// bf73d3e6f8b4d4aa 3fefe48f18f4502c
// 41668c332abcb596 7ff0000000000000
// beec4e02cfb706e8 3fefffec6178ae65
// 40dd70b10b863620 7ff0000000000000
// 3eac444c99c0d1a1 3ff000009cbe932f
// c178b7b6d077fabf 0000000000000000
// bf6b00e334851430 3fefed4dd247952c
// befc5f503bf32744 3fefffd8ab0105b1
// c021901561e5f3bb 3f629e13fbb91db4
// bf9fe1961bbabc9a 3fef511b6625c25a
// c0b82ff8a46d7370 0000000000000000
// bf441813cf8fabab 3feffc84c9ea8de1
// 4089045a47b8c079 71f75458a88018e8
// c07bf9e98e980d9f 23f4d41133b47132
// c08c2be7a4bbcb87 0796d091441bd648
// 40811f081d4297a7 622d6cc181090e3e
// 405566d61ad0e231 45485dc8be5e951a
// c08101b9da2c269e 1deb8e208426c94a
// c08a31e27a1d835e 0b8b2dccd53f0ac6
// c08aa54ac269a045 0aa43b23692b6fb4
// 4067ce94cf8091a0 4bd5f150ffa18b6e
// 408ba94277171cb2 7741d8568d123176
// 407afeb0d39dbd69 5aee3c4865ba1264
// 406dddc7ccb3faa2 4ede7f7897b81da0
// 4038ec6f580d899a 417e5953c9cfc0e3
// c05c9ef205a6bc34 38c6e328f93a2157
// c088d420c5366bbf 0e4660ba15d9ce22
// 40863da363414e13 6c6a140cef3210a5
// c089a72a0990087b 0ca1339e78f5bcab
// 406a4b15d13a7908 4d1457a3234bf5f7
// c065f8d41fe23f29 34f2b05878ea595e
// 40867b1e8721c6a9 6ce4f705b8df6347
// 4077c8882b2981fe 57b727a61fa281c8
// c07460d4022d36f8 2b8edf4db7e5225a
// c088043886eef719 0fe632d99071bd6e
// c07211561b55e493 2dde3334d60cb63a
// 408171fe0170a0b0 62d303b49e7b742c
// c053422327c05870 3b1f448c62925299
// c0889826c6d467f5 0ebf9530cd6895f6
// 407baeb37ff376d8 5b9e3fc8fa4f2024
// 408d71fa5ceb8ced 7ad2fdb542577e35
// c0863526f46028ca 13847a482678c83f
// c08a73755f0a012b 0b07b6e126949fdb
// c06077c19b5ceab4 37b320c688c1e007
// c08db6fd3eb3d7af 048176e2fa3801fb
// c084931eb03021f8 16c86b9ffb7e5678
// c080c58c2d474550 1e63c9bab1ed00d0
// 408ba17a7d743a80 77322fc9aecfcdad
// c0825a6354c0c00a 1b3a04e2f77f3cb7
// c0653ddadc66b0d6 3550c2cece6a3381
// 40898f904c7db4f1 730ed01460b79412
// 408c622861ab45a0 78b34a059f9841a5
// n=101592 hash=8c05dda9b4e32bb1
