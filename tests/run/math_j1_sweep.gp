// phase: 09
// math.J1 on about 100k inputs: bit-identical to Go (hash of the result bits and 200 samples).

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
		y := math.J1(x)
		h = mix(h, math.Float64bits(y))
		if i%500 == 0 {
			fmt.Printf("%016x %016x\n", math.Float64bits(x), math.Float64bits(y))
		}
	}
	fmt.Printf("n=%d hash=%016x\n", len(xs), h)
}

// Output:
// 0000000000000000 0000000000000000
// 40862e42fefa39f0 bf99d72732d959d1
// 4008000000000000 3fd5b324589238ce
// 4040bfffffffffff 3fc10d0126aab036
// 5b65923ac30ec1f4 3212d42001ecba17
// c024bfdbf760976a 3fa94f2abc50ed7f
// cc9f5dfc8a5c49d1 3991d43afa4b7dee
// 28faf9b36567288d 28eaf9b36567288d
// 5c01af71ea368d0e b1b0e77609769a77
// 52bd26a497b45c45 3681481fc131e75c
// 57a8503628a803dd b40a722c8b16fb41
// 9146970ff4ce0971 1136970ff4ce0971
// f56a5815e599ff1f a52c21565dad2bc8
// 398243dbd8701b2c 397243dbd8701b2c
// 5ba4a48740cb04df 31e3e31be43fb354
// da25dad4e83140c7 32ce4da6ff0c87e3
// 9ba45dbea8b9d076 1b945dbea8b9d076
// f762bc1ba29bdb35 a412c676cebf4bd1
// 662b00cc72923f60 accb3036ff77c03e
// 482a3a7945c16ef0 bbb7b17e0e8cdf6f
// a8698fe6f1d1b6d7 28598fe6f1d1b6d7
// e006469c13f2521e afd915d6594e4591
// e1dd8f8aae0c18d4 2ef2c75424278ff0
// e11d793466eefc2b af3e34ccc54579f7
// 4dcd1ad5a02370a4 38fa16979d0d1f7c
// 0deb45aa655f885e 0ddb45aa655f885e
// f6250f700d64e16e a4c2e3ea36688057
// d8536e85cdee3a05 3371316d2f097798
// 4e76b4f39cf95384 38a51fef2163e824
// f88bbc29e3ba37f8 23886d15c3b2cdd0
// 73ea5c3d1e048b48 a5e6462bf4f59244
// 5c7b45e3f515c5bb b19e5ab1d1214e6e
// ab2567ceba64e421 2b1567ceba64e421
// eb8d783bcea270b8 29d2a5b37e8f24ca
// c3b9dee0ebf001a5 bddcfb538357d21f
// 0e7855fba5612c1d 0e6855fba5612c1d
// 836e10793726318c 035e10793726318c
// 3b34dda55a15d732 3b24dda55a15d732
// 2f4e9ffba969cc56 2f3e9ffba969cc56
// 8b1e87ad9104ded4 0b0e87ad9104ded4
// 17c82fc123e13faa 17b82fc123e13faa
// f3be5d2a6268c34c a5c5bc67ea51cd9d
// 4b335c2e086fec82 ba32564767d89da0
// 304c21caf2bd8d77 303c21caf2bd8d77
// 3fe30bf498dd7094 3fd23730f6fcdb97
// 3fb5aac89bff8058 3fa5a5d17f10ef00
// 3feb34272e81d3a3 3fd8d1b148c7b634
// 3fe816820fd48922 3fd66bef0d937c2c
// bfd0fe16d5fa012a bfc0d7de8de2c346
// bfd7e00241eae292 bfc7764d434defbd
// 3fd04afd572abb6e 3fc0294a714de479
// bfd3fac47071ae3a bfc3bcb6160b3c46
// 3fee624f10cecad8 3fdb160ea0d6f0fa
// 3fea5ab522610f73 3fd82ea250e1a07e
// bfefdb1385845498 bfdc11b933ae00d8
// bfeab98dff3b972c bfd8762951a68440
// 3fe33b90cc1ec878 3fd26092445fe878
// bfbd77b29473af38 bfad6b35ded63741
// bfc0d52264f94548 bfb0cbd37c853325
// bfaca7d38cbe0a80 bf9ca4f45302628a
// 3fd89e3ee22ebdcc 3fc82a66788e1d05
// 3fdb17eda830b6f4 3fca7db55010d92b
// 3fdc928ac90fa0f4 3fcbddd079bcdee2
// bfe1262476c0cd54 bfd08a69e2a94099
// bfd891a76658a7ee bfc81e7faa459521
// 3fe75dada7e86de4 3fd5d7c7b1b27463
// 3feb0ff8154e2eb2 3fd8b6c9781ed71d
// 3fdf06d705d2c98a 3fce1fc4f386ca5e
// bfe3f76f15ef5ae3 bfd302b06ba3c8a9
// bfd111a2c1ca78b2 bfc0eae6403fd511
// 3fde48a32b9caa20 3fcd71ac3d0e657f
// 3fef97a196284ef1 3fdbe57cc16b9632
// 3fede2e4caecfaa7 3fdabe79389e952b
// bfb8f8d3e2cf5e90 bfa8f13a1778be70
// 3fb5e89f9a291078 3fa5e37d82188d72
// bfed44e8558c6b55 bfda502cafbff8f2
// bfdb8a8438eb88d8 bfcae890edca1334
// bfef3966dac6296e bfdba715c74945b8
// 3fe177a452bbd5c2 3fd0d326e2e62c5c
// 3fd5fa7497c7977e 3fc5a7eb81c5fadd
// bfd0f1b0f3280d4c bfc0cbcbeccff8aa
// 3fdce23d0b00be16 3fcc279298198b45
// 3f9b4fb30606ac40 3f8b4f13dfbbf94c
// 3febc6ef604f80de 3fd93de37e371bd8
// bfd1425a423a5810 bfc11a4f0bfa6ad2
// 3fc446f084e7184c 3fb436ac4c001cb2
// 3fc1dcd9164395c0 3fb1d1b99fe4a899
// 3fed04e125e49e2c 3fda22f15be92296
// bfd9c1bae0d1834a bfc93d221ec6076d
// 3fe9c5140195eae9 3fd7bc8b16e10087
// bfcbf4cde2506070 bfbbca36ed2d7798
// bfccc9fdcd96446c bfbc9b7ca1a29684
// 3fe908bfc5faf522 3fd72acea1ae6d1b
// bfe7529c40e5d1d8 bfd5ced72ceed27a
// 3fddc5abc9eb9c18 3fccf95a991e3b35
// bfcd86ee3ae16e44 bfbd54c32d7c60f2
// bfc3b6e0915c3048 bfb3a7ed4c50c77f
// bfe8a5ac83066a77 bfd6dd3338603336
// 3fdb59c5446ed18c 3fcabb23a64044d1
// bfd090d7606e9e78 bfc06d6c94f12e7a
// 3fe4d49c91a53133 3fd3bf196dc918d4
// 3fe8ba0c569302d0 3fd6ed363bc5f74a
// 3fedd712a427ca3e 3fdab6491bc025d1
// bfecd6ff4d77b8de bfda0257f3bed938
// 418b404fc3162484 3eedb634ff4176a8
// 41d17b6ce028e50d 3ef6547f74ee6af4
// 408655173bfff5d1 bf93ebb06019ff4e
// bfea65e0e6524d11 bfd8371730cf60c9
// be22aed75aeb9d29 3e12aed75aeb9d29
// c10cb224405f8efc bf3636ad0e238993
// 3feb027bf746a552 3fd8acbc8dfc899a
// c0609ecd76ff47a0 bf905e4936c8b85e
// c016e544370168a9 3fd48fbff56c087a
// c0a93a015cc53ccd 3f8bfe3df77ff4c2
// 3e87c667839667c9 3e77c667839667af
// 3e11d98eda6dcb94 3e01d98eda6dcb94
// 416b3a4c3d7d9238 bf2b7e21cee4f0fd
// c1656317acd288fe 3f18cab8a5d5643b
// 3fee7cfa81808980 3fdb283ba2bf4f2b
// bea144763594e4ae be9144763594e40d
// c06274676f3acee9 bfa81eda08ed15f6
// bf87a2dd3a378270 bf77a2c36fd89713
// 3f0138f32b5b8be8 3ef138f32b5191b4
// bf83230b4ef7d484 bf7322fd9ed19fb8
// 3e1422076fb38833 3e0422076fb38833
// 4017a2641f288700 bfd2cb41eb4a4afc
// be1c28c11f1e1e4e 3e0c28c11f1e1e4e
// 40bf4cf742ff052e 3f806cce9f1e0ae3
// c19940c466a73f26 3f12bb3c80283571
// bfc84b098455eb88 bfb82f13e9526954
// 3e73ccded0704d12 3e63ccded0704d0e
// c19259d0233a17c8 3f178dc69e266ed8
// 3e387a09cc4c64a0 3e287a09cc4c64a0
// bf31ffcdf0faf143 bf21ffcdee220905
// bf2dc2f4ae75ee28 bf1dc2f4ab3e23ee
// 413f1b588b0dde46 bf421665e03539e1
// c149b81eb24a22ae bf3c4e333ebc7a2b
// c03a3ee2751880ae bfaabbdaf821c379
// c040825333089264 bfba313f39ac811a
// c069e40733ff2e61 3fa7edbe342a7ad4
// 3fb3a5f40e85efb6 3fa3a24020eec8c4
// 4190e24e797072b6 3ee40c14ef88ab98
// 4158bac92af262e6 bf19eccad0143225
// bf98a2773cb47237 bf88a20271a13285
// c080f5aaa4e0668e bfa1892d5f8857fa
// c0c65c914e3bbb1a bf67027c3c3994f4
// 3fd71f13102b33ce 3fc6bf07d389edd0
// 3e6924bd2c13f2a4 3e5924bd2c13f2a2
// 4035402d0ef88402 3fc61ed1fff96c26
// be20d335ec1dfd0e 3e10d335ec1dfd0e
// bf2724e26d190e2c bf1724e26b95a573
// c1a6776763ade5a0 bef3d6ed3663aa57
// bf73d3e6f8b4d4aa bf63d3e32a52ee8f
// 41668c332abcb596 befb1120c27e056b
// beec4e02cfb706e8 bedc4e02cfb44244
// 40dd70b10b863620 bf6971980a9a56f7
// 3eac444c99c0d1a1 3e9c444c99c0cedf
// c178b7b6d077fabf bf206024e2774b75
// bf6b00e334851430 bf5b00e0cd307c66
// befc5f503bf32744 beec5f503be80058
// c021901561e5f3bb bfd0faf3dcf238f2
// bf9fe1961bbabc9a bf8fe098f59153fb
// c0b82ff8a46d7370 bf802db22ff2aa0f
// bf441813cf8fabab bf341813bfb6f936
// 4089045a47b8c079 3f9c280a156e74fa
// c07bf9e98e980d9f bf99b7ef35cc6943
// c08c2be7a4bbcb87 bf95e0c610906c70
// 40811f081d4297a7 3f8ed0192469ab9d
// 405566d61ad0e231 bf2fa16a2ddc0f87
// c08101b9da2c269e bf620e7da00eef04
// c08a31e27a1d835e bf9b902660846cca
// c08aa54ac269a045 3f8b122c83fa140f
// 4067ce94cf8091a0 3fab550b4b59003c
// 408ba94277171cb2 bf9b7587f9907e18
// 407afeb0d39dbd69 bf9a60445a514d11
// 406dddc7ccb3faa2 bf9e769a173acd19
// 4038ec6f580d899a bfc0fd46aa49b504
// c05c9ef205a6bc34 bfa5afb56e35440d
// c088d420c5366bbf bf99b98ad4fe7585
// 40863da363414e13 3f985d919555ef49
// c089a72a0990087b 3f7199368698fd6e
// 406a4b15d13a7908 3fa679f8ba5cb521
// c065f8d41fe23f29 3fa8d20ff1b882af
// 40867b1e8721c6a9 3f964402b3732679
// 4077c8882b2981fe 3f8f5f5f44630483
// c07460d4022d36f8 3fa67afeacd04550
// c088043886eef719 bf9b67caff7f37e1
// c07211561b55e493 3f9ff11def45dd1f
// 408171fe0170a0b0 bfa10ba4a61a43c9
// c053422327c05870 bfb18e443119052d
// c0889826c6d467f5 bf959b1e601c2d6f
// 407baeb37ff376d8 3f9ca7d98fd97190
// 408d71fa5ceb8ced bf969e45be889878
// c0863526f46028ca 3f7134029bc35638
// c08a73755f0a012b 3f8dc8a4b299927a
// c06077c19b5ceab4 3fadb2c0cfc5834e
// c08db6fd3eb3d7af bf99b776d2521e50
// c084931eb03021f8 3f9b00e1c34ed83f
// c080c58c2d474550 bfa10212e12750d3
// 408ba17a7d743a80 bf8f95b690604057
// c0825a6354c0c00a bf9bb26eafc58cc9
// c0653ddadc66b0d6 3f9dd74b756ece05
// 40898f904c7db4f1 3f8366618784d80a
// 408c622861ab45a0 3f86e0358020de48
// n=101592 hash=81bd66c4c690c4a9
