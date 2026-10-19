// phase: 09
// math.Cbrt on about 100k inputs: bit-identical to Go (hash of the result bits and 200 samples).

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
		y := math.Cbrt(x)
		h = mix(h, math.Float64bits(y))
		if i%500 == 0 {
			fmt.Printf("%016x %016x\n", math.Float64bits(x), math.Float64bits(y))
		}
	}
	fmt.Printf("n=%d hash=%016x\n", len(xs), h)
}

// Output:
// 0000000000000000 0000000000000000
// 40862e42fefa39f0 4021d725ed9a1980
// 4008000000000000 3ff7137449123ef6
// 4040bfffffffffff 4009ca0a8337b317
// 5b65923ac30ec1f4 4916450c9274ec72
// c024bfdbf760976a c00172b88dd29685
// cc9f5dfc8a5c49d1 c4293ad8787d6f8f
// 28faf9b36567288d 3847fe220bd4c261
// 5c01af71ea368d0e 494a42b5ad15018d
// 52bd26a497b45c45 46338ab26478b9df
// 57a8503628a803dd 47d72d0d0c2c2e6f
// 9146970ff4ce0971 b061f31329178044
// f56a5815e599ff1f d1c2e4aeefa71d70
// 398243dbd8701b2c 3dca8b6151d2c7a6
// 5ba4a48740cb04df 492ba64bb7ae2218
// da25dad4e83140c7 c8ac2e3691dc704d
// 9ba45dbea8b9d076 b3d5d8d9c1d20ff4
// f762bc1ba29bdb35 d26ac523dfaaccbe
// 662b00cc72923f60 4cae3d40c815d4e6
// 482a3a7945c16ef0 42adf280abda0d47
// a8698fe6f1d1b6d7 b81790f03f42163e
// e006469c13f2521e caa1ddaaade28968
// e1dd8f8aae0c18d4 cb3f2a527ad96122
// e11d793466eefc2b caff2276f0c67def
// 4dcd1ad5a02370a4 448f01186cc58b6a
// 0deb45aa655f885e 2f431cd318930d48
// f6250f700d64e16e d20188ea8a9d46ea
// d8536e85cdee3a05 c81112172efb7dd3
// 4e76b4f39cf95384 44c6a76371f57d19
// f88bbc29e3ba37f8 d2ce829202472104
// 73ea5c3d1e048b48 5142e5ad0e2b82c8
// 5c7b45e3f515c5bb 49731ce08ad9f277
// ab2567ceba64e421 b901a14f19cf7fb9
// eb8d783bcea270b8 ce78b5e3fb00cf96
// c3b9dee0ebf001a5 c132c78852d236a8
// 0e7855fba5612c1d 2f7266854e4ab8dc
// 836e10793726318c abc3be693a9ad603
// 3b34dda55a15d732 3e5bbfb4ccb7be40
// 2f4e9ffba969cc56 3a63dda204b4654c
// 8b1e87ad9104ded4 ae53d85f3ea4b1f8
// 17c82fc123e13faa 328d2628e017025a
// f3be5d2a6268c34c d133cf24d967158d
// 4b335c2e086fec82 43b10cb66b92a841
// 304c21caf2bd8d77 3ab854b1152d8dd2
// 3fe30bf498dd7094 3feaeaf65799db03
// 3fb5aac89bff8058 3fdc19808562e097
// 3feb34272e81d3a3 3fee50601b972513
// 3fe816820fd48922 3fed1c00f444a031
// bfd0fe16d5fa012a bfe49136c14d6e86
// bfd7e00241eae292 bfe7092ee7a190a2
// 3fd04afd572abb6e 3fe447f0a51e308d
// bfd3fac47071ae3a bfe5b53b99e3ff42
// 3fee624f10cecad8 3fef73b6dd0c01e1
// 3fea5ab522610f73 3fedfec0488c2108
// bfefdb1385845498 bfeff3ac6d9572a5
// bfeab98dff3b972c bfee229130241caa
// 3fe33b90cc1ec878 3feb01516da6bec6
// bfbd77b29473af38 bfdf21ef135824a7
// bfc0d52264f94548 bfe045d8d1c16bba
// bfaca7d38cbe0a80 bfd87b1871b96b18
// 3fd89e3ee22ebdcc 3fe745be1e1e0e44
// 3fdb17eda830b6f4 3fe80714eb47d1d1
// 3fdc928ac90fa0f4 3fe875074a486a8f
// bfe1262476c0cd54 bfe9fe0d3768a2cd
// bfd891a76658a7ee bfe741c5ae76728e
// 3fe75dada7e86de4 3fecd0cb3a8d540d
// 3feb0ff8154e2eb2 3fee42e969199744
// 3fdf06d705d2c98a 3fe92365374b32e1
// bfe3f76f15ef5ae3 bfeb582739a7f8b4
// bfd111a2c1ca78b2 bfe499169a4dfee9
// 3fde48a32b9caa20 3fe8ef9c90d965b4
// 3fef97a196284ef1 3fefdd0fc51180c5
// 3fede2e4caecfaa7 3fef4781e92b9e15
// bfb8f8d3e2cf5e90 bfdd7613b60c3379
// 3fb5e89f9a291078 3fdc3422de010e78
// bfed44e8558c6b55 bfef1001cb0471c0
// bfdb8a8438eb88d8 bfe828c55194659e
// bfef3966dac6296e bfefbd4220bea960
// 3fe177a452bbd5c2 3fea26f9a1b310b6
// 3fd5fa7497c7977e 3fe668b159698313
// bfd0f1b0f3280d4c bfe48c350314b3d3
// 3fdce23d0b00be16 3fe88baf83e7d619
// 3f9b4fb30606ac40 3fd31f2ad962bfe5
// 3febc6ef604f80de 3fee8684a0820ae4
// bfd1425a423a5810 bfe4ac9cc3a28c24
// 3fc446f084e7184c 3fe15091194cadf2
// 3fc1dcd9164395c0 3fe09926cbb3ed12
// 3fed04e125e49e2c 3feef94ab32b489a
// bfd9c1bae0d1834a bfe7a036500ffd1a
// 3fe9c5140195eae9 3fedc58f34c7774e
// bfcbf4cde2506070 bfe3456671a35bd5
// bfccc9fdcd96446c bfe375e838a5a288
// 3fe908bfc5faf522 3fed7c5541244479
// bfe7529c40e5d1d8 bfeccc3dc33b0f0b
// 3fddc5abc9eb9c18 3fe8cb75fa26e959
// bfcd86ee3ae16e44 bfe3a01ef55a196a
// bfc3b6e0915c3048 bfe1272d2506a5b8
// bfe8a5ac83066a77 bfed553bbc7850a2
// 3fdb59c5446ed18c 3fe81a7bff1aedc9
// bfd090d7606e9e78 bfe464c34e385c01
// 3fe4d49c91a53133 3febbbb2fc644653
// 3fe8ba0c569302d0 3fed5d4ea8043378
// 3fedd712a427ca3e 3fef4361a1e81148
// bfecd6ff4d77b8de bfeee8ef0cccd5de
// 418b404fc3162484 407812ff19a19e7e
// 41d17b6ce028e50d 40907ac25a2900f2
// 408655173bfff5d1 4021e188ecf77d3a
// bfea65e0e6524d11 bfee02fca16699a8
// be22aed75aeb9d29 bf553a522abc968e
// c10cb224405f8efc c04edbbeaa5f4abc
// 3feb027bf746a552 3fee3de1d89f98e8
// c0609ecd76ff47a0 c0146a7c58e43045
// c016e544370168a9 bffc9ef59a6f7936
// c0a93a015cc53ccd c02d8f9f3891c949
// 3e87c667839667c9 3f7700efd5d5c88e
// 3e11d98eda6dcb94 3f509821dc8be44f
// 416b3a4c3d7d9238 406e52a83d29e4bd
// c1656317acd288fe c06bfa6040c5ff90
// 3fee7cfa81808980 3fef7ce7fbd66e21
// bea144763594e4ae bf80696ae4153347
// c06274676f3acee9 c01524190dc6a143
// bf87a2dd3a378270 bfcced2028cbdf5a
// 3f0138f32b5b8be8 3fa065c469033f45
// bf83230b4ef7d484 bfcaf5d2675898db
// 3e1422076fb38833 3f514609291d2e9c
// 4017a2641f288700 3ffceceec1270714
// be1c28c11f1e1e4e bf53514eef2a11de
// 40bf4cf742ff052e 403402c3ba19c20a
// c19940c466a73f26 c07d924323fa0a09
// bfc84b098455eb88 bfe263c2a844ac8f
// 3e73ccded0704d12 3f712d8bb230c9dd
// c19259d0233a17c8 c07a95ffb54e5c71
// 3e387a09cc4c64a0 3f5d43e20175b05f
// bf31ffcdf0faf143 bfb0a3f38fa3d544
// bf2dc2f4ae75ee28 bfaf3c5987d9800c
// 413f1b588b0dde46 405fb30fc67e9561
// c149b81eb24a22ae c062be22c8d96dc9
// c03a3ee2751880ae c007c63d43300c0c
// c040825333089264 c009aa3c3f4af5ea
// c069e40733ff2e61 c017aaae4e53e2f8
// 3fb3a5f40e85efb6 3fdb32c1f7bfc59e
// 4190e24e797072b6 4079db99df57d4f3
// 4158bac92af262e6 40674eb8f3314fe0
// bf98a2773cb47237 bfd279b7fd7e1da4
// c080f5aaa4e0668e c020504dddd8a523
// c0c65c914e3bbb1a c03689d87aa253fa
// 3fd71f13102b33ce 3fe6ca77b40565fb
// 3e6924bd2c13f2a4 3f6d874e58483b69
// 4035402d0ef88402 400628abd5ea2c58
// be20d335ec1dfd0e bf547fdb6af4453d
// bf2724e26d190e2c bfacb95f77ae8f01
// c1a6776763ade5a0 c081eaacab9cf366
// bf73d3e6f8b4d4aa bfc5a71f0b940b76
// 41668c332abcb596 406c79a824bed36b
// beec4e02cfb706e8 bf986169dec363b9
// 40dd70b10b863620 403f1f77415eb726
// 3eac444c99c0d1a1 3f8357995b474673
// c178b7b6d077fabf c0727f065292bdef
// bf6b00e334851430 bfc30cb469ece033
// befc5f503bf32744 bf9ebdf1454614d2
// c021901561e5f3bb c000813d89e82a6b
// bf9fe1961bbabc9a bfd4223e0335cd54
// c0b82ff8a46d7370 c0325cebbf630333
// bf441813cf8fabab bfb5bfd3ea9cf5e8
// 4089045a47b8c079 40229210713b17d6
// c07bf9e98e980d9f c01e9925b876cf0c
// c08c2be7a4bbcb87 c023520755fd13f4
// 40811f081d4297a7 40205d8685a7dc1e
// 405566d61ad0e231 4011a10ad6db328f
// c08101b9da2c269e c020542ad38fed48
// c08a31e27a1d835e c022db88a8c4ac63
// c08aa54ac269a045 c022f711f7f22e18
// 4067ce94cf8091a0 40170392a90dd3bb
// 408ba94277171cb2 402333faf7f3868c
// 407afeb0d39dbd69 401e3c7754ad39da
// 406dddc7ccb3faa2 4018d2258f530d7b
// 4038ec6f580d899a 40075e47a43554b0
// c05c9ef205a6bc34 c0136c3059bdc0ac
// c088d420c5366bbf c0228619faae5ac9
// 40863da363414e13 4021db445a0f40c6
// c089a72a0990087b c022ba0378058e7f
// 406a4b15d13a7908 4017c9ebc7f2dd86
// c065f8d41fe23f29 c0166823cafd878e
// 40867b1e8721c6a9 4021eba974d5ee2d
// 4077c8882b2981fe 401cfc75be4dc282
// c07460d4022d36f8 c01b87f0656e0bc1
// c088043886eef719 c02251d2af2662f6
// c07211561b55e493 c01a72d11ff20fcf
// 408171fe0170a0b0 402077cb06b07563
// c053422327c05870 c011050de58c17d8
// c0889826c6d467f5 c0227723887b2f57
// 407baeb37ff376d8 401e7da1784de756
// 408d71fa5ceb8ced 40239b7979be6183
// c0863526f46028ca c021d8feaabb6c8f
// c08a73755f0a012b c022eb37de289eeb
// c06077c19b5ceab4 c0145a72e1a02c9e
// c08db6fd3eb3d7af c023aabefa2619ed
// c084931eb03021f8 c021662528fe8060
// c080c58c2d474550 c02040d16bc2d9b2
// 408ba17a7d743a80 4023322dd733fbb3
// c0825a6354c0c00a c020bfaf7eab89bc
// c0653ddadc66b0d6 c01627dd46940c83
// 40898f904c7db4f1 4022b4438d86330d
// 408c622861ab45a0 40235e66709c1b8e
// n=101592 hash=2034e882e9b92c2e
