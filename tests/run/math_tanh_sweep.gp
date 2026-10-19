// phase: 09
// math.Tanh on about 100k inputs: bit-identical to Go (hash of the result bits and 200 samples).

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
		y := math.Tanh(x)
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
// 4008000000000000 3fefd77d111a0b00
// 4040bfffffffffff 3ff0000000000000
// 5b65923ac30ec1f4 3ff0000000000000
// c024bfdbf760976a bfeffffffef439e7
// cc9f5dfc8a5c49d1 bff0000000000000
// 28faf9b36567288d 28faf9b36567288d
// 5c01af71ea368d0e 3ff0000000000000
// 52bd26a497b45c45 3ff0000000000000
// 57a8503628a803dd 3ff0000000000000
// 9146970ff4ce0971 9146970ff4ce0971
// f56a5815e599ff1f bff0000000000000
// 398243dbd8701b2c 398243dbd8701b2c
// 5ba4a48740cb04df 3ff0000000000000
// da25dad4e83140c7 bff0000000000000
// 9ba45dbea8b9d076 9ba45dbea8b9d076
// f762bc1ba29bdb35 bff0000000000000
// 662b00cc72923f60 3ff0000000000000
// 482a3a7945c16ef0 3ff0000000000000
// a8698fe6f1d1b6d7 a8698fe6f1d1b6d7
// e006469c13f2521e bff0000000000000
// e1dd8f8aae0c18d4 bff0000000000000
// e11d793466eefc2b bff0000000000000
// 4dcd1ad5a02370a4 3ff0000000000000
// 0deb45aa655f885e 0deb45aa655f885e
// f6250f700d64e16e bff0000000000000
// d8536e85cdee3a05 bff0000000000000
// 4e76b4f39cf95384 3ff0000000000000
// f88bbc29e3ba37f8 bff0000000000000
// 73ea5c3d1e048b48 3ff0000000000000
// 5c7b45e3f515c5bb 3ff0000000000000
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
// 4b335c2e086fec82 3ff0000000000000
// 304c21caf2bd8d77 304c21caf2bd8d77
// 3fe30bf498dd7094 3fe113833fc9c833
// 3fb5aac89bff8058 3fb59d93a672df2d
// 3feb34272e81d3a3 3fe61dbd2c143cfe
// 3fe816820fd48922 3fe4608b7687a082
// bfd0fe16d5fa012a bfd09aac15b80db5
// bfd7e00241eae292 bfd6d36cdd5d6ac9
// 3fd04afd572abb6e 3fcfe6518e74aa91
// bfd3fac47071ae3a bfd35ad7b2cb51f9
// 3fee624f10cecad8 3fe7aa732e17f384
// 3fea5ab522610f73 3fe5aa1054309b34
// bfefdb1385845498 bfe84f6b43072940
// bfeab98dff3b972c bfe5dd096175496f
// 3fe33b90cc1ec878 3fe13575a6971bae
// bfbd77b29473af38 bfbd568e46f2545b
// bfc0d52264f94548 bfc0bc77130b1e93
// bfaca7d38cbe0a80 bfaca02d2570b523
// 3fd89e3ee22ebdcc 3fd778c5771b7ca4
// 3fdb17eda830b6f4 3fd995491347dcde
// 3fdc928ac90fa0f4 3fdad06efadebda3
// bfe1262476c0cd54 bfdf5a3d33b19104
// bfd891a76658a7ee bfd76ddec3634ede
// 3fe75dada7e86de4 3fe3f1148441017f
// 3feb0ff8154e2eb2 3fe60ac7e921704a
// 3fdf06d705d2c98a 3fdcce012f4e9014
// bfe3f76f15ef5ae3 bfe1b9589edb9cb4
// bfd111a2c1ca78b2 bfd0ace5bfbf7c45
// 3fde48a32b9caa20 3fdc358a79cb91aa
// 3fef97a196284ef1 3fe832b855871f16
// 3fede2e4caecfaa7 3fe7700e8ae0ad64
// bfb8f8d3e2cf5e90 bfb8e4a0b9939977
// 3fb5e89f9a291078 3fb5daf88559a235
// bfed44e8558c6b55 bfe725c85f120949
// bfdb8a8438eb88d8 bfd9f54b2c846191
// bfef3966dac6296e bfe80a056da2a0b7
// 3fe177a452bbd5c2 3fdfd5840a56e510
// 3fd5fa7497c7977e 3fd5273c21a4571b
// bfd0f1b0f3280d4c bfd08f1b40134ab2
// 3fdce23d0b00be16 3fdb1201893d158f
// 3f9b4fb30606ac40 3f9b4e0abb99c5bb
// 3febc6ef604f80de 3fe669768cf3d1d0
// bfd1425a423a5810 bfd0da458e0783bf
// 3fc446f084e7184c 3fc41bf2c5d05d8a
// 3fc1dcd9164395c0 3fc1bf646cfa52b3
// 3fed04e125e49e2c 3fe70715a5e1d549
// bfd9c1bae0d1834a bfd873637a25de95
// 3fe9c5140195eae9 3fe55802a413eba5
// bfcbf4cde2506070 bfcb8522d3c8e6e1
// bfccc9fdcd96446c bfcc502e856f3586
// 3fe908bfc5faf522 3fe4eddb29de876d
// bfe7529c40e5d1d8 bfe3ea4e02f954d0
// 3fddc5abc9eb9c18 3fdbcba5bc57c483
// bfcd86ee3ae16e44 bfcd03a544a69039
// bfc3b6e0915c3048 bfc38f587038e2cc
// bfe8a5ac83066a77 bfe4b4b65742dfd8
// 3fdb59c5446ed18c 3fd9cc848c3c1a82
// bfd090d7606e9e78 bfd0349a4801ca68
// 3fe4d49c91a53133 3fe250604f68719a
// 3fe8ba0c569302d0 3fe4c0897ec72645
// 3fedd712a427ca3e 3fe76a924af30d09
// bfecd6ff4d77b8de bfe6f0df7a2494f4
// 418b404fc3162484 3ff0000000000000
// 41d17b6ce028e50d 3ff0000000000000
// 408655173bfff5d1 3ff0000000000000
// bfea65e0e6524d11 bfe5b01be9d7e0ba
// be22aed75aeb9d29 be22aed75aeb9d29
// c10cb224405f8efc bff0000000000000
// 3feb027bf746a552 3fe603afa7359e50
// c0609ecd76ff47a0 bff0000000000000
// c016e544370168a9 bfefffd33bfdf97d
// c0a93a015cc53ccd bff0000000000000
// 3e87c667839667c9 3e87c66783966783
// 3e11d98eda6dcb94 3e11d98eda6dcb94
// 416b3a4c3d7d9238 3ff0000000000000
// c1656317acd288fe bff0000000000000
// 3fee7cfa81808980 3fe7b680f3267ba0
// bea144763594e4ae bea144763594e301
// c06274676f3acee9 bff0000000000000
// bf87a2dd3a378270 bf87a29874bc2bce
// 3f0138f32b5b8be8 3f0138f32b40f0b2
// bf83230b4ef7d484 bf8322e6cedcc9f7
// 3e1422076fb38833 3e1422076fb38833
// 4017a2641f288700 3fefffe10f4699d0
// be1c28c11f1e1e4e be1c28c11f1e1e4e
// 40bf4cf742ff052e 3ff0000000000000
// c19940c466a73f26 bff0000000000000
// bfc84b098455eb88 bfc8016d5cd66305
// 3e73ccded0704d12 3e73ccded0704d08
// c19259d0233a17c8 bff0000000000000
// 3e387a09cc4c64a0 3e387a09cc4c64a0
// bf31ffcdf0faf143 bf31ffcde96330a1
// bf2dc2f4ae75ee28 bf2dc2f4a5e1283c
// 413f1b588b0dde46 3ff0000000000000
// c149b81eb24a22ae bff0000000000000
// c03a3ee2751880ae bff0000000000000
// c040825333089264 bff0000000000000
// c069e40733ff2e61 bff0000000000000
// 3fb3a5f40e85efb6 3fb39c19924e7dcb
// 4190e24e797072b6 3ff0000000000000
// 4158bac92af262e6 3ff0000000000000
// bf98a2773cb47237 bf98a13fda5e6d78
// c080f5aaa4e0668e bff0000000000000
// c0c65c914e3bbb1a bff0000000000000
// 3fd71f13102b33ce 3fd62a548a7d89e6
// 3e6924bd2c13f2a4 3e6924bd2c13f29f
// 4035402d0ef88402 3ff0000000000000
// be20d335ec1dfd0e be20d335ec1dfd0e
// bf2724e26d190e2c bf2724e2690ff6ea
// c1a6776763ade5a0 bff0000000000000
// bf73d3e6f8b4d4aa bf73d3dcd26004e5
// 41668c332abcb596 3ff0000000000000
// beec4e02cfb706e8 beec4e02cfafa532
// 40dd70b10b863620 3ff0000000000000
// 3eac444c99c0d1a1 3eac444c99c0ca47
// c178b7b6d077fabf bff0000000000000
// bf6b00e334851430 bf6b00dccba5221e
// befc5f503bf32744 befc5f503bd56a23
// c021901561e5f3bb bfefffffe6ae1d90
// bf9fe1961bbabc9a bf9fdef3474cf990
// c0b82ff8a46d7370 bff0000000000000
// bf441813cf8fabab bf441813a54dd027
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
// n=101592 hash=7b48eb05cc5e3728
