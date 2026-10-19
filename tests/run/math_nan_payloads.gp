// phase: 09
// NaN payloads and signs through Abs, Copysign, Floor, Ceil, Trunc, Sqrt, Max, Min, Float32 conversions and arithmetic: the bits are Go's on amd64.

package main

import (
	"fmt"
	"math"
)


func main() {

	nans := []uint64{0x7ff0000000000001, 0x7ff8000000000000, 0xfff8000000000000, 0x7ff4479c6aacc07a, 0xfff8000000000001, 0x7fffffffffffffff, 0xffffffffffffffff, 0xfff0000000000001, 0x7ff8000000000001}
	for _, b := range nans {
		x := math.Float64frombits(b)
		fmt.Printf("%016x abs=%016x neg=%016x cs=%016x fl=%016x ce=%016x tr=%016x sq=%016x\n", b, math.Float64bits(math.Abs(x)), math.Float64bits(-x), math.Float64bits(math.Copysign(x, -1)),
			math.Float64bits(math.Floor(x)), math.Float64bits(math.Ceil(x)), math.Float64bits(math.Trunc(x)), math.Float64bits(math.Sqrt(x)))
		fmt.Printf("  add=%016x mul=%016x max=%016x min=%016x f32=%08x rte=%016x rnd=%016x\n", math.Float64bits(x+1), math.Float64bits(x*2), math.Float64bits(math.Max(x, 1)), math.Float64bits(math.Min(1, x)),
			math.Float32bits(float32(x)), math.Float64bits(math.RoundToEven(x)), math.Float64bits(math.Round(x)))
		fmt.Printf("  exp=%016x log=%016x sin=%016x pow=%016x mod=%016x hyp=%016x fma=%016x\n", math.Float64bits(math.Exp(x)), math.Float64bits(math.Log(x)), math.Float64bits(math.Sin(x)),
			math.Float64bits(math.Pow(x, 2)), math.Float64bits(math.Mod(x, 2)), math.Float64bits(math.Hypot(x, 1)), math.Float64bits(math.FMA(x, 2, 1)))
	}
	fmt.Printf("%016x %016x %016x\n", math.Float64bits(math.Sqrt(-1)), math.Float64bits(math.Inf(1)-math.Inf(1)), math.Float64bits(math.Log(-1)))

}

// Output:
// 7ff0000000000001 abs=7ff0000000000001 neg=fff0000000000001 cs=fff0000000000001 fl=7ff8000000000001 ce=7ff8000000000001 tr=7ff8000000000001 sq=7ff8000000000001
//   add=7ff8000000000001 mul=7ff8000000000001 max=7ff8000000000001 min=7ff8000000000001 f32=7fc00000 rte=7ff8000000000001 rnd=7ff0000000000001
//   exp=7ff0000000000001 log=7ff0000000000001 sin=7ff0000000000001 pow=7ff8000000000001 mod=7ff8000000000001 hyp=7ff8000000000001 fma=7ff8000000000001
// 7ff8000000000000 abs=7ff8000000000000 neg=fff8000000000000 cs=fff8000000000000 fl=7ff8000000000000 ce=7ff8000000000000 tr=7ff8000000000000 sq=7ff8000000000000
//   add=7ff8000000000000 mul=7ff8000000000000 max=7ff8000000000001 min=7ff8000000000001 f32=7fc00000 rte=7ff8000000000000 rnd=7ff8000000000000
//   exp=7ff8000000000000 log=7ff8000000000000 sin=7ff8000000000000 pow=7ff8000000000001 mod=7ff8000000000001 hyp=7ff8000000000001 fma=7ff8000000000000
// fff8000000000000 abs=7ff8000000000000 neg=7ff8000000000000 cs=fff8000000000000 fl=fff8000000000000 ce=fff8000000000000 tr=fff8000000000000 sq=fff8000000000000
//   add=fff8000000000000 mul=fff8000000000000 max=7ff8000000000001 min=7ff8000000000001 f32=ffc00000 rte=fff8000000000000 rnd=fff8000000000000
//   exp=fff8000000000000 log=7ff8000000000001 sin=fff8000000000000 pow=7ff8000000000001 mod=7ff8000000000001 hyp=7ff8000000000001 fma=fff8000000000000
// 7ff4479c6aacc07a abs=7ff4479c6aacc07a neg=fff4479c6aacc07a cs=fff4479c6aacc07a fl=7ffc479c6aacc07a ce=7ffc479c6aacc07a tr=7ffc479c6aacc07a sq=7ffc479c6aacc07a
//   add=7ffc479c6aacc07a mul=7ffc479c6aacc07a max=7ff8000000000001 min=7ff8000000000001 f32=7fe23ce3 rte=7ffc479c6aacc07a rnd=7ff4479c6aacc07a
//   exp=7ff4479c6aacc07a log=7ff4479c6aacc07a sin=7ff4479c6aacc07a pow=7ff8000000000001 mod=7ff8000000000001 hyp=7ff8000000000001 fma=7ffc479c6aacc07a
// fff8000000000001 abs=7ff8000000000001 neg=7ff8000000000001 cs=fff8000000000001 fl=fff8000000000001 ce=fff8000000000001 tr=fff8000000000001 sq=fff8000000000001
//   add=fff8000000000001 mul=fff8000000000001 max=7ff8000000000001 min=7ff8000000000001 f32=ffc00000 rte=fff8000000000001 rnd=fff8000000000001
//   exp=fff8000000000001 log=7ff8000000000001 sin=fff8000000000001 pow=7ff8000000000001 mod=7ff8000000000001 hyp=7ff8000000000001 fma=fff8000000000001
// 7fffffffffffffff abs=7fffffffffffffff neg=ffffffffffffffff cs=ffffffffffffffff fl=7fffffffffffffff ce=7fffffffffffffff tr=7fffffffffffffff sq=7fffffffffffffff
//   add=7fffffffffffffff mul=7fffffffffffffff max=7ff8000000000001 min=7ff8000000000001 f32=7fffffff rte=7fffffffffffffff rnd=7fffffffffffffff
//   exp=7fffffffffffffff log=7fffffffffffffff sin=7fffffffffffffff pow=7ff8000000000001 mod=7ff8000000000001 hyp=7ff8000000000001 fma=7fffffffffffffff
// ffffffffffffffff abs=7fffffffffffffff neg=7fffffffffffffff cs=ffffffffffffffff fl=ffffffffffffffff ce=ffffffffffffffff tr=ffffffffffffffff sq=ffffffffffffffff
//   add=ffffffffffffffff mul=ffffffffffffffff max=7ff8000000000001 min=7ff8000000000001 f32=ffffffff rte=ffffffffffffffff rnd=ffffffffffffffff
//   exp=ffffffffffffffff log=7ff8000000000001 sin=ffffffffffffffff pow=7ff8000000000001 mod=7ff8000000000001 hyp=7ff8000000000001 fma=ffffffffffffffff
// fff0000000000001 abs=7ff0000000000001 neg=7ff0000000000001 cs=fff0000000000001 fl=fff8000000000001 ce=fff8000000000001 tr=fff8000000000001 sq=fff8000000000001
//   add=fff8000000000001 mul=fff8000000000001 max=7ff8000000000001 min=7ff8000000000001 f32=ffc00000 rte=fff8000000000001 rnd=fff0000000000001
//   exp=fff0000000000001 log=7ff8000000000001 sin=fff0000000000001 pow=7ff8000000000001 mod=7ff8000000000001 hyp=7ff8000000000001 fma=fff8000000000001
// 7ff8000000000001 abs=7ff8000000000001 neg=fff8000000000001 cs=fff8000000000001 fl=7ff8000000000001 ce=7ff8000000000001 tr=7ff8000000000001 sq=7ff8000000000001
//   add=7ff8000000000001 mul=7ff8000000000001 max=7ff8000000000001 min=7ff8000000000001 f32=7fc00000 rte=7ff8000000000001 rnd=7ff8000000000001
//   exp=7ff8000000000001 log=7ff8000000000001 sin=7ff8000000000001 pow=7ff8000000000001 mod=7ff8000000000001 hyp=7ff8000000000001 fma=7ff8000000000001
// fff8000000000000 fff8000000000000 7ff8000000000001
