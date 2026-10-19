// phase: 09
// math.Asinh on about 100k inputs: bit-identical to Go (hash of the result bits and 200 samples).

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
		y := math.Asinh(x)
		h = mix(h, math.Float64bits(y))
		if i%500 == 0 {
			fmt.Printf("%016x %016x\n", math.Float64bits(x), math.Float64bits(y))
		}
	}
	fmt.Printf("n=%d hash=%016x\n", len(xs), h)
}

// Output:
// 0000000000000000 0000000000000000
// 40862e42fefa39f0 401d084d162780df
// 4008000000000000 3ffd185b507edc0e
// 4040bfffffffffff 4010d1d551c9b909
// 5b65923ac30ec1f4 40731489570116ab
// c024bfdbf760976a c00847573d34a0f6
// cc9f5dfc8a5c49d1 c061ac39c01b1302
// 28faf9b36567288d 28faf9b36567288d
// 5c01af71ea368d0e 4073804315b4cb99
// 52bd26a497b45c45 406a279724a6a978
// 57a8503628a803dd 40707d07a31271d3
// 9146970ff4ce0971 9146970ff4ce0971
// f56a5815e599ff1f c0828ea93ffc78c5
// 398243dbd8701b2c 398243dbd8701b2c
// 5ba4a48740cb04df 40734031a56c247b
// da25dad4e83140c7 c07236f03919020f
// 9ba45dbea8b9d076 9ba45dbea8b9d076
// f762bc1ba29bdb35 c0833d61428a0a45
// 662b00cc72923f60 407a8babc75ed448
// 482a3a7945c16ef0 4056ff56f4d8b203
// a8698fe6f1d1b6d7 a8698fe6f1d1b6d7
// e006469c13f2521e c07649bcad6cd775
// e1dd8f8aae0c18d4 c0778fe26989f831
// e11d793466eefc2b c0770ac0bbeab2dc
// 4dcd1ad5a02370a4 40634f437eec6335
// 0deb45aa655f885e 0deb45aa655f885e
// f6250f700d64e16e c082cf69951a6f6f
// d8536e85cdee3a05 c070f37017782bfa
// 4e76b4f39cf95384 40643b4ecff5e15a
// f88bbc29e3ba37f8 c083a454e5106514
// 73ea5c3d1e048b48 40820994f0c85bd6
// 5c7b45e3f515c5bb 4073d4d33b6dc4e3
// ab2567ceba64e421 ab2567ceba64e421
// eb8d783bcea270b8 c07e46d70f1f3bc9
// c3b9dee0ebf001a5 c045619982695576
// 0e7855fba5612c1d 0e7855fba5612c1d
// 836e10793726318c 836e10793726318c
// 3b34dda55a15d732 3b34dda55a15d732
// 2f4e9ffba969cc56 2f4e9ffba969cc56
// 8b1e87ad9104ded4 8b1e87ad9104ded4
// 17c82fc123e13faa 17c82fc123e13faa
// f3be5d2a6268c34c c081fa13d2bbd37c
// 4b335c2e086fec82 405f699ded0f97b2
// 304c21caf2bd8d77 304c21caf2bd8d77
// 3fe30bf498dd7094 3fe2122048cf7b04
// 3fb5aac89bff8058 3fb5a42eba548859
// 3feb34272e81d3a3 3fe8aeb4050d5ebf
// 3fe816820fd48922 3fe640419b2976d8
// bfd0fe16d5fa012a bfd0cc8930efcf59
// bfd7e00241eae292 bfd75a73b88f0326
// 3fd04afd572abb6e 3fd01f33a587ef14
// bfd3fac47071ae3a bfd3ab22689a55ac
// 3fee624f10cecad8 3feb0bf70fae2512
// 3fea5ab522610f73 3fe807f1d02b5bd5
// bfefdb1385845498 bfec1a12ee924433
// bfeab98dff3b972c bfe850f35b3bece2
// 3fe33b90cc1ec878 3fe23afc43cef048
// bfbd77b29473af38 bfbd67232cf45e1c
// bfc0d52264f94548 bfc0c8cf62802dee
// bfaca7d38cbe0a80 bfaca4008024315c
// 3fd89e3ee22ebdcc 3fd80c59731e1888
// 3fdb17eda830b6f4 3fda57e0a529d0ee
// 3fdc928ac90fa0f4 3fdbb3136c3ce067
// bfe1262476c0cd54 bfe06b451436e536
// bfd891a76658a7ee bfd800987b48cc21
// 3fe75dada7e86de4 3fe5abc954ddba76
// 3feb0ff8154e2eb2 3fe8931acf9c3424
// 3fdf06d705d2c98a 3fddeca4dd10c927
// bfe3f76f15ef5ae3 bfe2db30fc831b3a
// bfd111a2c1ca78b2 bfd0df6cd47e3610
// 3fde48a32b9caa20 3fdd411acd110531
// 3fef97a196284ef1 3febea2d4ea6d5e2
// 3fede2e4caecfaa7 3feaaf34dedaf346
// bfb8f8d3e2cf5e90 bfb8eebb83ffebbb
// 3fb5e89f9a291078 3fb5e1ccb188c4b0
// bfed44e8558c6b55 bfea3b30391bf2a2
// bfdb8a8438eb88d8 bfdac1444c67a9ee
// bfef3966dac6296e bfeba6ed817a3ce3
// 3fe177a452bbd5c2 3fe0b2f48605e792
// 3fd5fa7497c7977e 3fd5915a7701d53c
// bfd0f1b0f3280d4c bfd0c08d4d16fc3c
// 3fdce23d0b00be16 3fdbfbc87dba27bf
// 3f9b4fb30606ac40 3f9b4edee2be1f1b
// 3febc6ef604f80de 3fe91e0ad244b822
// bfd1425a423a5810 bfd10e7aa70b2a4a
// 3fc446f084e7184c 3fc43178425a3675
// 3fc1dcd9164395c0 3fc1ce224e896261
// 3fed04e125e49e2c 3fea0bd9c16f3cc9
// bfd9c1bae0d1834a bfd91b952b14a5f2
// 3fe9c5140195eae9 3fe793ecd6c5eda0
// bfcbf4cde2506070 bfcbbd17c333dc18
// bfccc9fdcd96446c bfcc8d3a5016f1e0
// 3fe908bfc5faf522 3fe7006ba0b4b184
// bfe7529c40e5d1d8 bfe5a2d84b86af7d
// 3fddc5abc9eb9c18 3fdcca8a4274c097
// bfcd86ee3ae16e44 bfcd45728cdfe255
// bfc3b6e0915c3048 bfc3a32243d288a9
// bfe8a5ac83066a77 bfe6b228731adf1e
// 3fdb59c5446ed18c 3fda947774299d7d
// bfd090d7606e9e78 bfd062dc1313a680
// 3fe4d49c91a53133 3fe395b29ec810fe
// 3fe8ba0c569302d0 3fe6c24a2bbff490
// 3fedd712a427ca3e 3feaa69077aff0f8
// bfecd6ff4d77b8de bfe9e9d0aec812d6
// 418b404fc3162484 40328de8f96b7488
// 41d17b6ce028e50d 4035937ec4ec228a
// 408655173bfff5d1 401d0f47933a499c
// bfea65e0e6524d11 bfe81090912fb579
// be22aed75aeb9d29 be22aed75aeb9d29
// c10cb224405f8efc c02a1f22f558fc7f
// 3feb027bf746a552 3fe888cdd9fa65a0
// c0609ecd76ff47a0 c016553a00052454
// c016e544370168a9 c003900ef6518b32
// c0a93a015cc53ccd c0218bd097572ae6
// 3e87c667839667c9 3e87c667839667a6
// 3e11d98eda6dcb94 3e11d98eda6dcb94
// 416b3a4c3d7d9238 40312acc44dc8abf
// c1656317acd288fe c030ecfcae7298d7
// 3fee7cfa81808980 3feb1f4a29e94b5e
// bea144763594e4ae bea144763594e3d7
// c06274676f3acee9 c016c06df4d339d0
// bf87a2dd3a378270 bf87a2bad788d84a
// 3f0138f32b5b8be8 3f0138f32b4e3e4d
// bf83230b4ef7d484 bf8322f90eef87c2
// 3e1422076fb38833 3e1422076fb38833
// 4017a2641f288700 4003d0280625b46a
// be1c28c11f1e1e4e be1c28c11f1e1e4e
// 40bf4cf742ff052e 40235d2a4f6ae669
// c19940c466a73f26 c0332bdc4838bef6
// bfc84b098455eb88 bfc8264b63da8c6f
// 3e73ccded0704d12 3e73ccded0704d0c
// c19259d0233a17c8 c032da209bf36608
// 3e387a09cc4c64a0 3e387a09cc4c64a0
// bf31ffcdf0faf143 bf31ffcded2f10f2
// bf2dc2f4ae75ee28 bf2dc2f4aa2b8b32
// 413f1b588b0dde46 402e711da02e66a2
// c149b81eb24a22ae c02f72a16a9cc763
// c03a3ee2751880ae c00fb025f67de5c6
// c040825333089264 c010c30132660429
// c069e40733ff2e61 c0181b1d8084b997
// 3fb3a5f40e85efb6 3fb3a1072e8e504e
// 4190e24e797072b6 4032c4ccf12eba3c
// 4158bac92af262e6 403060b7694b654f
// bf98a2773cb47237 bf98a1db8cb067f0
// c080f5aaa4e0668e c01bf5792beacdbe
// c0c65c914e3bbb1a c02413dfc6fc4d42
// 3fd71f13102b33ce 3fd6a5570a5d369e
// 3e6924bd2c13f2a4 3e6924bd2c13f2a1
// 4035402d0ef88402 400e002ef5aa31a0
// be20d335ec1dfd0e be20d335ec1dfd0e
// bf2724e26d190e2c bf2724e26b14828b
// c1a6776763ade5a0 c033bf5fb555a1ca
// bf73d3e6f8b4d4aa bf73d3e1e58ad088
// 41668c332abcb596 4030fa83df57ee43
// beec4e02cfb706e8 beec4e02cfb3560d
// 40dd70b10b863620 40260392e61c74df
// 3eac444c99c0d1a1 3eac444c99c0cdf4
// c178b7b6d077fabf c031c37bcaf6061a
// bf6b00e334851430 bf6b00e00015385d
// befc5f503bf32744 befc5f503be448b4
// c021901561e5f3bb c006f3bbf6f38acb
// bf9fe1961bbabc9a bf9fe044b5b1cbfc
// c0b82ff8a46d7370 c022d92af85736b0
// bf441813cf8fabab bf441813ba6ebdf0
// 4089045a47b8c079 401d83858ec72f5f
// c07bf9e98e980d9f c01b3038e3af8cdb
// c08c2be7a4bbcb87 c01dfd20833e06b2
// 40811f081d4297a7 401bff2eea39dd24
// 405566d61ad0e231 401492607e7a8fb1
// c08101b9da2c269e c01bf8504ab1442c
// c08a31e27a1d835e c01db2a22980baed
// c08aa54ac269a045 c01dc41b3d76b2dd
// 4067ce94cf8091a0 4017c531fda500b9
// 408ba94277171cb2 401dea682e3eb103
// 407afeb0d39dbd69 401b0ba86c55e374
// 406dddc7ccb3faa2 4018ad637ee9e23b
// 4038ec6f580d899a 400f46605e645aab
// c05c9ef205a6bc34 c015bc00e46399a2
// c088d420c5366bbf c01d7bc8269beb84
// 40863da363414e13 401d0b1200c5404e
// c089a72a0990087b c01d9d3a7b347762
// 406a4b15d13a7908 40182aea15d20e55
// c065f8d41fe23f29 c017730fd7ff2fa8
// 40867b1e8721c6a9 401d16118220b7e5
// 4077c8882b2981fe 401a89f4ae9bc028
// c07460d4022d36f8 c019ebbb093dd1d7
// c088043886eef719 c01d59ba2095993f
// c07211561b55e493 c0197080aa89a3ed
// 408171fe0170a0b0 401c12624e18e5fc
// c053422327c05870 c01426539dbbec92
// c0889826c6d467f5 c01d7212d01ca461
// 407baeb37ff376d8 401b25696bfc3cee
// 408d71fa5ceb8ced 401e2a68bd38b24a
// c0863526f46028ca c01d098b000a8d38
// c08a73755f0a012b c01dbc99187c0feb
// c06077c19b5ceab4 c0164bc9446a2bc5
// c08db6fd3eb3d7af c01e33bdc8adbc12
// c084931eb03021f8 c01cbb55fc2fd66f
// c080c58c2d474550 c01bea0f98561d4f
// 408ba17a7d743a80 401de947f8048fdc
// c0825a6354c0c00a c01c465457b2cee6
// c0653ddadc66b0d6 c0175072006e942f
// 40898f904c7db4f1 401d998ab69c5422
// 408c622861ab45a0 401e04cd285381e5
// n=101592 hash=79a3852e9ca72c3e
