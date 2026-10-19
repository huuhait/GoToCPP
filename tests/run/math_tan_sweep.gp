// phase: 09
// math.Tan on about 100k inputs: bit-identical to Go (hash of the result bits and 200 samples).

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
		y := math.Tan(x)
		h = mix(h, math.Float64bits(y))
		if i%500 == 0 {
			fmt.Printf("%016x %016x\n", math.Float64bits(x), math.Float64bits(y))
		}
	}
	fmt.Printf("n=%d hash=%016x\n", len(xs), h)
}

// Output:
// 0000000000000000 0000000000000000
// 40862e42fefa39f0 bfcc4034518509ec
// 4008000000000000 bfc23ef71254b86f
// 4040bfffffffffff bffc62aca32ebb07
// 5b65923ac30ec1f4 3ffee55ea6ab63d5
// c024bfdbf760976a bff65f2291c67cbe
// cc9f5dfc8a5c49d1 3fe4b0335b56c4ef
// 28faf9b36567288d 28faf9b36567288d
// 5c01af71ea368d0e 3fe8f0a360b224ac
// 52bd26a497b45c45 bfd8947c12de3ec5
// 57a8503628a803dd c0068bf555a5b7fb
// 9146970ff4ce0971 9146970ff4ce0971
// f56a5815e599ff1f 3fee5bf465c7302d
// 398243dbd8701b2c 398243dbd8701b2c
// 5ba4a48740cb04df 3fe7404d2ea5b903
// da25dad4e83140c7 3fe56523a3b6d768
// 9ba45dbea8b9d076 9ba45dbea8b9d076
// f762bc1ba29bdb35 bffd45c7268cfbe2
// 662b00cc72923f60 bff8a042bcfa5b32
// 482a3a7945c16ef0 3fd77f9170e1855a
// a8698fe6f1d1b6d7 a8698fe6f1d1b6d7
// e006469c13f2521e 3fc6b3fe359124cb
// e1dd8f8aae0c18d4 3fee7a7a17419bd8
// e11d793466eefc2b bfd8feabf81225f0
// 4dcd1ad5a02370a4 bff9955f10fe61ab
// 0deb45aa655f885e 0deb45aa655f885e
// f6250f700d64e16e c01c0db59fb9e37d
// d8536e85cdee3a05 bfed2907cb40af68
// 4e76b4f39cf95384 bff69d5e21bdc918
// f88bbc29e3ba37f8 c007d4f80a4e96ef
// 73ea5c3d1e048b48 c01eff1c2f35cf5b
// 5c7b45e3f515c5bb bfba77fe896129e2
// ab2567ceba64e421 ab2567ceba64e421
// eb8d783bcea270b8 bff177a97661f5e2
// c3b9dee0ebf001a5 bff7303a1db168c8
// 0e7855fba5612c1d 0e7855fba5612c1d
// 836e10793726318c 836e10793726318c
// 3b34dda55a15d732 3b34dda55a15d732
// 2f4e9ffba969cc56 2f4e9ffba969cc56
// 8b1e87ad9104ded4 8b1e87ad9104ded4
// 17c82fc123e13faa 17c82fc123e13faa
// f3be5d2a6268c34c bfeb9e4c4958e746
// 4b335c2e086fec82 40041197c17b0639
// 304c21caf2bd8d77 304c21caf2bd8d77
// 3fe30bf498dd7094 3fe5ab0618ea0e83
// 3fb5aac89bff8058 3fb5b810ffecb768
// 3feb34272e81d3a3 3ff237b477887d40
// 3fe816820fd48922 3fedf9c9d1cc8265
// bfd0fe16d5fa012a bfd16746907c8267
// bfd7e00241eae292 bfd90c4218f8f9fb
// 3fd04afd572abb6e 3fd0a77eb575e417
// bfd3fac47071ae3a bfd4a7aab06dab0a
// 3fee624f10cecad8 3ff659bbd1385b15
// 3fea5ab522610f73 3ff1454d5601bcc0
// bfefdb1385845498 bff8ac5749c25fb5
// bfeab98dff3b972c bff1ad49044564b9
// 3fe33b90cc1ec878 3fe5f0bd0848ad1d
// bfbd77b29473af38 bfbd99314b418e16
// bfc0d52264f94548 bfc0ee25b199f4c3
// bfaca7d38cbe0a80 bfacaf7eddf84467
// 3fd89e3ee22ebdcc 3fd9e8a54365d3d3
// 3fdb17eda830b6f4 3fdcd64a115aed10
// 3fdc928ac90fa0f4 3fdea2a52c5f1471
// bfe1262476c0cd54 bfe3010cf9456729
// bfd891a76658a7ee bfd9d9feabf85db8
// 3fe75dada7e86de4 3feca5e63231524f
// 3feb0ff8154e2eb2 3ff20e5dafcf8971
// 3fdf06d705d2c98a 3fe0dadebbab6e88
// bfe3f76f15ef5ae3 bfe70962a32ab244
// bfd111a2c1ca78b2 bfd17c463f5695ea
// 3fde48a32b9caa20 3fe0621e1d9c3fc5
// 3fef97a196284ef1 3ff83bda1c3e0f32
// 3fede2e4caecfaa7 3ff5a1b1ade5f11e
// bfb8f8d3e2cf5e90 bfb90d2e8fd8cf89
// 3fb5e89f9a291078 3fb5f65b38e4a530
// bfed44e8558c6b55 bff4c7f6db036c34
// bfdb8a8438eb88d8 bfdd609452fda10e
// bfef3966dac6296e bff7a34e018b70d6
// 3fe177a452bbd5c2 3fe36ff40408439f
// 3fd5fa7497c7977e 3fd6e2974309dfeb
// bfd0f1b0f3280d4c bfd159f6af8bc00e
// 3fdce23d0b00be16 3fdf04d4f5eaf948
// 3f9b4fb30606ac40 3f9b515b8e482e18
// 3febc6ef604f80de 3ff2e3c4fca67a8f
// bfd1425a423a5810 bfd1b0ab9ae94b64
// 3fc446f084e7184c 3fc472cd75cea781
// 3fc1dcd9164395c0 3fc1fac42736a49e
// 3fed04e125e49e2c 3ff472ceb717d5d0
// bfd9c1bae0d1834a bfdb3e6588f528bb
// 3fe9c5140195eae9 3ff0a66e920dea67
// bfcbf4cde2506070 bfcc68d119794e63
// bfccc9fdcd96446c bfcd48d51d95035f
// 3fe908bfc5faf522 3fefcdb084bac7e5
// bfe7529c40e5d1d8 bfec91fc0e4a4963
// 3fddc5abc9eb9c18 3fe00fce1a675536
// bfcd86ee3ae16e44 bfce0fed130f0f15
// bfc3b6e0915c3048 bfc3df2a972cfb03
// bfe8a5ac83066a77 bfef0b14f788c458
// 3fdb59c5446ed18c 3fdd25a49ff2a014
// bfd090d7606e9e78 bfd0f228f7c1bf61
// 3fe4d49c91a53133 3fe85fedec443058
// 3fe8ba0c569302d0 3fef32b9e9791651
// 3fedd712a427ca3e 3ff5910345b5808f
// bfecd6ff4d77b8de bff436d2f5f1e34f
// 418b404fc3162484 3ff50392398e3600
// 41d17b6ce028e50d bfd8af29a871eb91
// 408655173bfff5d1 402a454e9e03d5f5
// bfea65e0e6524d11 bff15169ce92c18f
// be22aed75aeb9d29 be22aed75aeb9d29
// c10cb224405f8efc bfe4df13e1c5aeb0
// 3feb027bf746a552 3ff1ff10b72d255e
// c0609ecd76ff47a0 bff9ccd0e8724943
// c016e544370168a9 3fe407e5cac3bc8c
// c0a93a015cc53ccd 3fe3d267d57c2f72
// 3e87c667839667c9 3e87c6678396680f
// 3e11d98eda6dcb94 3e11d98eda6dcb94
// 416b3a4c3d7d9238 bff43b448d5ef894
// c1656317acd288fe c00430b6edc32072
// 3fee7cfa81808980 3ff68144f7a8101d
// bea144763594e4ae bea144763594e65b
// c06274676f3acee9 3f91a4c75026f171
// bf87a2dd3a378270 bf87a3220193212b
// 3f0138f32b5b8be8 3f0138f32b76271e
// bf83230b4ef7d484 bf83232fcfb9f79d
// 3e1422076fb38833 3e1422076fb38833
// 4017a2641f288700 bfd9299d5b8eeecf
// be1c28c11f1e1e4e be1c28c11f1e1e4e
// 40bf4cf742ff052e c00711c46aff3860
// c19940c466a73f26 3fda14c684409bac
// bfc84b098455eb88 bfc896cca0a59dfa
// 3e73ccded0704d12 3e73ccded0704d12
// c19259d0233a17c8 3ff5f10fb135adb9
// 3e387a09cc4c64a0 3e387a09cc4c64a0
// bf31ffcdf0faf143 bf31ffcdf892b1ed
// bf2dc2f4ae75ee28 bf2dc2f4b70ab41a
// 413f1b588b0dde46 bfe7467088128613
// c149b81eb24a22ae 3ff4004c6bbb8900
// c03a3ee2751880ae c0003b98e8e42da7
// c040825333089264 403fcb81413605c2
// c069e40733ff2e61 3fcc85461fe8bf03
// 3fb3a5f40e85efb6 3fb3afda7519b876
// 4190e24e797072b6 3ff39bc4c7bf13dc
// 4158bac92af262e6 3ffff22782b407e8
// bf98a2773cb47237 bf98a3aec3f50f6d
// c080f5aaa4e0668e 3ff00ed7d2d29066
// c0c65c914e3bbb1a c002f67288b0427e
// 3fd71f13102b33ce 3fd82ec83111c0f6
// 3e6924bd2c13f2a4 3e6924bd2c13f2a4
// 4035402d0ef88402 bfed3ef5428deabb
// be20d335ec1dfd0e be20d335ec1dfd0e
// bf2724e26d190e2c bf2724e27122256f
// c1a6776763ade5a0 c00066b3768531d1
// bf73d3e6f8b4d4aa bf73d3f11f161cac
// 41668c332abcb596 3ff4087971436c9f
// beec4e02cfb706e8 beec4e02cfbe689e
// 40dd70b10b863620 3fa6250ec19adc9f
// 3eac444c99c0d1a1 3eac444c99c0d8fb
// c178b7b6d077fabf 401cf926e6575534
// bf6b00e334851430 bf6b00e99d68ad08
// befc5f503bf32744 befc5f503c10e465
// c021901561e5f3bb 3fe7fe41794c10b7
// bf9fe1961bbabc9a bf9fe439762d0341
// c0b82ff8a46d7370 3fbbb8562de59b79
// bf441813cf8fabab bf441813f9d18804
// 4089045a47b8c079 bfe4275540907c28
// c07bf9e98e980d9f c03164837e6cfe8a
// c08c2be7a4bbcb87 3fc3365d6c9e5b5a
// 40811f081d4297a7 40076f8d74501f59
// 405566d61ad0e231 3fefe61f8816b9ca
// c08101b9da2c269e bfec141dc4c14723
// c08a31e27a1d835e 3fe47ed11162c137
// c08aa54ac269a045 c00bba06583668d3
// 4067ce94cf8091a0 c003801928ab28f6
// 408ba94277171cb2 bfef222ef1ca6d64
// 407afeb0d39dbd69 4033a91e4c384522
// 406dddc7ccb3faa2 3fc5eb49d5b690df
// 4038ec6f580d899a bfcb2bbac8584d8e
// c05c9ef205a6bc34 c0156babd668f306
// c088d420c5366bbf 3fd449062c9f0c83
// 40863da363414e13 c01da986461718de
// c089a72a0990087b bff5e68aa6247750
// 406a4b15d13a7908 bfc213928a491f65
// c065f8d41fe23f29 3fc3c6dfbe0d8680
// 40867b1e8721c6a9 bfa1d4a7df86f6f1
// 4077c8882b2981fe 3fdb19581182e57c
// c07460d4022d36f8 3fe98dfd209537c6
// c088043886eef719 40027f5f9451d8a7
// c07211561b55e493 bfad36f270ca39c0
// 408171fe0170a0b0 bff6962e49b38b42
// c053422327c05870 402f0652871ab042
// c0889826c6d467f5 4033fe60c61e6852
// 407baeb37ff376d8 bfa76f8b3f40fabd
// 408d71fa5ceb8ced bfce0b5f33e9fe6d
// c0863526f46028ca bfe8076d4bae06e9
// c08a73755f0a012b c011531584b446de
// c06077c19b5ceab4 3fca8c5347f1cafc
// c08db6fd3eb3d7af 3ffa942aeae2c7db
// c084931eb03021f8 40115efa98ea3099
// c080c58c2d474550 3fe2467281bac630
// 408ba17a7d743a80 4016d1e8574d601f
// c0825a6354c0c00a 3fc733eb9193845f
// c0653ddadc66b0d6 bfd2e30e9f633c36
// 40898f904c7db4f1 400104b0542918ed
// 408c622861ab45a0 3fd7523e55e471fb
// n=101592 hash=e35cab925166af1b
