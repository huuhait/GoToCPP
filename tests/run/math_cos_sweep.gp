// phase: 09
// math.Cos on about 100k inputs: bit-identical to Go (hash of the result bits and 200 samples).

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
		y := math.Cos(x)
		h = mix(h, math.Float64bits(y))
		if i%500 == 0 {
			fmt.Printf("%016x %016x\n", math.Float64bits(x), math.Float64bits(y))
		}
	}
	fmt.Printf("n=%d hash=%016x\n", len(xs), h)
}

// Output:
// 0000000000000000 3ff0000000000000
// 40862e42fefa39f0 3fef3f7a97bc685d
// 4008000000000000 bfefae04be85e5d2
// 4040bfffffffffff bfdf6d1ce81ec485
// 5b65923ac30ec1f4 3fdd6e5ce00bbe77
// c024bfdbf760976a bfe29d800d2de9ac
// cc9f5dfc8a5c49d1 3feadf7bdc200e54
// 28faf9b36567288d 3ff0000000000000
// 5c01af71ea368d0e 3fe93d5e98374c21
// 52bd26a497b45c45 bfeddf61d9bd2a4f
// 57a8503628a803dd 3fd566b30f0ee4bb
// 9146970ff4ce0971 3ff0000000000000
// f56a5815e599ff1f bfe736fa56915f24
// 398243dbd8701b2c 3ff0000000000000
// 5ba4a48740cb04df bfe9e349cd3d599f
// da25dad4e83140c7 3fea9a15e4b13aaf
// 9ba45dbea8b9d076 3ff0000000000000
// f762bc1ba29bdb35 3fdeb20e3cae1c4b
// 662b00cc72923f60 3fe16f3066b4e448
// 482a3a7945c16ef0 3fee0a0b86bfcd57
// a8698fe6f1d1b6d7 3ff0000000000000
// e006469c13f2521e bfef821ae81cad6b
// e1dd8f8aae0c18d4 3fe72bee576222fc
// e11d793466eefc2b bfedceb54f8cabc2
// 4dcd1ad5a02370a4 bfe0f7c440a24a3c
// 0deb45aa655f885e 3ff0000000000000
// f6250f700d64e16e 3fc2116cda8bc5aa
// d8536e85cdee3a05 3fe7a70d4161addb
// 4e76b4f39cf95384 bfe27b6c6db2b5b9
// f88bbc29e3ba37f8 bfd45dedf5a84581
// 73ea5c3d1e048b48 3fc061d3ce3152d7
// 5c7b45e3f515c5bb 3fefd48f957e7b10
// ab2567ceba64e421 3ff0000000000000
// eb8d783bcea270b8 bfe59d50fa9301ec
// c3b9dee0ebf001a5 3fe22c724c4958dd
// 0e7855fba5612c1d 3ff0000000000000
// 836e10793726318c 3ff0000000000000
// 3b34dda55a15d732 3ff0000000000000
// 2f4e9ffba969cc56 3ff0000000000000
// 8b1e87ad9104ded4 3ff0000000000000
// 17c82fc123e13faa 3ff0000000000000
// f3be5d2a6268c34c bfe8399edc0f71cb
// 4b335c2e086fec82 bfd7b2e3ba73a1d5
// 304c21caf2bd8d77 3ff0000000000000
// 3fe30bf498dd7094 3fea7f3b1755526d
// 3fb5aac89bff8058 3fefe2ad0e15ce16
// 3feb34272e81d3a3 3fe51dddaa95a57f
// 3fe816820fd48922 3fe75aa157d40064
// bfd0fe16d5fa012a 3feee0f22cec2960
// bfd7e00241eae292 3fedcc9009c952a1
// 3fd04afd572abb6e 3feef7f8994065cd
// bfd3fac47071ae3a 3fee740c709cbe59
// 3fee624f10cecad8 3fe2a0799eca0ff9
// 3fea5ab522610f73 3fe5bf52581173ba
// bfefdb1385845498 3fe1692e82cf8c9e
// bfeab98dff3b972c 3fe5795f4bf9c0f5
// 3fe33b90cc1ec878 3fea646c18abaaaf
// bfbd77b29473af38 3fefc9c9ee94b536
// bfc0d52264f94548 3fefb944ae512f5e
// bfaca7d38cbe0a80 3feff32c4a45b6bb
// 3fd89e3ee22ebdcc 3feda96200caabb2
// 3fdb17eda830b6f4 3fed2cd75efe87cf
// 3fdc928ac90fa0f4 3fecdd1609bd8f85
// bfe1262476c0cd54 3feb8388908f3574
// bfd891a76658a7ee 3fedabbe3926d1ae
// 3fe75dada7e86de4 3fe7d774c07c7613
// 3feb0ff8154e2eb2 3fe539000a1a327a
// 3fdf06d705d2c98a 3fec500cb262d9c8
// bfe3f76f15ef5ae3 3fe9f86b10efdc3b
// bfd111a2c1ca78b2 3feede6038ab785a
// 3fde48a32b9caa20 3fec7be137082b9d
// 3fef97a196284ef1 3fe1a19e43a37735
// 3fede2e4caecfaa7 3fe3077fa0bc5fc8
// bfb8f8d3e2cf5e90 3fefd90e4f663a94
// 3fb5e89f9a291078 3fefe204d6997dfb
// bfed44e8558c6b55 3fe38599f83710c9
// bfdb8a8438eb88d8 3fed151f484fed44
// bfef3966dac6296e 3fe1eff44396c3a1
// 3fe177a452bbd5c2 3feb5991ff59a2da
// 3fd5fa7497c7977e 3fee21ae5bebf403
// bfd0f1b0f3280d4c 3feee292027a0c28
// 3fdce23d0b00be16 3feccbcb3a204e80
// 3f9b4fb30606ac40 3feffd1622be7d67
// 3febc6ef604f80de 3fe4aeb7e45b0880
// bfd1425a423a5810 3feed7ebd7cf8cd2
// 3fc446f084e7184c 3fef996cacb63c85
// 3fc1dcd9164395c0 3fefb05c47e63405
// 3fed04e125e49e2c 3fe3b82e6abfd9f3
// bfd9c1bae0d1834a 3fed717ecfe642b5
// 3fe9c5140195eae9 3fe62c26dd04ab08
// bfcbf4cde2506070 3fef3d63237cf0dd
// bfccc9fdcd96446c 3fef31ab81c53c90
// 3fe908bfc5faf522 3fe6b26ef454ecf5
// bfe7529c40e5d1d8 3fe7ded53a0002a0
// 3fddc5abc9eb9c18 3fec997d14ee106a
// bfcd86ee3ae16e44 3fef2700b9188fff
// bfc3b6e0915c3048 3fef9f072370111c
// bfe8a5ac83066a77 3fe6f7d8f16859df
// 3fdb59c5446ed18c 3fed1f41c452ef67
// bfd090d7606e9e78 3feeef1a7d452dad
// 3fe4d49c91a53133 3fe974c9bd0d0324
// 3fe8ba0c569302d0 3fe6e9a4885853a0
// 3fedd712a427ca3e 3fe310ff4613df7d
// bfecd6ff4d77b8de 3fe3dc3cb3a941f0
// 418b404fc3162484 3fe3629eb6ad32d3
// 41d17b6ce028e50d bfeddb35893a6aa9
// 408655173bfff5d1 bfb36ee2008813c7
// bfea65e0e6524d11 3fe5b71f2833f4f5
// be22aed75aeb9d29 3ff0000000000000
// c10cb224405f8efc bfeacd86b9d7d8bd
// 3feb027bf746a552 3fe54315d6f93b41
// c0609ecd76ff47a0 3fe0dd763a675d5c
// c016e544370168a9 3feb1fcab1ef097a
// c0a93a015cc53ccd 3feb342aac51dc7b
// 3e87c667839667c9 3fefffffffffff73
// 3e11d98eda6dcb94 3ff0000000000000
// 416b3a4c3d7d9238 3fe3d98dc9cdb988
// c1656317acd288fe bfd793545f84d9c0
// 3fee7cfa81808980 3fe28ac3996a3406
// bea144763594e4ae 3feffffffffffb57
// c06274676f3acee9 bfeffec8c534c5d8
// bf87a2dd3a378270 3fefff74558e4c78
// 3f0138f32b5b8be8 3fefffffffb5d8c2
// bf83230b4ef7d484 3fefffa4720d3436
// 3e1422076fb38833 3ff0000000000000
// 4017a2641f288700 3fedc7ead34f64b9
// be1c28c11f1e1e4e 3ff0000000000000
// 40bf4cf742ff052e bfd4f80790af8879
// c19940c466a73f26 3feda2415ddb786e
// bfc84b098455eb88 3fef6ce74af11bb3
// 3e73ccded0704d12 3fefffffffffffe7
// c19259d0233a17c8 3fe2daac1b1ad691
// 3e387a09cc4c64a0 3ff0000000000000
// bf31ffcdf0faf143 3fefffffebc070a3
// bf2dc2f4ae75ee28 3feffffff2290064
// 413f1b588b0dde46 3fe9e0ed4ced4732
// c149b81eb24a22ae bfe3fd51e4e7ec43
// c03a3ee2751880ae 3fdc4ab866c139af
// c040825333089264 bfa0186120133c63
// c069e40733ff2e61 3fef3bea6942c764
// 3fb3a5f40e85efb6 3fefe7e22b2b2255
// 4190e24e797072b6 3fe43b15b46bf0a0
// 4158bac92af262e6 bfdca9110193e5f6
// bf98a2773cb47237 3feffda12a079b4f
// c080f5aaa4e0668e bfe69621f53caee3
// c0c65c914e3bbb1a 3fd8e08c5c696a2e
// 3fd71f13102b33ce 3fedef30dc22dfe0
// 3e6924bd2c13f2a4 3feffffffffffff6
// 4035402d0ef88402 bfe79efcb469af22
// be20d335ec1dfd0e 3ff0000000000000
// bf2724e26d190e2c 3feffffff7a16800
// c1a6776763ade5a0 3fdc0eb77e59d758
// bf73d3e6f8b4d4aa 3fefffe76dc82d6b
// 41668c332abcb596 bfe3f85730bdb9ae
// beec4e02cfb706e8 3feffffffff37b5e
// 40dd70b10b863620 3feff8592fd70d1a
// 3eac444c99c0d1a1 3feffffffffff384
// c178b7b6d077fabf bfc1816407cf2811
// bf6b00e334851430 3feffff49b40f59e
// befc5f503bf32744 3fefffffffcdb02f
// c021901561e5f3bb bfe99a4511685898
// bf9fe1961bbabc9a 3feffc07abe095ce
// c0b82ff8a46d7370 bfefd064a2ab0b72
// bf441813cf8fabab 3fefffff9b0ea92b
// 4089045a47b8c079 bfeb13cd72fa5125
// c07bf9e98e980d9f 3fad63a3bbab1ce5
// c08c2be7a4bbcb87 bfefa5407493e19d
// 40811f081d4297a7 3fd4acef9e4efc8e
// 405566d61ad0e231 bfe6a9c659aac923
// c08101b9da2c269e bfe80d9be28418e5
// c08a31e27a1d835e bfeaf261bd58b0cb
// c08aa54ac269a045 bfd1be025903dd67
// 4067ce94cf8091a0 bfd84a8beaf38d69
// 408ba94277171cb2 3fe6ef90ee6563ae
// 407afeb0d39dbd69 bfaa021f0d00ac33
// 406dddc7ccb3faa2 3fef8a778d993142
// 4038ec6f580d899a 3fef4d72d4a47b73
// c05c9ef205a6bc34 3fc77eff8530e4de
// c088d420c5366bbf bfee81206a634294
// 40863da363414e13 bfc11b2efc0dbea9
// c089a72a0990087b bfe2e09439865493
// 406a4b15d13a7908 bfefaf832b804be5
// c065f8d41fe23f29 3fef9fef84597bd1
// 40867b1e8721c6a9 bfeffb096c0e0ef1
// 4077c8882b2981fe bfed77a12e2946f4
// c07460d4022d36f8 3fe9014d015f8611
// c088043886eef719 bfd967b79e119966
// c07211561b55e493 3feff2b2557b4263
// 408171fe0170a0b0 3fe27f5718b645c2
// c053422327c05870 bfb078031a174cef
// c0889826c6d467f5 bfa9937e0aabd729
// 407baeb37ff376d8 bfeff76e83baa2bc
// 408d71fa5ceb8ced 3fef273fdd5cd0a6
// c0863526f46028ca 3fe996bf89bb30f0
// c08a73755f0a012b bfcccbbe7ab3454d
// c06077c19b5ceab4 3fef5549946fb00f
// c08db6fd3eb3d7af bfe080fbbd946119
// c084931eb03021f8 3fccb905292e5284
// c080c58c2d474550 bfebc9a7ea5164da
// 408ba17a7d743a80 bfc6197d89c88c38
// c0825a6354c0c00a bfef7ca2d4d66029
// c0653ddadc66b0d6 3feeb102f4f4a207
// 40898f904c7db4f1 3fdb3a1c2b8c1846
// 408c622861ab45a0 bfee10e949abae94
// n=101592 hash=7818c7f932766c96
