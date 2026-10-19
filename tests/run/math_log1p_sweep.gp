// phase: 09
// math.Log1p on about 100k inputs: bit-identical to Go (hash of the result bits and 200 samples).

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
		y := math.Log1p(x)
		h = mix(h, math.Float64bits(y))
		if i%500 == 0 {
			fmt.Printf("%016x %016x\n", math.Float64bits(x), math.Float64bits(y))
		}
	}
	fmt.Printf("n=%d hash=%016x\n", len(xs), h)
}

// Output:
// 0000000000000000 0000000000000000
// 40862e42fefa39f0 401a43f5a6e6b0c3
// 4008000000000000 3ff62e42fefa39ef
// 4040bfffffffffff 400c53e27b69a6f9
// 5b65923ac30ec1f4 407309723581998e
// c024bfdbf760976a 7ff8000000000001
// cc9f5dfc8a5c49d1 7ff8000000000001
// 28faf9b36567288d 28faf9b36567288d
// 5c01af71ea368d0e 4073752bf4354e7c
// 52bd26a497b45c45 406a1168e1a7af3e
// 57a8503628a803dd 407071f08192f4b6
// 9146970ff4ce0971 9146970ff4ce0971
// f56a5815e599ff1f 7ff8000000000001
// 398243dbd8701b2c 398243dbd8701b2c
// 5ba4a48740cb04df 4073351a83eca75e
// da25dad4e83140c7 7ff8000000000001
// 9ba45dbea8b9d076 9ba45dbea8b9d076
// f762bc1ba29bdb35 7ff8000000000001
// 662b00cc72923f60 407a8094a5df572b
// 482a3a7945c16ef0 4056d2fa6edabd8f
// a8698fe6f1d1b6d7 a8698fe6f1d1b6d7
// e006469c13f2521e 7ff8000000000001
// e1dd8f8aae0c18d4 7ff8000000000001
// e11d793466eefc2b 7ff8000000000001
// 4dcd1ad5a02370a4 406339153bed68fb
// 0deb45aa655f885e 0deb45aa655f885e
// f6250f700d64e16e 7ff8000000000001
// d8536e85cdee3a05 7ff8000000000001
// 4e76b4f39cf95384 406425208cf6e720
// f88bbc29e3ba37f8 7ff8000000000001
// 73ea5c3d1e048b48 4082040960089d48
// 5c7b45e3f515c5bb 4073c9bc19ee47c6
// ab2567ceba64e421 ab2567ceba64e421
// eb8d783bcea270b8 7ff8000000000001
// c3b9dee0ebf001a5 7ff8000000000001
// 0e7855fba5612c1d 0e7855fba5612c1d
// 836e10793726318c 836e10793726318c
// 3b34dda55a15d732 3b34dda55a15d732
// 2f4e9ffba969cc56 2f4e9ffba969cc56
// 8b1e87ad9104ded4 8b1e87ad9104ded4
// 17c82fc123e13faa 17c82fc123e13faa
// f3be5d2a6268c34c 7ff8000000000001
// 4b335c2e086fec82 405f3d416711a33e
// 304c21caf2bd8d77 304c21caf2bd8d77
// 3fe30bf498dd7094 3fdde36902afbd34
// 3fb5aac89bff8058 3fb4cc82376d5040
// 3feb34272e81d3a3 3fe3b01dabd57301
// 3fe816820fd48922 3fe1f53964c38cb0
// bfd0fe16d5fa012a bfd3bfb86c3237d0
// bfd7e00241eae292 bfdde16ef0b940ea
// 3fd04afd572abb6e 3fcd07bb6d4e2dfa
// bfd3fac47071ae3a bfd7f35e3bf30c38
// 3fee624f10cecad8 3fe55cc28a810c26
// 3fea5ab522610f73 3fe339bbe2bbc4cf
// bfefdb1385845498 c0159bb53b9a75df
// bfeab98dff3b972c bffcd7f4c3d6591e
// 3fe33b90cc1ec878 3fde1efe3eb2a280
// bfbd77b29473af38 bfbf4e5a6beee5b0
// bfc0d52264f94548 bfc20c0c73c50732
// bfaca7d38cbe0a80 bfad7d1ba80cf8aa
// 3fd89e3ee22ebdcc 3fd4d43b3917084c
// 3fdb17eda830b6f4 3fd6979a10198093
// 3fdc928ac90fa0f4 3fd79f785a0487ce
// bfe1262476c0cd54 bfe890bd163cd2bc
// bfd891a76658a7ee bfdeff424f594279
// 3fe75dada7e86de4 3fe18b169365a25c
// 3feb0ff8154e2eb2 3fe39c88eb71f150
// 3fdf06d705d2c98a 3fd94c2f949554c1
// bfe3f76f15ef5ae3 bfef4c246562f9dc
// bfd111a2c1ca78b2 bfd3da5abad04250
// 3fde48a32b9caa20 3fd8cb94ff906d26
// 3fef97a196284ef1 3fe5f9e90eba4387
// 3fede2e4caecfaa7 3fe51b23d5ba3214
// bfb8f8d3e2cf5e90 bfba46838c665f7e
// 3fb5e89f9a291078 3fb5057f874ee4a7
// bfed44e8558c6b55 c003b0651d60cb50
// bfdb8a8438eb88d8 bfe2019d0203fcba
// bfef3966dac6296e c00dc1cd2172ed13
// 3fe177a452bbd5c2 3fdbe07e40938d5d
// 3fd5fa7497c7977e 3fd2e4c24b437689
// bfd0f1b0f3280d4c bfd3aed9722c5e89
// 3fdce23d0b00be16 3fd7d679cca37da4
// 3f9b4fb30606ac40 3f9af4160cf590d1
// 3febc6ef604f80de 3fe3ff1221e7847f
// bfd1425a423a5810 bfd41cecb41a94ee
// 3fc446f084e7184c 3fc2d29eadb7e754
// 3fc1dcd9164395c0 3fc0b8a95135a7fd
// 3fed04e125e49e2c 3fe4a7878debb609
// bfd9c1bae0d1834a bfe07a3190c6cdf1
// 3fe9c5140195eae9 3fe2e74470c35e56
// bfcbf4cde2506070 bfcf8ac899212712
// bfccc9fdcd96446c bfd04e57bd255f67
// 3fe908bfc5faf522 3fe27e471cbac3ad
// bfe7529c40e5d1d8 bff4e162019865e0
// 3fddc5abc9eb9c18 3fd8727092b3a296
// bfcd86ee3ae16e44 bfd0c8ae5c8959a6
// bfc3b6e0915c3048 bfc568b3b6614b84
// bfe8a5ac83066a77 bff787c809b2d74a
// 3fdb59c5446ed18c 3fd6c5cbb52a2ebc
// bfd090d7606e9e78 bfd32ba6ebdbd8d6
// 3fe4d49c91a53133 3fe00b1587167298
// 3fe8ba0c569302d0 3fe252007941b96e
// 3fedd712a427ca3e 3fe515067f2a68ba
// bfecd6ff4d77b8de c0028558f2ed4050
// 418b404fc3162484 4031dc76e1bec9ca
// 41d17b6ce028e50d 4034e20cacf7f9ea
// 408655173bfff5d1 401a4aeda32d1548
// bfea65e0e6524d11 bffbe1b66c03aa97
// be22aed75aeb9d29 be22aed75b42e096
// c10cb224405f8efc 7ff8000000000001
// 3feb027bf746a552 3fe39539ba53b347
// c0609ecd76ff47a0 7ff8000000000001
// c016e544370168a9 7ff8000000000001
// c0a93a015cc53ccd 7ff8000000000001
// 3e87c667839667c9 3e87c66760426230
// 3e11d98eda6dcb94 3e11d98eda45f7d8
// 416b3a4c3d7d9238 4030795a2e119798
// c1656317acd288fe 7ff8000000000001
// 3fee7cfa81808980 3fe56a6dc4b4c3e2
// bea144763594e4ae bea14476801f66e3
// c06274676f3acee9 7ff8000000000001
// bf87a2dd3a378270 bf87c60d5047cc93
// 3f0138f32b5b8be8 3f0138e0a1a6b631
// bf83230b4ef7d484 bf833a1398fe7801
// 3e1422076fb38833 3e1422076f80dd7d
// 4017a2641f288700 3ffeec9ad5c2c7bb
// be1c28c11f1e1e4e be1c28c11f813c65
// 40bf4cf742ff052e 4021fa567a9b51cb
// c19940c466a73f26 7ff8000000000001
// bfc84b098455eb88 bfcaf06919404fd3
// 3e73ccded0704d12 3e73ccdec42fe4e6
// c19259d0233a17c8 7ff8000000000001
// 3e387a09cc4c64a0 3e387a09cb20d69e
// bf31ffcdf0faf143 bf32006ff50e0ef8
// bf2dc2f4ae75ee28 bf2dc3d22704e621
// 413f1b588b0dde46 402d0e3980b45eb8
// c149b81eb24a22ae 7ff8000000000001
// c03a3ee2751880ae 7ff8000000000001
// c040825333089264 7ff8000000000001
// c069e40733ff2e61 7ff8000000000001
// 3fb3a5f40e85efb6 3fb2ee446ffe876c
// 4190e24e797072b6 4032135ad9738e98
// 4158bac92af262e6 402f5e8aa7d433c4
// bf98a2773cb47237 bf98ef901b2d343d
// c080f5aaa4e0668e 7ff8000000000001
// c0c65c914e3bbb1a 7ff8000000000001
// 3fd71f13102b33ce 3fd3bd2441677b9d
// 3e6924bd2c13f2a4 3e6924bd223329c8
// 4035402d0ef88402 4008d1a8e3374dff
// be20d335ec1dfd0e be20d335ec64c24e
// bf2724e26d190e2c bf2725685aa24cd3
// c1a6776763ade5a0 7ff8000000000001
// bf73d3e6f8b4d4aa bf73e03a4604086d
// 41668c332abcb596 40304911c8cb6e34
// beec4e02cfb706e8 beec4e0f545ff53e
// 40dd70b10b863620 4024a0b30f2ee8b5
// 3eac444c99c0d1a1 3eac444bd2001a60
// c178b7b6d077fabf 7ff8000000000001
// bf6b00e334851430 bf6b0c4e612f908a
// befc5f503bf32744 befc5f6963f932a3
// c021901561e5f3bb 7ff8000000000001
// bf9fe1961bbabc9a bfa031ab412ac529
// c0b82ff8a46d7370 7ff8000000000001
// bf441813cf8fabab bf4419a7bf32a8cd
// 4089045a47b8c079 401abf04557b12ac
// c07bf9e98e980d9f 7ff8000000000001
// c08c2be7a4bbcb87 7ff8000000000001
// 40811f081d4297a7 40193b445b4c5ed3
// 405566d61ad0e231 4011d8739dc7dd6a
// c08101b9da2c269e 7ff8000000000001
// c08a31e27a1d835e 7ff8000000000001
// c08aa54ac269a045 7ff8000000000001
// 4067ce94cf8091a0 401504c49cf14aa5
// 408ba94277171cb2 401b25c7b5e6e5cf
// 407afeb0d39dbd69 4018483decf76d16
// 406dddc7ccb3faa2 4015ebe0d6c37f86
// 4038ec6f580d899a 400a0a8d85f71b02
// c05c9ef205a6bc34 7ff8000000000001
// c088d420c5366bbf 7ff8000000000001
// 40863da363414e13 401a46b992b69c1f
// c089a72a0990087b 7ff8000000000001
// 406a4b15d13a7908 401569fb8687ddc0
// c065f8d41fe23f29 7ff8000000000001
// 40867b1e8721c6a9 401a51b526dcb06a
// 4077c8882b2981fe 4017c6dbd65c3948
// c07460d4022d36f8 7ff8000000000001
// c088043886eef719 7ff8000000000001
// c07211561b55e493 7ff8000000000001
// 408171fe0170a0b0 40194e6ee1fb2701
// c053422327c05870 7ff8000000000001
// c0889826c6d467f5 7ff8000000000001
// 407baeb37ff376d8 401861efe6d98972
// 408d71fa5ceb8ced 401b65b65ad2eba5
// c0863526f46028ca 7ff8000000000001
// c08a73755f0a012b 7ff8000000000001
// c06077c19b5ceab4 7ff8000000000001
// c08db6fd3eb3d7af 7ff8000000000001
// c084931eb03021f8 7ff8000000000001
// c080c58c2d474550 7ff8000000000001
// 408ba17a7d743a80 401b24a7d2ef33b1
// c0825a6354c0c00a 7ff8000000000001
// c0653ddadc66b0d6 7ff8000000000001
// 40898f904c7db4f1 401ad50289321755
// 408c622861ab45a0 401b402529f71aaa
// n=101592 hash=6a719be57f0373be
