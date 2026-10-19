// phase: 09
// math.Expm1 on about 100k inputs: bit-identical to Go (hash of the result bits and 200 samples).

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
		y := math.Expm1(x)
		h = mix(h, math.Float64bits(y))
		if i%500 == 0 {
			fmt.Printf("%016x %016x\n", math.Float64bits(x), math.Float64bits(y))
		}
	}
	fmt.Printf("n=%d hash=%016x\n", len(xs), h)
}

// Output:
// 0000000000000000 0000000000000000
// 40862e42fefa39f0 7ff0000000000000
// 4008000000000000 403315e5bf6fb106
// 4040bfffffffffff 42f41dbd6e3e0606
// 5b65923ac30ec1f4 7ff0000000000000
// c024bfdbf760976a bfefffbe8b79024b
// cc9f5dfc8a5c49d1 bff0000000000000
// 28faf9b36567288d 28faf9b36567288d
// 5c01af71ea368d0e 7ff0000000000000
// 52bd26a497b45c45 7ff0000000000000
// 57a8503628a803dd 7ff0000000000000
// 9146970ff4ce0971 9146970ff4ce0971
// f56a5815e599ff1f bff0000000000000
// 398243dbd8701b2c 398243dbd8701b2c
// 5ba4a48740cb04df 7ff0000000000000
// da25dad4e83140c7 bff0000000000000
// 9ba45dbea8b9d076 9ba45dbea8b9d076
// f762bc1ba29bdb35 bff0000000000000
// 662b00cc72923f60 7ff0000000000000
// 482a3a7945c16ef0 7ff0000000000000
// a8698fe6f1d1b6d7 a8698fe6f1d1b6d7
// e006469c13f2521e bff0000000000000
// e1dd8f8aae0c18d4 bff0000000000000
// e11d793466eefc2b bff0000000000000
// 4dcd1ad5a02370a4 7ff0000000000000
// 0deb45aa655f885e 0deb45aa655f885e
// f6250f700d64e16e bff0000000000000
// d8536e85cdee3a05 bff0000000000000
// 4e76b4f39cf95384 7ff0000000000000
// f88bbc29e3ba37f8 bff0000000000000
// 73ea5c3d1e048b48 7ff0000000000000
// 5c7b45e3f515c5bb 7ff0000000000000
// ab2567ceba64e421 ab2567ceba64e421
// eb8d783bcea270b8 bff0000000000000
// c3b9dee0ebf001a5 bff0000000000000
// 0e7855fba5612c1d 0e7855fba5612c1d
// 836e10793726318c 836e10793726318c
// 3b34dda55a15d732 3b34dda55a15d732
// 2f4e9ffba969cc56 2f4e9ffba969cc56
// 8b1e87ad9104ded4 8b1e87ad9104ded4
// 17c82fc123e13faa 17c82fc123e13faa
// f3be5d2a6268c34c bff0000000000000
// 4b335c2e086fec82 7ff0000000000000
// 304c21caf2bd8d77 304c21caf2bd8d77
// 3fe30bf498dd7094 3fea0775c2d8cc51
// 3fb5aac89bff8058 3fb69c47d7f1cd06
// 3feb34272e81d3a3 3ff5704ef82fd19c
// 3fe816820fd48922 3ff1f716f04151d8
// bfd0fe16d5fa012a bfcdd8f8fd01f894
// bfd7e00241eae292 bfd3ed719b93d0e8
// 3fd04afd572abb6e 3fd28dfb4fd70c22
// bfd3fac47071ae3a bfd1296170e13b5d
// 3fee624f10cecad8 3ff959c760e679db
// 3fea5ab522610f73 3ff47540b426fb4f
// bfefdb1385845498 bfe42cb7ac5318ee
// bfeab98dff3b972c bfe21e3c6413b3b8
// 3fe33b90cc1ec878 3fea5e0c530cb327
// bfbd77b29473af38 bfbbd5b80c73f63f
// bfc0d52264f94548 bfbf8ba49ed7fbe7
// bfaca7d38cbe0a80 bfabde515675d331
// 3fd89e3ee22ebdcc 3fde05f0de562fde
// 3fdb17eda830b6f4 3fe0dd909937613a
// 3fdc928ac90fa0f4 3fe20202e37e40a8
// bfe1262476c0cd54 bfda8d15a8518212
// bfd891a76658a7ee bfd4671d7b8a82d5
// 3fe75dada7e86de4 3ff1351b0c5d6d31
// 3feb0ff8154e2eb2 3ff546116b1dd313
// 3fdf06d705d2c98a 3fe3f67b1625b544
// bfe3f76f15ef5ae3 bfddb51a0e7dec41
// bfd111a2c1ca78b2 bfcdf6ee9610de3f
// 3fde48a32b9caa20 3fe35cf1d615639a
// 3fef97a196284ef1 3ffaf121585e2830
// 3fede2e4caecfaa7 3ff8b667d6a31ebd
// bfb8f8d3e2cf5e90 bfb7caec6f9be3c3
// 3fb5e89f9a291078 3fb6df9d31ee1b7a
// bfed44e8558c6b55 bfe32ddb7185debb
// bfdb8a8438eb88d8 bfd66191a241fb26
// bfef3966dac6296e bfe3f060ed06e727
// 3fe177a452bbd5c2 3fe73c129242537c
// 3fd5fa7497c7977e 3fda39535a5da2aa
// bfd0f1b0f3280d4c bfcdc5f392a7d136
// 3fdce23d0b00be16 3fe2406f7e1317f7
// 3f9b4fb30606ac40 3f9badc5b943f0d6
// 3febc6ef604f80de 3ff61d95d0809df9
// bfd1425a423a5810 bfce41721a0e40f2
// 3fc446f084e7184c 3fc5f8b325bbdc35
// 3fc1dcd9164395c0 3fc32b4c7df4545e
// 3fed04e125e49e2c 3ff79fbcd21333e5
// bfd9c1bae0d1834a bfd53457e8436138
// 3fe9c5140195eae9 3ff3cc5351552630
// bfcbf4cde2506070 bfc91d2cba52b721
// bfccc9fdcd96446c bfc9c7fa4dcaa5e6
// 3fe908bfc5faf522 3ff2fc0be9d8deda
// bfe7529c40e5d1d8 bfe08f98c8dd17f5
// 3fddc5abc9eb9c18 3fe2f441c36d0ad3
// bfcd86ee3ae16e44 bfca5e6d714f6036
// bfc3b6e0915c3048 bfc2456de324a6dd
// bfe8a5ac83066a77 bfe12fd890a550e8
// 3fdb59c5446ed18c 3fe10ff0249d9c08
// bfd090d7606e9e78 bfcd30dde0c4e223
// 3fe4d49c91a53133 3fed5b14bf239787
// 3fe8ba0c569302d0 3ff2a66ad8aa3324
// 3fedd712a427ca3e 3ff8a7608b5a246b
// bfecd6ff4d77b8de bfe3018650d79ce3
// 418b404fc3162484 7ff0000000000000
// 41d17b6ce028e50d 7ff0000000000000
// 408655173bfff5d1 7ff0000000000000
// bfea65e0e6524d11 bfe1f9c01a4b783b
// be22aed75aeb9d29 be22aed75a9459bc
// c10cb224405f8efc bff0000000000000
// 3feb027bf746a552 3ff5365fb051755c
// c0609ecd76ff47a0 bff0000000000000
// c016e544370168a9 bfefe53cac119146
// c0a93a015cc53ccd bff0000000000000
// 3e87c667839667c9 3e87c667a6ea6dcb
// 3e11d98eda6dcb94 3e11d98eda959f50
// 416b3a4c3d7d9238 7ff0000000000000
// c1656317acd288fe bff0000000000000
// 3fee7cfa81808980 3ff97c4c3cb81a80
// bea144763594e4ae bea14475eb0a64fc
// c06274676f3acee9 bff0000000000000
// bf87a2dd3a378270 bf878014ce42b910
// 3f0138f32b5b8be8 3f013905b5384a4c
// bf83230b4ef7d484 bf830c39fcad2aa6
// 3e1422076fb38833 3e1422076fe632e9
// 4017a2641f288700 4076f2f50d05d3b2
// be1c28c11f1e1e4e be1c28c11ebb0037
// 40bf4cf742ff052e 7ff0000000000000
// c19940c466a73f26 bff0000000000000
// bfc84b098455eb88 bfc62082bc5ad257
// 3e73ccded0704d12 3e73ccdedcb0b54e
// c19259d0233a17c8 bff0000000000000
// 3e387a09cc4c64a0 3e387a09cd77f2a2
// bf31ffcdf0faf143 bf31ff2bf84bc9f7
// bf2dc2f4ae75ee28 bf2dc21742c66eea
// 413f1b588b0dde46 7ff0000000000000
// c149b81eb24a22ae bff0000000000000
// c03a3ee2751880ae bfefffffffff7364
// c040825333089264 bfefffffffffffd7
// c069e40733ff2e61 bff0000000000000
// 3fb3a5f40e85efb6 3fb46c03cf3217e9
// 4190e24e797072b6 7ff0000000000000
// 4158bac92af262e6 7ff0000000000000
// bf98a2773cb47237 bf98573659aa8dd3
// c080f5aaa4e0668e bff0000000000000
// c0c65c914e3bbb1a bff0000000000000
// 3fd71f13102b33ce 3fdbd98cffa10da7
// 3e6924bd2c13f2a4 3e6924bd35f4bb88
// 4035402d0ef88402 41d9403966829352
// be20d335ec1dfd0e be20d335ebd737ce
// bf2724e26d190e2c bf27245c859d8f9a
// c1a6776763ade5a0 bff0000000000000
// bf73d3e6f8b4d4aa bf73c7a2ecd4758d
// 41668c332abcb596 7ff0000000000000
// beec4e02cfb706e8 beec4df64b192b28
// 40dd70b10b863620 7ff0000000000000
// 3eac444c99c0d1a1 3eac444d618193e9
// c178b7b6d077fabf bff0000000000000
// bf6b00e334851430 bf6af581a89187bb
// befc5f503bf32744 befc5f371419b7b7
// c021901561e5f3bb bfeffebdf59a8e60
// bf9fe1961bbabc9a bf9f63d7e686b2c3
// c0b82ff8a46d7370 bff0000000000000
// bf441813cf8fabab bf4416801f539e7b
// 4089045a47b8c079 7ff0000000000000
// c07bf9e98e980d9f bff0000000000000
// c08c2be7a4bbcb87 bff0000000000000
// 40811f081d4297a7 715570bfaa4c6e2b
// 405566d61ad0e231 47a6b2d7b845d83a
// c08101b9da2c269e bff0000000000000
// c08a31e27a1d835e bff0000000000000
// c08aa54ac269a045 bff0000000000000
// 4067ce94cf8091a0 511b4617dfac99c7
// 408ba94277171cb2 7ff0000000000000
// 407afeb0d39dbd69 66e17671ed25cf46
// 406dddc7ccb3faa2 557a10ac0747620f
// 4038ec6f580d899a 422f101481584422
// c05c9ef205a6bc34 bff0000000000000
// c088d420c5366bbf bff0000000000000
// 40863da363414e13 7ff0000000000000
// c089a72a0990087b bff0000000000000
// 406a4b15d13a7908 52e618a171efae90
// c065f8d41fe23f29 bff0000000000000
// 40867b1e8721c6a9 7ff0000000000000
// 4077c8882b2981fe 623fdabf44631c00
// c07460d4022d36f8 bff0000000000000
// c088043886eef719 bff0000000000000
// c07211561b55e493 bff0000000000000
// 408171fe0170a0b0 7244ddc4a87ad201
// c053422327c05870 bff0000000000000
// c0889826c6d467f5 bff0000000000000
// 407baeb37ff376d8 67dfedcaa47ef3d5
// 408d71fa5ceb8ced 7ff0000000000000
// c0863526f46028ca bff0000000000000
// c08a73755f0a012b bff0000000000000
// c06077c19b5ceab4 bff0000000000000
// c08db6fd3eb3d7af bff0000000000000
// c084931eb03021f8 bff0000000000000
// c080c58c2d474550 bff0000000000000
// 408ba17a7d743a80 7ff0000000000000
// c0825a6354c0c00a bff0000000000000
// c0653ddadc66b0d6 bff0000000000000
// 40898f904c7db4f1 7ff0000000000000
// 408c622861ab45a0 7ff0000000000000
// n=101592 hash=61aaf706a7c4f590
