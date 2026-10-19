// phase: 09
// math.Sin on about 100k inputs: bit-identical to Go (hash of the result bits and 200 samples).

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
		y := math.Sin(x)
		h = mix(h, math.Float64bits(y))
		if i%500 == 0 {
			fmt.Printf("%016x %016x\n", math.Float64bits(x), math.Float64bits(y))
		}
	}
	fmt.Printf("n=%d hash=%016x\n", len(xs), h)
}

// Output:
// 0000000000000000 0000000000000000
// 40862e42fefa39f0 bfcb963d50b6228a
// 4008000000000000 3fc210386db6d55b
// 4040bfffffffffff 3febe060fe5ed998
// 5b65923ac30ec1f4 3fec6a6be8294c18
// c024bfdbf760976a 3fea073f72b7c494
// cc9f5dfc8a5c49d1 3fe15fa5b3b0864f
// 28faf9b36567288d 28faf9b36567288d
// 5c01af71ea368d0e 3fe3abd4147d8d42
// 52bd26a497b45c45 3fd6f2265b1535b6
// 57a8503628a803dd bfee286a8f6c62e6
// 9146970ff4ce0971 9146970ff4ce0971
// f56a5815e599ff1f bfe60640561951a7
// 398243dbd8701b2c 398243dbd8701b2c
// 5ba4a48740cb04df bfe2cf620fc4f4b2
// da25dad4e83140c7 3fe1c93253ceeef9
// 9ba45dbea8b9d076 9ba45dbea8b9d076
// f762bc1ba29bdb35 bfec144bddf3d7ef
// 662b00cc72923f60 bfead56936a5546b
// 482a3a7945c16ef0 3fd60ef8ae13bb8a
// a8698fe6f1d1b6d7 a8698fe6f1d1b6d7
// e006469c13f2521e bfc65aac938a105c
// e1dd8f8aae0c18d4 3fe611df56ece97c
// e11d793466eefc2b 3fd74840ea7cb341
// 4dcd1ad5a02370a4 3feb218ac9ba4434
// 0deb45aa655f885e 0deb45aa655f885e
// f6250f700d64e16e bfefadf9c064a7e3
// d8536e85cdee3a05 bfe58db7cf02bcda
// 4e76b4f39cf95384 3fea1f7ceae0f1cd
// f88bbc29e3ba37f8 3fee561e5f1ff7f5
// 73ea5c3d1e048b48 bfefbca11dcbccca
// 5c7b45e3f515c5bb bfba54104f4511a7
// ab2567ceba64e421 ab2567ceba64e421
// eb8d783bcea270b8 3fe798cd0217b937
// c3b9dee0ebf001a5 bfea56aba766b231
// 0e7855fba5612c1d 0e7855fba5612c1d
// 836e10793726318c 836e10793726318c
// 3b34dda55a15d732 3b34dda55a15d732
// 2f4e9ffba969cc56 2f4e9ffba969cc56
// 8b1e87ad9104ded4 8b1e87ad9104ded4
// 17c82fc123e13faa 17c82fc123e13faa
// f3be5d2a6268c34c 3fe4e8744a6b9eda
// 4b335c2e086fec82 bfedb9ab81dfe830
// 304c21caf2bd8d77 304c21caf2bd8d77
// 3fe30bf498dd7094 3fe1f11bb790fd1d
// 3fb5aac89bff8058 3fb5a429e4a8052f
// 3feb34272e81d3a3 3fe80b1e37a2bf7a
// 3fe816820fd48922 3fe5e06eb96e285d
// bfd0fe16d5fa012a bfd0cb28cadc37af
// bfd7e00241eae292 bfd7533acad34e69
// 3fd04afd572abb6e 3fd01e1505ff83ec
// bfd3fac47071ae3a bfd3a817d7b661b5
// 3fee624f10cecad8 3fea051e85f85a25
// 3fea5ab522610f73 3fe77979ae515528
// bfefdb1385845498 bfead94f7465d90c
// bfeab98dff3b972c bfe7b987969d1e8b
// 3fe33b90cc1ec878 3fe2187402dd2cf9
// bfbd77b29473af38 bfbd670cc572c0de
// bfc0d52264f94548 bfc0c8b9a441e168
// bfaca7d38cbe0a80 bfaca3ff4650dc59
// 3fd89e3ee22ebdcc 3fd803f9c6fe6fbc
// 3fdb17eda830b6f4 3fda4a9bdaeec159
// 3fdc928ac90fa0f4 3fdba1f8e4b26984
// bfe1262476c0cd54 bfe0570059bfed62
// bfd891a76658a7ee bfd7f84d6143d31c
// 3fe75dada7e86de4 3fe558209c69f63a
// 3feb0ff8154e2eb2 3fe7f32e2fe9429b
// 3fdf06d705d2c98a 3fddd3598228773d
// bfe3f76f15ef5ae3 bfe2b22adf7d15b7
// bfd111a2c1ca78b2 bfd0de0496828008
// 3fde48a32b9caa20 3fdd2a8d97763665
// 3fef97a196284ef1 3feab461b63a2f24
// 3fede2e4caecfaa7 3fe9ba2648ff156e
// bfb8f8d3e2cf5e90 bfb8eeb1b306710f
// 3fb5e89f9a291078 3fb5e1c795739059
// bfed44e8558c6b55 bfe95afb529c22b3
// bfdb8a8438eb88d8 bfdab2ea847eaec6
// bfef3966dac6296e bfea80031d1b64fc
// 3fe177a452bbd5c2 3fe09cddf076751a
// 3fd5fa7497c7977e 3fd58c842ca9fd50
// bfd0f1b0f3280d4c bfd0bf31ce981cf6
// 3fdce23d0b00be16 3fdbe9c5fd24afb2
// 3f9b4fb30606ac40 3f9b4eded34ad88a
// 3febc6ef604f80de 3fe86afc54c1e5e2
// bfd1425a423a5810 bfd10cfe42ed7d6a
// 3fc446f084e7184c 3fc43141624e67e6
// 3fc1dcd9164395c0 3fc1ce05162e4793
// 3fed04e125e49e2c 3fe933b91525f8f2
// bfd9c1bae0d1834a bfd9112c515943f9
// 3fe9c5140195eae9 3fe712ca2a2b164b
// bfcbf4cde2506070 bfcbbc0a630e8ab2
// bfccc9fdcd96446c bfcc8c02eef29871
// 3fe908bfc5faf522 3fe68ebfc7064a0b
// bfe7529c40e5d1d8 bfe54fe04e66e26d
// 3fddc5abc9eb9c18 3fdcb5bd618e3cba
// bfcd86ee3ae16e44 bfcd4411c148f6c5
// bfc3b6e0915c3048 bfc3a2f28ce5ba07
// bfe8a5ac83066a77 bfe6480e6902e69f
// 3fdb59c5446ed18c 3fda86958fb753da
// bfd090d7606e9e78 bfd061a51157ee26
// 3fe4d49c91a53133 3fe363e7499caf98
// 3fe8ba0c569302d0 3fe656a99315b8b6
// 3fedd712a427ca3e 3fe9b31cfba69ce9
// bfecd6ff4d77b8de bfe91759095744b0
// 418b404fc3162484 3fe975c3f7297833
// 41d17b6ce028e50d 3fd707d5d1b80237
// 408655173bfff5d1 bfefe85cebf8cdfc
// bfea65e0e6524d11 bfe7810fc28c11f6
// be22aed75aeb9d29 be22aed75aeb9d29
// c10cb224405f8efc 3fe17b4d1e28acde
// 3feb027bf746a552 3fe7ea3a96fc122f
// c0609ecd76ff47a0 bfeb31ec92aec879
// c016e544370168a9 3fe0fa906aed749c
// c0a93a015cc53ccd 3fe0d9d7f3c19d4d
// 3e87c667839667c9 3e87c667839667a6
// 3e11d98eda6dcb94 3e11d98eda6dcb94
// 416b3a4c3d7d9238 bfe919788316ec38
// c1656317acd288fe 3fedbff0feba326e
// 3fee7cfa81808980 3fea149ba8b72b11
// bea144763594e4ae bea144763594e3d7
// c06274676f3acee9 bf91a41bb64b27d4
// bf87a2dd3a378270 bf87a2bad710c8fe
// 3f0138f32b5b8be8 3f0138f32b4e3e4d
// bf83230b4ef7d484 bf8322f90ec5c23f
// 3e1422076fb38833 3e1422076fb38833
// 4017a2641f288700 bfd76aea0c026a52
// be1c28c11f1e1e4e be1c28c11f1e1e4e
// 40bf4cf742ff052e 3fee3bd3d29c680e
// c19940c466a73f26 3fd827125b8a9211
// bfc84b098455eb88 bfc825c4e869c5dd
// 3e73ccded0704d12 3e73ccded0704d0d
// c19259d0233a17c8 3fe9db1221bf2843
// 3e387a09cc4c64a0 3e387a09cc4c64a0
// bf31ffcdf0faf143 bf31ffcded2f10f0
// bf2dc2f4ae75ee28 bf2dc2f4aa2b8b31
// 413f1b588b0dde46 bfe2d2a197ac4af0
// c149b81eb24a22ae bfe8fd05d7ff4623
// c03a3ee2751880ae bfecb41a5062ddaf
// c040825333089264 bfeffbf38494ae4e
// c069e40733ff2e61 3fcbd6829495ea30
// 3fb3a5f40e85efb6 3fb3a10436f3ee5e
// 4190e24e797072b6 3fe8cb1ef8de5df6
// 4158bac92af262e6 bfec9caa6d400258
// bf98a2773cb47237 bf98a1db8376ab22
// c080f5aaa4e0668e bfe6ab15fdf74f63
// c0c65c914e3bbb1a bfed7bcc6a72351f
// 3fd71f13102b33ce 3fd69f27c02ed848
// 3e6924bd2c13f2a4 3e6924bd2c13f2a1
// 4035402d0ef88402 3fe5968e177cc3c6
// be20d335ec1dfd0e be20d335ec1dfd0e
// bf2724e26d190e2c bf2724e26b14828b
// c1a6776763ade5a0 bfecc2d0049a4496
// bf73d3e6f8b4d4aa bf73d3e1e587b27c
// 41668c332abcb596 bfe90100bbf6f6d8
// beec4e02cfb706e8 beec4e02cfb3560d
// 40dd70b10b863620 3fa61fc33616c017
// 3eac444c99c0d1a1 3eac444c99c0cdf4
// c178b7b6d077fabf bfefb30741b52b40
// bf6b00e334851430 bf6b00e000144eac
// befc5f503bf32744 befc5f503be448b3
// c021901561e5f3bb bfe3324e8b25a836
// bf9fe1961bbabc9a bf9fe04494363568
// c0b82ff8a46d7370 bfbb8f18d99771bc
// bf441813cf8fabab bf441813ba6ebdbb
// 4089045a47b8c079 3fe10da8ad7c2a43
// c07bf9e98e980d9f bfeff27e3ec02a33
// c08c2be7a4bbcb87 bfc2ffe185aa2e1f
// 40811f081d4297a7 3fee48befc38c488
// 405566d61ad0e231 bfe69772bd5bb888
// c08101b9da2c269e 3fe51b0747db0ff8
// c08a31e27a1d835e bfe1424784e3454f
// c08aa54ac269a045 3feebeebda93965b
// 4067ce94cf8091a0 3fed9b00b891ac28
// 408ba94277171cb2 bfe65094969d2d2a
// 407afeb0d39dbd69 bfeff56c8749e743
// 406dddc7ccb3faa2 3fc59ac8196abe74
// 4038ec6f580d899a bfca941fabde67b5
// c05c9ef205a6bc34 bfef74cd03c27acc
// c088d420c5366bbf bfd35650b72ffa18
// 40863da363414e13 3fefb683d7b858ea
// c089a72a0990087b 3fe9d6c2733eb20d
// 406a4b15d13a7908 3fc1e61b180dc27c
// c065f8d41fe23f29 3fc38b80eef8298a
// 40867b1e8721c6a9 3fa1d1e3e550cca8
// 4077c8882b2981fe bfd8f4468dfa3a00
// c07460d4022d36f8 3fe3f7f7b01f6724
// c088043886eef719 bfed5eed969e0a56
// c07211561b55e493 bfad2acd3667f076
// 408171fe0170a0b0 bfea1cb715d142c3
// c053422327c05870 bfefef07f5d6e621
// c0889826c6d467f5 bfeff5c5cf3b8a93
// 407baeb37ff376d8 3fa76944cfff2b78
// 408d71fa5ceb8ced bfcd3fde0c8b0ff3
// c0863526f46028ca bfe3370007336cef
// c08a73755f0a012b 3fef2e0208ba53c3
// c06077c19b5ceab4 3fc9feb2762cfd04
// c08db6fd3eb3d7af bfeb6a6e7187d9af
// c084931eb03021f8 3fef2f165785e576
// c080c58c2d474550 bfdfbd3604575557
// 408ba17a7d743a80 bfef84fa7f7d545e
// c0825a6354c0c00a bfc6d4ab7706fe6d
// c0653ddadc66b0d6 bfd21d575d1341bf
// 40898f904c7db4f1 3fecf5b883bc9f0d
// 408c622861ab45a0 bfd5e96d9437bb68
// n=101592 hash=fc135aad11248c16
