// phase: 09
// math.Sinh on about 100k inputs: bit-identical to Go (hash of the result bits and 200 samples).

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
		y := math.Sinh(x)
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
// 4008000000000000 40240926e70949ae
// 4040bfffffffffff 42e41dbd6e3e0616
// 5b65923ac30ec1f4 7ff0000000000000
// c024bfdbf760976a c0cf49e09753f04c
// cc9f5dfc8a5c49d1 fff0000000000000
// 28faf9b36567288d 28faf9b36567288d
// 5c01af71ea368d0e 7ff0000000000000
// 52bd26a497b45c45 7ff0000000000000
// 57a8503628a803dd 7ff0000000000000
// 9146970ff4ce0971 9146970ff4ce0971
// f56a5815e599ff1f fff0000000000000
// 398243dbd8701b2c 398243dbd8701b2c
// 5ba4a48740cb04df 7ff0000000000000
// da25dad4e83140c7 fff0000000000000
// 9ba45dbea8b9d076 9ba45dbea8b9d076
// f762bc1ba29bdb35 fff0000000000000
// 662b00cc72923f60 7ff0000000000000
// 482a3a7945c16ef0 7ff0000000000000
// a8698fe6f1d1b6d7 a8698fe6f1d1b6d7
// e006469c13f2521e fff0000000000000
// e1dd8f8aae0c18d4 fff0000000000000
// e11d793466eefc2b fff0000000000000
// 4dcd1ad5a02370a4 7ff0000000000000
// 0deb45aa655f885e 0deb45aa655f885e
// f6250f700d64e16e fff0000000000000
// d8536e85cdee3a05 fff0000000000000
// 4e76b4f39cf95384 7ff0000000000000
// f88bbc29e3ba37f8 fff0000000000000
// 73ea5c3d1e048b48 7ff0000000000000
// 5c7b45e3f515c5bb 7ff0000000000000
// ab2567ceba64e421 ab2567ceba64e421
// eb8d783bcea270b8 fff0000000000000
// c3b9dee0ebf001a5 fff0000000000000
// 0e7855fba5612c1d 0e7855fba5612c1d
// 836e10793726318c 836e10793726318b
// 3b34dda55a15d732 3b34dda55a15d732
// 2f4e9ffba969cc56 2f4e9ffba969cc56
// 8b1e87ad9104ded4 8b1e87ad9104ded4
// 17c82fc123e13faa 17c82fc123e13faa
// f3be5d2a6268c34c fff0000000000000
// 4b335c2e086fec82 7ff0000000000000
// 304c21caf2bd8d77 304c21caf2bd8d77
// 3fe30bf498dd7094 3fe43100b5cb0ab2
// 3fb5aac89bff8058 3fb5b1688a3c59ba
// 3feb34272e81d3a3 3fee99d211459212
// 3fe816820fd48922 3fea6d95be2c0e15
// bfd0fe16d5fa012a bfd131611d829f4d
// bfd7e00241eae292 bfd86ec2c4fbbba4
// 3fd04afd572abb6e 3fd07830681131f1
// bfd3fac47071ae3a bfd44e404eb2a62b
// 3fee624f10cecad8 3ff19472b9772cce
// 3fea5ab522610f73 3fed6fada9bfe4b8
// bfefdb1385845498 bff2b12f4a93dab4
// bfeab98dff3b972c bfedf10f2af2e1de
// 3fe33b90cc1ec878 3fe46962d1a2c81c
// bfbd77b29473af38 bfbd885e09fec176
// bfc0d52264f94548 bfc0e190a54a2d98
// bfaca7d38cbe0a80 bfacaba821c9fea8
// 3fd89e3ee22ebdcc 3fd93ad0aebb3f17
// 3fdb17eda830b6f4 3fdbe8f5ecbabeef
// 3fdc928ac90fa0f4 3fdd87f479518e82
// bfe1262476c0cd54 bfe1fb5198a2b7dc
// bfd891a76658a7ee bfd92d4841414a7a
// 3fe75dada7e86de4 3fe97f927f1765b3
// 3feb0ff8154e2eb2 3fee67d4c68a9085
// 3fdf06d705d2c98a 3fe020d23f4816ff
// bfe3f76f15ef5ae3 bfe5499d01951702
// bfd111a2c1ca78b2 bfd1459f40c69a82
// 3fde48a32b9caa20 3fdf6d330cf713f7
// 3fef97a196284ef1 3ff27d7ce32a2c6d
// 3fede2e4caecfaa7 3ff1365701dc47db
// bfb8f8d3e2cf5e90 bfb902f88ad2c0b6
// 3fb5e89f9a291078 3fb5ef78e77c95e3
// bfed44e8558c6b55 bff0c321385ae94a
// bfdb8a8438eb88d8 bfdc66258b54f569
// bfef3966dac6296e bff235cc11ad08e7
// 3fe177a452bbd5c2 3fe25908906c1581
// 3fd5fa7497c7977e 3fd669b2e32f69ac
// bfd0f1b0f3280d4c bfd1248b05909118
// 3fdce23d0b00be16 3fdddfd0ec84d97b
// 3f9b4fb30606ac40 3f9b50873c9fc9a6
// 3febc6ef604f80de 3fef662ec901fe11
// bfd1425a423a5810 bfd17819f5b12965
// 3fc446f084e7184c 3fc45cad9a3e97a4
// 3fc1dcd9164395c0 3fc1ebb47cb564b0
// 3fed04e125e49e2c 3ff094e50c13570b
// bfd9c1bae0d1834a bfda752b7593ed8e
// 3fe9c5140195eae9 3feca59dbb8e9520
// bfcbf4cde2506070 bfcc2dd6dd7b83d0
// bfccc9fdcd96446c bfcd084924157d86
// 3fe908bfc5faf522 3febaac3493dbe2d
// bfe7529c40e5d1d8 bfe9716d1ca66f2e
// 3fddc5abc9eb9c18 3fdedb8d14775aa9
// bfcd86ee3ae16e44 bfcdca2607c196d4
// bfc3b6e0915c3048 bfc3cadab3d456f7
// bfe8a5ac83066a77 bfeb284e5f8f854c
// 3fdb59c5446ed18c 3fdc30d96af351b3
// bfd090d7606e9e78 bfd0c05ae829b18b
// 3fe4d49c91a53133 3fe655477004c284
// 3fe8ba0c569302d0 3feb430cdb40bb4d
// 3fedd712a427ca3e 3ff12da9d39d9377
// bfecd6ff4d77b8de bff073ec2ca8ef78
// 418b404fc3162484 7ff0000000000000
// 41d17b6ce028e50d 7ff0000000000000
// 408655173bfff5d1 7ff0000000000000
// bfea65e0e6524d11 bfed7edd254c9746
// be22aed75aeb9d29 be22aed75aeb9d29
// c10cb224405f8efc fff0000000000000
// 3feb027bf746a552 3fee553d834b8fe4
// c0609ecd76ff47a0 cbdc5579669e349b
// c016e544370168a9 c0632174a12782b6
// c0a93a015cc53ccd fff0000000000000
// 3e87c667839667c9 3e87c667839667ec
// 3e11d98eda6dcb94 3e11d98eda6dcb94
// 416b3a4c3d7d9238 7ff0000000000000
// c1656317acd288fe fff0000000000000
// 3fee7cfa81808980 3ff1a84885c043cd
// bea144763594e4ae bea144763594e585
// c06274676f3acee9 cd2fe9b8d9719670
// bf87a2dd3a378270 bf87a2ff9d7c4063
// 3f0138f32b5b8be8 3f0138f32b68d983
// bf83230b4ef7d484 bf83231d8f345851
// 3e1422076fb38833 3e1422076fb38833
// 4017a2641f288700 406702e9ed0fd489
// be1c28c11f1e1e4e be1c28c11f1e1e4e
// 40bf4cf742ff052e 7ff0000000000000
// c19940c466a73f26 fff0000000000000
// bfc84b098455eb88 bfc870708dafcae6
// 3e73ccded0704d12 3e73ccded0704d17
// c19259d0233a17c8 fff0000000000000
// 3e387a09cc4c64a0 3e387a09cc4c64a0
// bf31ffcdf0faf143 bf31ffcdf4c6d196
// bf2dc2f4ae75ee28 bf2dc2f4b2c05120
// 413f1b588b0dde46 7ff0000000000000
// c149b81eb24a22ae fff0000000000000
// c03a3ee2751880ae c23d2168f7a42b72
// c040825333089264 c2d8d974ffff1555
// c069e40733ff2e61 d28c3c7dd19e7fc0
// 3fb3a5f40e85efb6 3fb3aae4a4bd6f83
// 4190e24e797072b6 7ff0000000000000
// 4158bac92af262e6 7ff0000000000000
// bf98a2773cb47237 bf98a312f840e2d2
// c080f5aaa4e0668e f0cf2e6412b876bc
// c0c65c914e3bbb1a fff0000000000000
// 3fd71f13102b33ce 3fd7a0ac95e50693
// 3e6924bd2c13f2a4 3e6924bd2c13f2a6
// 4035402d0ef88402 41c9403966c29352
// be20d335ec1dfd0e be20d335ec1dfd0e
// bf2724e26d190e2c bf2724e26f1d99cd
// c1a6776763ade5a0 fff0000000000000
// bf73d3e6f8b4d4aa bf73d3ec0be2be5c
// 41668c332abcb596 7ff0000000000000
// beec4e02cfb706e8 beec4e02cfbab7c3
// 40dd70b10b863620 7ff0000000000000
// 3eac444c99c0d1a1 3eac444c99c0d54e
// c178b7b6d077fabf fff0000000000000
// bf6b00e334851430 bf6b00e668f61420
// befc5f503bf32744 befc5f503c0205d5
// c021901561e5f3bb c0a97012ad66365f
// bf9fe1961bbabc9a bf9fe2e7ab9f8bf1
// c0b82ff8a46d7370 fff0000000000000
// bf441813cf8fabab bf441813e4b099a9
// 4089045a47b8c079 7ff0000000000000
// c07bf9e98e980d9f e83b71e0c05c6413
// c08c2be7a4bbcb87 fff0000000000000
// 40811f081d4297a7 714570bfaa4c6e2b
// 405566d61ad0e231 4796b2d7b845d83b
// c08101b9da2c269e f0f19927b21fd08c
// c08a31e27a1d835e fff0000000000000
// c08aa54ac269a045 fff0000000000000
// 4067ce94cf8091a0 510b4617dfac99c7
// 408ba94277171cb2 7ff0000000000000
// 407afeb0d39dbd69 66d17671ed25cf46
// 406dddc7ccb3faa2 556a10ac0747620f
// 4038ec6f580d899a 421f1014815a4422
// c05c9ef205a6bc34 ca31efb068691ba4
// c088d420c5366bbf fff0000000000000
// 40863da363414e13 7ff0000000000000
// c089a72a0990087b fff0000000000000
// 406a4b15d13a7908 52d618a171efae90
// c065f8d41fe23f29 cfb819cfca4efcd6
// 40867b1e8721c6a9 7ff0000000000000
// 4077c8882b2981fe 622fdabf44631c00
// c07460d4022d36f8 dd450380a92734e2
// c088043886eef719 fff0000000000000
// c07211561b55e493 d9f0ac0eaabef76b
// 408171fe0170a0b0 7234ddc4a87ad201
// c053422327c05870 c6d193f6f6efb7ea
// c0889826c6d467f5 fff0000000000000
// 407baeb37ff376d8 67cfedcaa47ef3d5
// 408d71fa5ceb8ced 7ff0000000000000
// c0863526f46028ca fff0000000000000
// c08a73755f0a012b fff0000000000000
// c06077c19b5ceab4 cbc0ba0b02b43103
// c08db6fd3eb3d7af fff0000000000000
// c084931eb03021f8 fb3cf5a2d9ef976f
// c080c58c2d474550 f0437e83889b51c6
// 408ba17a7d743a80 7ff0000000000000
// c0825a6354c0c00a f4d3991782a9c9ef
// c0653ddadc66b0d6 cf31e500e3ef21d0
// 40898f904c7db4f1 7ff0000000000000
// 408c622861ab45a0 7ff0000000000000
// n=101592 hash=e0c8d5d7fb159f3d
