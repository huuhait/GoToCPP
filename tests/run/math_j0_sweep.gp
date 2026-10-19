// phase: 09
// math.J0 on about 100k inputs: bit-identical to Go (hash of the result bits and 200 samples).

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
		y := math.J0(x)
		h = mix(h, math.Float64bits(y))
		if i%500 == 0 {
			fmt.Printf("%016x %016x\n", math.Float64bits(x), math.Float64bits(y))
		}
	}
	fmt.Printf("n=%d hash=%016x\n", len(xs), h)
}

// Output:
// 0000000000000000 3ff0000000000000
// 40862e42fefa39f0 3f907f50f640797f
// 4008000000000000 bfd0a4b0f4642393
// 4040bfffffffffff 3fa339065c1a3af8
// 5b65923ac30ec1f4 322da36f21bc0003
// c024bfdbf760976a bfcf52401959dfe1
// cc9f5dfc8a5c49d1 396e9f3460f9ccb9
// 28faf9b36567288d 3ff0000000000000
// 5c01af71ea368d0e 31e10ab37a504dbb
// 52bd26a497b45c45 b66ec350ab8b6487
// 57a8503628a803dd b3f93013a82655fe
// 9146970ff4ce0971 3ff0000000000000
// f56a5815e599ff1f a4d7af6fa124f928
// 398243dbd8701b2c 3ff0000000000000
// 5ba4a48740cb04df b20f65eeaba4d7a9
// da25dad4e83140c7 32a8131077d9e957
// 9ba45dbea8b9d076 3ff0000000000000
// f762bc1ba29bdb35 243002b27b220dad
// 662b00cc72923f60 aca71760d881f4f7
// 482a3a7945c16ef0 3bc997d4890eb4d7
// a8698fe6f1d1b6d7 3ff0000000000000
// e006469c13f2521e afd186f7201b58c5
// e1dd8f8aae0c18d4 2e9d44d931e9d2b3
// e11d793466eefc2b af513ad1466ead51
// 4dcd1ad5a02370a4 38d80cb1075ee508
// 0deb45aa655f885e 3ff0000000000000
// f6250f700d64e16e 24c92c4c326fa7e4
// d8536e85cdee3a05 33b724b42d14d35c
// 4e76b4f39cf95384 387cf30d24bd16f9
// f88bbc29e3ba37f8 a3988e47b3100305
// 73ea5c3d1e048b48 a5e12eb04f6ee07b
// 5c7b45e3f515c5bb 3198aa639367d157
// ab2567ceba64e421 3ff0000000000000
// eb8d783bcea270b8 aa1a94908cb7d6c5
// c3b9dee0ebf001a5 3e03bfec38eb8b8b
// 0e7855fba5612c1d 3ff0000000000000
// 836e10793726318c 3ff0000000000000
// 3b34dda55a15d732 3ff0000000000000
// 2f4e9ffba969cc56 3ff0000000000000
// 8b1e87ad9104ded4 3ff0000000000000
// 17c82fc123e13faa 3ff0000000000000
// f3be5d2a6268c34c a6027bf64964d0fe
// 4b335c2e086fec82 ba4552d6aebccb51
// 304c21caf2bd8d77 3ff0000000000000
// 3fe30bf498dd7094 3fed3a5abcf65bab
// 3fb5aac89bff8058 3feff155f79b4723
// 3feb34272e81d3a3 3fea7970f7cf2e29
// 3fe816820fd48922 3feb9fffe63df7ea
// bfd0fe16d5fa012a 3fef7042ffa14e1b
// bfd7e00241eae292 3feee575f34af0a0
// 3fd04afd572abb6e 3fef7bce91d6ea49
// bfd3fac47071ae3a 3fef399efba056fb
// 3fee624f10cecad8 3fe92f176afc6d45
// 3fea5ab522610f73 3feaccb01c454306
// bfefdb1385845498 3fe88cb87d96a60b
// bfeab98dff3b972c 3feaa8a4991bff60
// 3fe33b90cc1ec878 3fed2cbe59c9dbc7
// bfbd77b29473af38 3fefe4e30cb937f6
// bfc0d52264f94548 3fefdc9f13a51f0e
// bfaca7d38cbe0a80 3feff99609b44561
// 3fd89e3ee22ebdcc 3feed3c3a319f2bc
// 3fdb17eda830b6f4 3fee951002b793a3
// 3fdc928ac90fa0f4 3fee6cdd6ab28912
// bfe1262476c0cd54 3fedbe4c39b02a41
// bfd891a76658a7ee 3feed4f3a1ad12ac
// 3fe75dada7e86de4 3febdfeb3540fa18
// 3feb0ff8154e2eb2 3fea8771954e9b80
// 3fdf06d705d2c98a 3fee25b21381154b
// bfe3f76f15ef5ae3 3fecf5ddf30faf3f
// bfd111a2c1ca78b2 3fef6ef90b64a991
// 3fde48a32b9caa20 3fee3bd3210f3643
// 3fef97a196284ef1 3fe8aa35bccbe037
// 3fede2e4caecfaa7 3fe964ad2f91a846
// bfb8f8d3e2cf5e90 3fefec862aa5212d
// 3fb5e89f9a291078 3feff101d55c539b
// bfed44e8558c6b55 3fe9a62a2f12a578
// bfdb8a8438eb88d8 3fee891c86a19024
// bfef3966dac6296e 3fe8d31a8b455490
// 3fe177a452bbd5c2 3feda90da7524852
// 3fd5fa7497c7977e 3fef1040298b1ea0
// bfd0f1b0f3280d4c 3fef71138750d62a
// 3fdce23d0b00be16 3fee642513fb09df
// 3f9b4fb30606ac40 3feffe8b0ff50816
// 3febc6ef604f80de 3fea40089b9b422d
// bfd1425a423a5810 3fef6bbc5ce24039
// 3fc446f084e7184c 3fefccaf77974b06
// 3fc1dcd9164395c0 3fefd82a00780b08
// 3fed04e125e49e2c 3fe9c066b66754b4
// bfd9c1bae0d1834a 3feeb7a32fa3e3ff
// 3fe9c5140195eae9 3feb04b47d0605e2
// bfcbf4cde2506070 3fef9e98c6254253
// bfccc9fdcd96446c 3fef98b9df840e5f
// 3fe908bfc5faf522 3feb49b7ba80e110
// bfe7529c40e5d1d8 3febe3b176315884
// 3fddc5abc9eb9c18 3fee4ac48b78f573
// bfcd86ee3ae16e44 3fef936183af52ab
// bfc3b6e0915c3048 3fefcf7d6e1bf0d8
// bfe8a5ac83066a77 3feb6d58ee5d01c2
// 3fdb59c5446ed18c 3fee8e37e11bd050
// bfd090d7606e9e78 3fef775c61150333
// 3fe4d49c91a53133 3fecb2e4d027f2ce
// 3fe8ba0c569302d0 3feb660f05cbc33d
// 3fedd712a427ca3e 3fe9699cfc0f6270
// bfecd6ff4d77b8de 3fe9d317d642f82b
// 418b404fc3162484 3f1b6acfb690680f
// 41d17b6ce028e50d bee3cc82f2fa4afe
// 408655173bfff5d1 bf97316ac15f3f43
// bfea65e0e6524d11 3feac876d209753d
// be22aed75aeb9d29 3ff0000000000000
// c10cb224405f8efc bf5a6252e5eb14c6
// 3feb027bf746a552 3fea8ca594f9ed56
// c0609ecd76ff47a0 3fb14001d7116a0c
// c016e544370168a9 3fb15041de5febc5
// c0a93a015cc53ccd 3f6a4f5166afa6db
// 3e87c667839667c9 3fefffffffffffb9
// 3e11d98eda6dcb94 3ff0000000000000
// 416b3a4c3d7d9238 bef9afec99c9a446
// c1656317acd288fe bf2caa75897fbef1
// 3fee7cfa81808980 3fe923ca1c71b9ba
// bea144763594e4ae 3feffffffffffdac
// c06274676f3acee9 bfa7570d468088ee
// bf87a2dd3a378270 3fefffba2aba7318
// 3f0138f32b5b8be8 3fefffffffdaec61
// bf83230b4ef7d484 3fefffd23901250e
// 3e1422076fb38833 3ff0000000000000
// 4017a2641f288700 3fbfe3377bc90efd
// be1c28c11f1e1e4e 3ff0000000000000
// 40bf4cf742ff052e 3f6fddce17fba2ea
// c19940c466a73f26 3eff8a0b9115789a
// bfc84b098455eb88 3fefb665800c4642
// 3e73ccded0704d12 3feffffffffffff4
// c19259d0233a17c8 beed82153837ba8d
// 3e387a09cc4c64a0 3ff0000000000000
// bf31ffcdf0faf143 3feffffff5e03851
// bf2dc2f4ae75ee28 3feffffff9148032
// 413f1b588b0dde46 3f16d709c01ecf23
// c149b81eb24a22ae 3f092abce83eef38
// c03a3ee2751880ae 3fc2e77c55589988
// c040825333089264 3fb86df100dd263b
// c069e40733ff2e61 3f9e6c2e0036bfea
// 3fb3a5f40e85efb6 3feff3f0b4954689
// 4190e24e797072b6 3f18ba6f9f4b9b6c
// 4158bac92af262e6 bf337cd938e81b84
// bf98a2773cb47237 3feffed094140a8f
// c080f5aaa4e0668e 3f124e6c9a7d7307
// c0c65c914e3bbb1a 3f7c4b43310941b6
// 3fd71f13102b33ce 3feef6df93753ae4
// 3e6924bd2c13f2a4 3feffffffffffffb
// 4035402d0ef88402 bf7bae8a674afcc9
// be20d335ec1dfd0e 3ff0000000000000
// bf2724e26d190e2c 3feffffffbd0b400
// c1a6776763ade5a0 3f0ccfdbf4549bce
// bf73d3e6f8b4d4aa 3feffff3b6e3b216
// 41668c332abcb596 bf2e3a64c360da9b
// beec4e02cfb706e8 3feffffffff9bdaf
// 40dd70b10b863620 3f6bbe9472a4abc9
// 3eac444c99c0d1a1 3feffffffffff9c2
// c178b7b6d077fabf 3f18ce15a048a418
// bf6b00e334851430 3feffffa4da0652d
// befc5f503bf32744 3fefffffffe6d818
// c021901561e5f3bb bfa191947288a33e
// bf9fe1961bbabc9a 3feffe03d34fbd75
// c0b82ff8a46d7370 bf7a08f05bdc4831
// bf441813cf8fabab 3fefffffcd87548f
// 4089045a47b8c079 bf799139f719dc7d
// c07bf9e98e980d9f 3f9cd61b59c03629
// c08c2be7a4bbcb87 bf902b611199cdc6
// 40811f081d4297a7 3f9f55fbecc0ac7a
// 405566d61ad0e231 bfb61373e8dd0fb7
// c08101b9da2c269e bfa1795e4a72c1b0
// c08a31e27a1d835e bf7825f4dea86987
// c08aa54ac269a045 bf987f142bf373af
// 4067ce94cf8091a0 3f96e084d67f30c9
// 408ba94277171cb2 3f37defb50b91c3b
// 407afeb0d39dbd69 bf9d2eb9514af931
// 406dddc7ccb3faa2 3fa590e25b7e7394
// 4038ec6f580d899a 3fb61e51ad3b48ec
// c05c9ef205a6bc34 3faf8494ee27d539
// c088d420c5366bbf bf8aaed01bc15d95
// 40863da363414e13 3f92925c9a1a96fd
// c089a72a0990087b bf9c2d7f517647aa
// 406a4b15d13a7908 bfa0ec471e2086aa
// c065f8d41fe23f29 3fa2301289549f4c
// 40867b1e8721c6a9 bf94c5f3d62c3f7a
// 4077c8882b2981fe bfa36843cac35925
// c07460d4022d36f8 3f7412dde36ea466
// c088043886eef719 3f85b81ba51fcadb
// c07211561b55e493 3fa1ec45944e8887
// 408171fe0170a0b0 bf774e086edc0547
// c053422327c05870 3faec872f85e8257
// c0889826c6d467f5 3f938ae820d3e1d1
// 407baeb37ff376d8 bf9a28c650f8efd5
// 408d71fa5ceb8ced 3f8c09d41adab834
// c0863526f46028ca 3f9e57b0edda6652
// c08a73755f0a012b bf97d1760a5fd8f0
// c06077c19b5ceab4 3fa380d38b724b7c
// c08db6fd3eb3d7af 3f79918437ff6f7b
// c084931eb03021f8 bf90e4e414bc8c86
// c080c58c2d474550 bf828f634efb9e95
// 408ba17a7d743a80 bf967e7caae84993
// c0825a6354c0c00a bf9332f8f2e7fbee
// c0653ddadc66b0d6 3fab839a12b84616
// 40898f904c7db4f1 3f9ae0698e1d2ace
// 408c622861ab45a0 bf9892a924030244
// n=101592 hash=daac077025c5811f
