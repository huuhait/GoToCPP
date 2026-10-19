// phase: 09
// math.Erf on about 100k inputs: bit-identical to Go (hash of the result bits and 200 samples).

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
		y := math.Erf(x)
		h = mix(h, math.Float64bits(y))
		if i%500 == 0 {
			fmt.Printf("%016x %016x\n", math.Float64bits(x), math.Float64bits(y))
		}
	}
	fmt.Printf("n=%d hash=%016x\n", len(xs), h)
}

// Output:
// 0000000000000000 0000000000000000
// 40862e42fefa39f0 3ff0000000000000
// 4008000000000000 3fefffd1ac4135f9
// 4040bfffffffffff 3ff0000000000000
// 5b65923ac30ec1f4 3ff0000000000000
// c024bfdbf760976a bff0000000000000
// cc9f5dfc8a5c49d1 bff0000000000000
// 28faf9b36567288d 28fe703fb681e114
// 5c01af71ea368d0e 3ff0000000000000
// 52bd26a497b45c45 3ff0000000000000
// 57a8503628a803dd 3ff0000000000000
// 9146970ff4ce0971 91497d7caf9675f3
// f56a5815e599ff1f bff0000000000000
// 398243dbd8701b2c 39849c2441977a9d
// 5ba4a48740cb04df 3ff0000000000000
// da25dad4e83140c7 bff0000000000000
// 9ba45dbea8b9d076 9ba6fb14ba959e94
// f762bc1ba29bdb35 bff0000000000000
// 662b00cc72923f60 3ff0000000000000
// 482a3a7945c16ef0 3ff0000000000000
// a8698fe6f1d1b6d7 a86cd800b999a62b
// e006469c13f2521e bff0000000000000
// e1dd8f8aae0c18d4 bff0000000000000
// e11d793466eefc2b bff0000000000000
// 4dcd1ad5a02370a4 3ff0000000000000
// 0deb45aa655f885e 0deec5f74d6d6583
// f6250f700d64e16e bff0000000000000
// d8536e85cdee3a05 bff0000000000000
// 4e76b4f39cf95384 3ff0000000000000
// f88bbc29e3ba37f8 bff0000000000000
// 73ea5c3d1e048b48 3ff0000000000000
// 5c7b45e3f515c5bb 3ff0000000000000
// ab2567ceba64e421 ab28274cf7f30112
// eb8d783bcea270b8 bff0000000000000
// c3b9dee0ebf001a5 bff0000000000000
// 0e7855fba5612c1d 0e7b75c873fc682e
// 836e10793726318c 8370f6452a5a10f6
// 3b34dda55a15d732 3b378b66e6bac61b
// 2f4e9ffba969cc56 2f51473c9d3ae408
// 8b1e87ad9104ded4 8b2139862c3e8bb4
// 17c82fc123e13faa 17cb4aa59051566a
// f3be5d2a6268c34c bff0000000000000
// 4b335c2e086fec82 3ff0000000000000
// 304c21caf2bd8d77 304fbe5a592faa66
// 3fe30bf498dd7094 3fe333ce2b8653ff
// 3fb5aac89bff8058 3fb863f64bffc9cd
// 3feb34272e81d3a3 3fe8a9d5b71dd68d
// 3fe816820fd48922 3fe6d03a6f692816
// bfd0fe16d5fa012a bfd2bb9b50afd4fb
// bfd7e00241eae292 bfd9bdaa760aa3ac
// 3fd04afd572abb6e 3fd1febaed41a7c7
// bfd3fac47071ae3a bfd5d54437da0544
// 3fee624f10cecad8 3fea42dd4a5c5f2e
// 3fea5ab522610f73 3fe83006e461a600
// bfefdb1385845498 bfeae802257e55cc
// bfeab98dff3b972c bfe865d2d8310937
// 3fe33b90cc1ec878 3fe3595eef140c37
// bfbd77b29473af38 bfc08d5a4c034531
// bfc0d52264f94548 bfc2e274173a7566
// bfaca7d38cbe0a80 bfb0267abaeb936d
// 3fd89e3ee22ebdcc 3fda779f669e3514
// 3fdb17eda830b6f4 3fdcd6edc7a767b2
// 3fdc928ac90fa0f4 3fde38852e386e85
// bfe1262476c0cd54 bfe1a5ba83bada7f
// bfd891a76658a7ee bfda6b5d6ca2f369
// 3fe75dada7e86de4 3fe657df752a5456
// 3feb0ff8154e2eb2 3fe895f0ae44b5ca
// 3fdf06d705d2c98a 3fe039a0ca32dffb
// bfe3f76f15ef5ae3 bfe3eb0b7abb5047
// bfd111a2c1ca78b2 bfd2d027a302158a
// 3fde48a32b9caa20 3fdfc8a2f94e9c55
// 3fef97a196284ef1 3feacb86bd7c903d
// 3fede2e4caecfaa7 3fea07a2c47a2b22
// bfb8f8d3e2cf5e90 bfbc16b809d98381
// 3fb5e89f9a291078 3fb8a93cbb473d6f
// bfed44e8558c6b55 bfe9bbc4fb2e0e73
// bfdb8a8438eb88d8 bfdd42b135e10cd2
// bfef3966dac6296e bfeaa2f364ab68fa
// 3fe177a452bbd5c2 3fe1ea5d7109b666
// 3fd5fa7497c7977e 3fd7dbc9f9a6daa0
// bfd0f1b0f3280d4c bfd2ae91122bef40
// 3fdce23d0b00be16 3fde82097db77b0d
// 3f9b4fb30606ac40 3f9ecf6ae4b65f67
// 3febc6ef604f80de 3fe8f903fd591236
// bfd1425a423a5810 bfd3034f95dad88e
// 3fc446f084e7184c 3fc6b0b7aa4948f3
// 3fc1dcd9164395c0 3fc4069ae870305b
// 3fed04e125e49e2c 3fe99c3fc206e8ae
// bfd9c1bae0d1834a bfdb915442185f1a
// 3fe9c5140195eae9 3fe7d90cdaed130d
// bfcbf4cde2506070 bfcf0cff78f67d68
// bfccc9fdcd96446c bfcff204fa2861ce
// 3fe908bfc5faf522 3fe767e2a681df45
// bfe7529c40e5d1d8 bfe65089b45b9cc4
// 3fddc5abc9eb9c18 3fdf520ea0a0a2cd
// bfcd86ee3ae16e44 bfd05e376ec5b2ba
// bfc3b6e0915c3048 bfc612146a2ef284
// bfe8a5ac83066a77 bfe72ab0b1f2d46a
// 3fdb59c5446ed18c 3fdd14ed9fbabaf4
// bfd090d7606e9e78 bfd24885c96980bc
// 3fe4d49c91a53133 3fe4914718053bae
// 3fe8ba0c569302d0 3fe7375e5a3b199a
// 3fedd712a427ca3e 3fea020d7f382b4f
// bfecd6ff4d77b8de bfe9856292fbb11f
// 418b404fc3162484 3ff0000000000000
// 41d17b6ce028e50d 3ff0000000000000
// 408655173bfff5d1 3ff0000000000000
// bfea65e0e6524d11 bfe8366aaed50cfd
// be22aed75aeb9d29 be2514dbc068c178
// c10cb224405f8efc bff0000000000000
// 3feb027bf746a552 3fe88e7cdfd18f31
// c0609ecd76ff47a0 bff0000000000000
// c016e544370168a9 bfeffffffffffffb
// c0a93a015cc53ccd bff0000000000000
// 3e87c667839667c9 3e8ad3c5997615ce
// 3e11d98eda6dcb94 3e142431aec6ae76
// 416b3a4c3d7d9238 3ff0000000000000
// c1656317acd288fe bff0000000000000
// 3fee7cfa81808980 3fea4f0af731acdd
// bea144763594e4ae bea37bf4fae5a98a
// c06274676f3acee9 bff0000000000000
// bf87a2dd3a378270 bf8aab5dae5ecd64
// 3f0138f32b5b8be8 3f036ef799e43ad5
// bf83230b4ef7d484 bf8597d189248d2c
// 3e1422076fb38833 3e16b7b2f1f69d35
// 4017a2641f288700 3fefffffffffffff
// be1c28c11f1e1e4e be1fc63550cf3bf8
// 40bf4cf742ff052e 3ff0000000000000
// c19940c466a73f26 bff0000000000000
// bfc84b098455eb88 bfcb161406341a0b
// 3e73ccded0704d12 3e76579b93cc8a4d
// c19259d0233a17c8 bff0000000000000
// 3e387a09cc4c64a0 3e3b9e779077aeb3
// bf31ffcdf0faf143 bf344f59b5861b0a
// bf2dc2f4ae75ee28 bf30ca8911331adf
// 413f1b588b0dde46 3ff0000000000000
// c149b81eb24a22ae bff0000000000000
// c03a3ee2751880ae bff0000000000000
// c040825333089264 bff0000000000000
// c069e40733ff2e61 bff0000000000000
// 3fb3a5f40e85efb6 3fb62091d15fc6e9
// 4190e24e797072b6 3ff0000000000000
// 4158bac92af262e6 3ff0000000000000
// bf98a2773cb47237 bf9bcab646ae2d2a
// c080f5aaa4e0668e bff0000000000000
// c0c65c914e3bbb1a bff0000000000000
// 3fd71f13102b33ce 3fd8ff6b7fec4b04
// 3e6924bd2c13f2a4 3e6c5f1507189f1c
// 4035402d0ef88402 3ff0000000000000
// be20d335ec1dfd0e be22fc2aaffc422c
// bf2724e26d190e2c bf2a1d8421e27b8d
// c1a6776763ade5a0 bff0000000000000
// bf73d3e6f8b4d4aa bf765f7f627256d6
// 41668c332abcb596 3ff0000000000000
// beec4e02cfb706e8 beeff03f72300224
// 40dd70b10b863620 3ff0000000000000
// 3eac444c99c0d1a1 3eafe54a0ec64d96
// c178b7b6d077fabf bff0000000000000
// bf6b00e334851430 bf6e78547b8a6309
// befc5f503bf32744 bf0001e2c1ccc362
// c021901561e5f3bb bff0000000000000
// bf9fe1961bbabc9a bfa1fb31d3611059
// c0b82ff8a46d7370 bff0000000000000
// bf441813cf8fabab bf46ac781241df4b
// 4089045a47b8c079 3ff0000000000000
// c07bf9e98e980d9f bff0000000000000
// c08c2be7a4bbcb87 bff0000000000000
// 40811f081d4297a7 3ff0000000000000
// 405566d61ad0e231 3ff0000000000000
// c08101b9da2c269e bff0000000000000
// c08a31e27a1d835e bff0000000000000
// c08aa54ac269a045 bff0000000000000
// 4067ce94cf8091a0 3ff0000000000000
// 408ba94277171cb2 3ff0000000000000
// 407afeb0d39dbd69 3ff0000000000000
// 406dddc7ccb3faa2 3ff0000000000000
// 4038ec6f580d899a 3ff0000000000000
// c05c9ef205a6bc34 bff0000000000000
// c088d420c5366bbf bff0000000000000
// 40863da363414e13 3ff0000000000000
// c089a72a0990087b bff0000000000000
// 406a4b15d13a7908 3ff0000000000000
// c065f8d41fe23f29 bff0000000000000
// 40867b1e8721c6a9 3ff0000000000000
// 4077c8882b2981fe 3ff0000000000000
// c07460d4022d36f8 bff0000000000000
// c088043886eef719 bff0000000000000
// c07211561b55e493 bff0000000000000
// 408171fe0170a0b0 3ff0000000000000
// c053422327c05870 bff0000000000000
// c0889826c6d467f5 bff0000000000000
// 407baeb37ff376d8 3ff0000000000000
// 408d71fa5ceb8ced 3ff0000000000000
// c0863526f46028ca bff0000000000000
// c08a73755f0a012b bff0000000000000
// c06077c19b5ceab4 bff0000000000000
// c08db6fd3eb3d7af bff0000000000000
// c084931eb03021f8 bff0000000000000
// c080c58c2d474550 bff0000000000000
// 408ba17a7d743a80 3ff0000000000000
// c0825a6354c0c00a bff0000000000000
// c0653ddadc66b0d6 bff0000000000000
// 40898f904c7db4f1 3ff0000000000000
// 408c622861ab45a0 3ff0000000000000
// n=101592 hash=f15d53aa936503c5
