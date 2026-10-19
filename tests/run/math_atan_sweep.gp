// phase: 09
// math.Atan on about 100k inputs: bit-identical to Go (hash of the result bits and 200 samples).

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
		y := math.Atan(x)
		h = mix(h, math.Float64bits(y))
		if i%500 == 0 {
			fmt.Printf("%016x %016x\n", math.Float64bits(x), math.Float64bits(y))
		}
	}
	fmt.Printf("n=%d hash=%016x\n", len(xs), h)
}

// Output:
// 0000000000000000 0000000000000000
// 40862e42fefa39f0 3ff91c3602aaf162
// 4008000000000000 3ff3fc176b7a8560
// 4040bfffffffffff 3ff8a7bfd859094d
// 5b65923ac30ec1f4 3ff921fb54442d18
// c024bfdbf760976a bff798645ace5676
// cc9f5dfc8a5c49d1 bff921fb54442d18
// 28faf9b36567288d 28faf9b36567288d
// 5c01af71ea368d0e 3ff921fb54442d18
// 52bd26a497b45c45 3ff921fb54442d18
// 57a8503628a803dd 3ff921fb54442d18
// 9146970ff4ce0971 9146970ff4ce0971
// f56a5815e599ff1f bff921fb54442d18
// 398243dbd8701b2c 398243dbd8701b2c
// 5ba4a48740cb04df 3ff921fb54442d18
// da25dad4e83140c7 bff921fb54442d18
// 9ba45dbea8b9d076 9ba45dbea8b9d076
// f762bc1ba29bdb35 bff921fb54442d18
// 662b00cc72923f60 3ff921fb54442d18
// 482a3a7945c16ef0 3ff921fb54442d18
// a8698fe6f1d1b6d7 a8698fe6f1d1b6d7
// e006469c13f2521e bff921fb54442d18
// e1dd8f8aae0c18d4 bff921fb54442d18
// e11d793466eefc2b bff921fb54442d18
// 4dcd1ad5a02370a4 3ff921fb54442d18
// 0deb45aa655f885e 0deb45aa655f885e
// f6250f700d64e16e bff921fb54442d18
// d8536e85cdee3a05 bff921fb54442d18
// 4e76b4f39cf95384 3ff921fb54442d18
// f88bbc29e3ba37f8 bff921fb54442d18
// 73ea5c3d1e048b48 3ff921fb54442d18
// 5c7b45e3f515c5bb 3ff921fb54442d18
// ab2567ceba64e421 ab2567ceba64e421
// eb8d783bcea270b8 bff921fb54442d18
// c3b9dee0ebf001a5 bff921fb54442d18
// 0e7855fba5612c1d 0e7855fba5612c1d
// 836e10793726318c 836e10793726318c
// 3b34dda55a15d732 3b34dda55a15d732
// 2f4e9ffba969cc56 2f4e9ffba969cc56
// 8b1e87ad9104ded4 8b1e87ad9104ded4
// 17c82fc123e13faa 17c82fc123e13faa
// f3be5d2a6268c34c bff921fb54442d18
// 4b335c2e086fec82 3ff921fb54442d18
// 304c21caf2bd8d77 304c21caf2bd8d77
// 3fe30bf498dd7094 3fe12e33033eabe6
// 3fb5aac89bff8058 3fb59d98763d582e
// 3feb34272e81d3a3 3fe68bc4c588572d
// 3fe816820fd48922 3fe4a5f27efb62b3
// bfd0fe16d5fa012a bfd09bfca7a56dc8
// bfd7e00241eae292 bfd6da089aa1d00e
// 3fd04afd572abb6e 3fcfe8770bd3429a
// bfd3fac47071ae3a bfd35db2b8a4bb3f
// 3fee624f10cecad8 3fe84dd356dfef29
// 3fea5ab522610f73 3fe60be1407c8184
// bfefdb1385845498 bfe90f7a6c33cce5
// bfeab98dff3b972c bfe6441310cc94c4
// 3fe33b90cc1ec878 3fe15143d4912320
// bfbd77b29473af38 bfbd56a47c3a48f6
// bfc0d52264f94548 bfc0bc8c91d5dd0d
// bfaca7d38cbe0a80 bfaca02e5e9c9d3d
// 3fd89e3ee22ebdcc 3fd7806462aa5bc7
// 3fdb17eda830b6f4 3fd9a122b15a6e84
// 3fdc928ac90fa0f4 3fdadf86eaf64f96
// bfe1262476c0cd54 bfdf7c3e227800d4
// bfd891a76658a7ee bfd7756ba84b4bae
// 3fe75dada7e86de4 3fe42eaef3885237
// 3feb0ff8154e2eb2 3fe676b7f2f05066
// 3fdf06d705d2c98a 3fdce3dd4cb55d2b
// bfe3f76f15ef5ae3 bfe1d9e200c9aff0
// bfd111a2c1ca78b2 bfd0ae3dabbedef5
// 3fde48a32b9caa20 3fdc492826055a8b
// 3fef97a196284ef1 3fe8ed76a922ffeb
// 3fede2e4caecfaa7 3fe80a4bf62dcbbc
// bfb8f8d3e2cf5e90 bfb8e4aa7ab57d63
// 3fb5e89f9a291078 3fb5dafd9b139abe
// bfed44e8558c6b55 bfe7b518eecab34a
// bfdb8a8438eb88d8 bfda02107e81ef19
// bfef3966dac6296e bfe8bd7821cf7c81
// 3fe177a452bbd5c2 3fdffa595404b830
// 3fd5fa7497c7977e 3fd52bb80ee87e66
// bfd0f1b0f3280d4c bfd0906738901fa4
// 3fdce23d0b00be16 3fdb21dbb4a64fe0
// 3f9b4fb30606ac40 3f9b4e0acb0b2c55
// 3febc6ef604f80de 3fe6e0393e03be36
// bfd1425a423a5810 bfd0dbb0580f94d2
// 3fc446f084e7184c 3fc41c28bea5d4f6
// 3fc1dcd9164395c0 3fc1bf8145713d45
// 3fed04e125e49e2c 3fe79219654fe12f
// bfd9c1bae0d1834a bfd87cc834ffda9d
// 3fe9c5140195eae9 3fe5b1eb99b79e94
// bfcbf4cde2506070 bfcb8627e58a84c7
// bfccc9fdcd96446c bfcc515bbcc6d417
// 3fe908bfc5faf522 3fe53e639f379d6a
// bfe7529c40e5d1d8 bfe42775b1d453de
// 3fddc5abc9eb9c18 3fdbddd1c996120a
// bfcd86ee3ae16e44 bfcd04f9f8770cd9
// bfc3b6e0915c3048 bfc38f8768f0dd81
// bfe8a5ac83066a77 bfe5009134e66962
// 3fdb59c5446ed18c 3fd9d8e3fecbca03
// bfd090d7606e9e78 bfd035c3fb73970c
// 3fe4d49c91a53133 3fe27727a5a6e4e9
// 3fe8ba0c569302d0 3fe50d56fc56c2bb
// 3fedd712a427ca3e 3fe803fa7c54a1cf
// bfecd6ff4d77b8de bfe778da07d5bb74
// 418b404fc3162484 3ff921fb4f91bc04
// 41d17b6ce028e50d 3ff921fb54099a20
// 408655173bfff5d1 3ff91c400b42d650
// bfea65e0e6524d11 bfe612881b23c0b0
// be22aed75aeb9d29 be22aed75aeb9d29
// c10cb224405f8efc bff921f6de5c1276
// 3feb027bf746a552 3fe66ed9a968a06b
// c0609ecd76ff47a0 bff9032d3aa14d2d
// c016e544370168a9 bff65d88f143976a
// c0a93a015cc53ccd bff920b697a5c14b
// 3e87c667839667c9 3e87c66783966783
// 3e11d98eda6dcb94 3e11d98eda6dcb94
// 416b3a4c3d7d9238 3ff921fb41764275
// c1656317acd288fe bff921fb3c539dc5
// 3fee7cfa81808980 3fe85bd3f772fc6d
// bea144763594e4ae bea144763594e301
// c06274676f3acee9 bff9063d132aea92
// bf87a2dd3a378270 bf87a2987534385f
// 3f0138f32b5b8be8 3f0138f32b40f0b2
// bf83230b4ef7d484 bf8322e6cf068edb
// 3e1422076fb38833 3e1422076fb38833
// 4017a2641f288700 3ff67342b92311c6
// be1c28c11f1e1e4e be1c28c11f1e1e4e
// 40bf4cf742ff052e 3ff921787821180f
// c19940c466a73f26 bff921fb51bb61c2
// bfc84b098455eb88 bfc801f0b0ac4778
// 3e73ccded0704d12 3e73ccded0704d08
// c19259d0233a17c8 bff921fb50c75b09
// 3e387a09cc4c64a0 3e387a09cc4c64a0
// bf31ffcdf0faf143 bf31ffcde96330a3
// bf2dc2f4ae75ee28 bf2dc2f4a5e1283d
// 413f1b588b0dde46 3ff921fad0974c47
// c149b81eb24a22ae bff921fb04a31bd8
// c03a3ee2751880ae bff885fe430d34d9
// c040825333089264 bff8a5f77bd3f8b0
// c069e40733ff2e61 bff90e34dccedce0
// 3fb3a5f40e85efb6 3fb39c1c86f027aa
// 4190e24e797072b6 3ff921fb5079ca62
// 4158bac92af262e6 3ff921fb2adbc69c
// bf98a2773cb47237 bf98a13fe3974117
// c080f5aaa4e0668e bff91a6f36dcd067
// c0c65c914e3bbb1a bff9219fbe59d55a
// 3fd71f13102b33ce 3fd63004ee553a2b
// 3e6924bd2c13f2a4 3e6924bd2c13f29f
// 4035402d0ef88402 3ff861608bc9fa80
// be20d335ec1dfd0e be20d335ec1dfd0e
// bf2724e26d190e2c bf2724e2690ff6eb
// c1a6776763ade5a0 bff921fb52d78b08
// bf73d3e6f8b4d4aa bf73d3dcd26322ed
// 41668c332abcb596 3ff921fb3d8f10f8
// beec4e02cfb706e8 beec4e02cfafa532
// 40dd70b10b863620 3ff921d88bfabc1b
// 3eac444c99c0d1a1 3eac444c99c0ca47
// c178b7b6d077fabf bff921fb49e8ca24
// bf6b00e334851430 bf6b00dccba60bce
// befc5f503bf32744 befc5f503bd56a23
// c021901561e5f3bb bff7518af07f8035
// bf9fe1961bbabc9a bf9fdef368c30708
// c0b82ff8a46d7370 bff92151fc160d0c
// bf441813cf8fabab bf441813a54dd05d
// 4089045a47b8c079 3ff91cdd802b9237
// c07bf9e98e980d9f bff918d4c5aa97fc
// c08c2be7a4bbcb87 bff91d702b1e3487
// 40811f081d4297a7 3ff91a8172dede9e
// 405566d61ad0e231 3ff8f22324682c83
// c08101b9da2c269e bff91a7490e9e9a4
// c08a31e27a1d835e bff91d1865bd5329
// c08aa54ac269a045 bff91d2d8fc090e5
// 4067ce94cf8091a0 3ff90c79c3328a1d
// 408ba94277171cb2 3ff91d5ab57357df
// 407afeb0d39dbd69 3ff9187f9d485b3a
// 406dddc7ccb3faa2 3ff910d6bbdfd4e0
// 4038ec6f580d899a 3ff87dba3a8882ca
// c05c9ef205a6bc34 bff8fe345ffddd4f
// c088d420c5366bbf bff91cd39017ad8c
// 40863da363414e13 3ff91c3a00090c24
// c089a72a0990087b bff91cfdf93cd80f
// 406a4b15d13a7908 3ff90e825ef071fc
// c065f8d41fe23f29 bff90aadfaba5397
// 40867b1e8721c6a9 3ff91c49bd4cf39a
// 4077c8882b2981fe 3ff91737cab12f84
// c07460d4022d36f8 bff9156b5beb6da9
// c088043886eef719 bff91ca6ef151794
// c07211561b55e493 bff913d019ec65bb
// 408171fe0170a0b0 3ff91aa500407135
// c053422327c05870 bff8ecd02091b8dd
// c0889826c6d467f5 bff91cc6fdac13ef
// 407baeb37ff376d8 3ff918bbe9210567
// 408d71fa5ceb8ced 3ff91da27bd1b266
// c0863526f46028ca bff91c37cd0aac6d
// c08a73755f0a012b bff91d2482e11bc4
// c06077c19b5ceab4 bff902e430d738c3
// c08db6fd3eb3d7af bff91dac94597f4b
// c084931eb03021f8 bff91bc2b19add18
// c080c58c2d474550 bff91a598f831b74
// 408ba17a7d743a80 3ff91d5967d905d6
// c0825a6354c0c00a bff91b01e887b23f
// c0653ddadc66b0d6 bff909e0df8629e8
// 40898f904c7db4f1 3ff91cf95dd7f039
// 408c622861ab45a0 3ff91d78da6a0f05
// n=101592 hash=03b9818d24ab83bd
