// phase: 09
// math/rand and math/rand/v2: the generators with a seed give Go's sequences (Source, PCG, ChaCha8, Zipf, Perm, Shuffle, NormFloat64, ExpFloat64), the global generators stay in their ranges.

package main

import (
	"fmt"
	"math/rand"
	randv2 "math/rand/v2"
)

func main() {
	// math/rand with a seeded source: Go's additive lagged Fibonacci generator.
	r := rand.New(rand.NewSource(42))
	fmt.Println(r.Int63(), r.Int31(), r.Int(), r.Uint32(), r.Uint64())
	fmt.Println(r.Intn(100), r.Int63n(1e12), r.Int31n(1000), r.Float64(), r.Float32())
	fmt.Println(r.NormFloat64(), r.ExpFloat64())
	fmt.Println(r.Perm(8))
	s := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	r.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
	fmt.Println(s)
	buf := make([]byte, 12)
	n, err := r.Read(buf)
	fmt.Println(n, err, buf)
	r.Seed(7)
	fmt.Println(r.Intn(1000), r.Intn(1000), r.Intn(1000))
	z := rand.NewZipf(rand.New(rand.NewSource(1)), 1.5, 2, 100)
	fmt.Println(z.Uint64(), z.Uint64(), z.Uint64(), z.Uint64())
	for _, seed := range []int64{0, 1, -1, 1 << 40} {
		x := rand.New(rand.NewSource(seed))
		fmt.Println(seed, x.Int63(), x.Intn(10))
	}

	// math/rand/v2: PCG and ChaCha8.
	p := randv2.New(randv2.NewPCG(1, 2))
	fmt.Println(p.Int64(), p.Uint32(), p.IntN(100), p.Int64N(1e15), p.Uint64N(1000), p.Float64(), p.Float32())
	fmt.Println(p.NormFloat64(), p.ExpFloat64(), p.Perm(6), p.Uint())
	var seed [32]byte
	copy(seed[:], "chacha8 seed for the test 123456")
	c := randv2.New(randv2.NewChaCha8(seed))
	fmt.Println(c.Uint64(), c.Uint64(), c.Uint64(), c.Uint64())
	fmt.Println(c.IntN(1000), c.Int32N(77), c.UintN(5), c.Float64())
	cc := randv2.NewChaCha8(seed)
	b := make([]byte, 20)
	cc.Read(b)
	fmt.Println(b)
	state, _ := cc.MarshalBinary()
	fmt.Println(len(state))
	pcg := randv2.NewPCG(3, 4)
	pcg.Uint64()
	ps, _ := pcg.MarshalBinary()
	pcg2 := randv2.NewPCG(0, 0)
	fmt.Println(pcg2.UnmarshalBinary(ps), pcg2.Uint64() == pcg.Uint64())
	zv2 := randv2.NewZipf(randv2.New(randv2.NewPCG(5, 6)), 2, 1, 50)
	fmt.Println(zv2.Uint64(), zv2.Uint64(), zv2.Uint64())
	sv := []string{"a", "b", "c", "d", "e"}
	p.Shuffle(len(sv), func(i, j int) { sv[i], sv[j] = sv[j], sv[i] })
	fmt.Println(sv, randv2.N[int64](1) == 0)

	// The global generators are random: only their ranges are checked.
	ok := true
	counts := map[int]int{}
	for i := 0; i < 2000; i++ {
		a, b := rand.Intn(10), randv2.IntN(10)
		if a < 0 || a >= 10 || b < 0 || b >= 10 {
			ok = false
		}
		counts[a]++
		f := rand.Float64()
		g := randv2.Float64()
		if f < 0 || f >= 1 || g < 0 || g >= 1 {
			ok = false
		}
	}
	fmt.Println(ok, len(counts), rand.Perm(5) != nil, len(randv2.Perm(7)))
	x1, x2 := rand.Int63(), rand.Int63()
	y1, y2 := randv2.Uint64(), randv2.Uint64()
	fmt.Println(x1 != x2, y1 != y2)
	func() {
		defer func() { fmt.Println(recover()) }()
		rand.Intn(0)
	}()
	func() {
		defer func() { fmt.Println(recover()) }()
		randv2.IntN(-1)
	}()
}

// Output:
// 3440579354231278675 141734987 5571782338101878760 896869500 9627525982598323465
// 45 244883513247 176 0.3830446530497163 0.6463764
// -1.3183925147106337 0.7777953930592235
// [7 3 4 6 2 5 0 1]
// [6 7 4 5 0 8 9 2 3 1]
// 12 <nil> [225 41 112 94 39 63 5 201 35 38 130 142]
// 886 870 853
// 2 0 1 4
// 0 8717895732742165505 4
// 1 5577006791947779410 7
// -1 3644962268338389676 2
// 1099511627776 5470237361722394362 0
// 4969059760275911952 2647573421 78 796596578488087 234 0.3883664855410056 0.97740936
// 1.726131393719264 0.7426404617374481 [0 5 4 2 3 1] 7311121042813227358
// 9492550970221228079 3782266590968023926 15945607294765129540 905735118235063284
// 302 72 3 0.15522134239599206
// [47 112 83 80 208 80 188 131 118 11 122 216 108 79 125 52 68 95 90 228]
// 61
// <nil> true
// 0 0 0
// [a c b e d] true
// true 10 true 7
// true true
// invalid argument to Intn
// invalid argument to IntN
