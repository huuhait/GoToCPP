// phase: 09
// NaN payloads (signaling and quiet) survive being stored in variables, structs, arrays, slices, maps, interfaces, channels, closures and function arguments.

package main

import (
	"fmt"
	"math"
)


func main() {

	type box struct {
		a float64
		b float32
	}
	pats := []uint64{0x7ff0000000000001, 0x7ff8000000000000, 0xfff8000000000001, 0x7ff4479c6aacc07a, 0xffffffffffffffff}
	ch := make(chan float64, 1)
	for _, p := range pats {
		x := math.Float64frombits(p)
		bx := box{x, math.Float32frombits(uint32(p>>32) | 1)}
		arr := [2]float64{x, 1}
		sl := []float64{1, x}
		m := map[int]float64{1: x}
		var any1 any = x
		ch <- x
		fn := func(v float64) float64 { return v }
		cl := func() float64 { return x }
		r := <-ch
		fmt.Printf("%016x %016x %08x %016x %016x %016x %016x %016x %016x %016x\n", p, math.Float64bits(bx.a), math.Float32bits(bx.b), math.Float64bits(arr[0]), math.Float64bits(sl[1]),
			math.Float64bits(m[1]), math.Float64bits(any1.(float64)), math.Float64bits(r), math.Float64bits(fn(x)), math.Float64bits(cl()))
	}
	for _, b := range []uint32{0x7f800001, 0x7fc00000, 0xffc00001, 0x7fa00000} {
		f := math.Float32frombits(b)
		var a any = f
		s := []float32{f}
		fmt.Printf("%08x %08x %08x\n", b, math.Float32bits(a.(float32)), math.Float32bits(s[0]))
	}

}

// Output:
// 7ff0000000000001 7ff0000000000001 7ff00001 7ff0000000000001 7ff0000000000001 7ff0000000000001 7ff0000000000001 7ff0000000000001 7ff0000000000001 7ff0000000000001
// 7ff8000000000000 7ff8000000000000 7ff80001 7ff8000000000000 7ff8000000000000 7ff8000000000000 7ff8000000000000 7ff8000000000000 7ff8000000000000 7ff8000000000000
// fff8000000000001 fff8000000000001 fff80001 fff8000000000001 fff8000000000001 fff8000000000001 fff8000000000001 fff8000000000001 fff8000000000001 fff8000000000001
// 7ff4479c6aacc07a 7ff4479c6aacc07a 7ff4479d 7ff4479c6aacc07a 7ff4479c6aacc07a 7ff4479c6aacc07a 7ff4479c6aacc07a 7ff4479c6aacc07a 7ff4479c6aacc07a 7ff4479c6aacc07a
// ffffffffffffffff ffffffffffffffff ffffffff ffffffffffffffff ffffffffffffffff ffffffffffffffff ffffffffffffffff ffffffffffffffff ffffffffffffffff ffffffffffffffff
// 7f800001 7f800001 7f800001
// 7fc00000 7fc00000 7fc00000
// ffc00001 ffc00001 ffc00001
// 7fa00000 7fa00000 7fa00000
