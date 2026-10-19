// phase: 09
// Differential sweep of package cmp: Less, Compare, Or and Ordered over every integer and float type, strings and
// named types, on the pairs of a table of edge values (NaN, -0, infinities, min and max) and on pseudo-random values;
// one hash per function and type is printed. The expected output comes from real Go 1.25 (GOCPP_RUN_VS_GO=1
// checks it again).

package main

import (
	"cmp"
	"fmt"
	"math"
)

type rngT struct{ s uint64 }

func (r *rngT) next() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

type hasher struct{ h uint64 }

func (x *hasher) add(v int) {
	x.h ^= uint64(v + 2)
	x.h *= 1099511628211
}

type MyInt int16
type MyStr string
type MyFloat float32

func sweep[T cmp.Ordered](name string, vals []T) {
	h := &hasher{14695981039346656037}
	for _, a := range vals {
		for _, b := range vals {
			h.add(cmp.Compare(a, b))
			if cmp.Less(a, b) {
				h.add(10)
			}
			h.add(len(fmt.Sprint(cmp.Or(a, b))))
			h.add(cmp.Compare(cmp.Or(a, b), b))
		}
	}
	fmt.Printf("%-12s %016x %d\n", name, h.h, len(vals))
}

func randInts[T ~int8 | ~int16 | ~int32 | ~int64 | ~int | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uint | ~uintptr](r *rngT, edge []T) []T {
	vals := append([]T(nil), edge...)
	for i := 0; i < 40; i++ {
		vals = append(vals, T(r.next()>>(r.next()%64)))
	}
	return vals
}

func main() {
	r := &rngT{99}
	sweep("int8", randInts(r, []int8{math.MinInt8, -1, 0, 1, math.MaxInt8}))
	sweep("int16", randInts(r, []int16{math.MinInt16, -1, 0, 1, math.MaxInt16}))
	sweep("int32", randInts(r, []int32{math.MinInt32, -1, 0, 1, math.MaxInt32}))
	sweep("int64", randInts(r, []int64{math.MinInt64, -1, 0, 1, math.MaxInt64}))
	sweep("int", randInts(r, []int{math.MinInt, -1, 0, 1, math.MaxInt}))
	sweep("uint8", randInts(r, []uint8{0, 1, math.MaxUint8}))
	sweep("uint16", randInts(r, []uint16{0, 1, math.MaxUint16}))
	sweep("uint32", randInts(r, []uint32{0, 1, math.MaxUint32}))
	sweep("uint64", randInts(r, []uint64{0, 1, math.MaxUint64}))
	sweep("uint", randInts(r, []uint{0, 1, math.MaxUint}))
	sweep("uintptr", randInts(r, []uintptr{0, 1, math.MaxUint32}))
	sweep("MyInt", randInts(r, []MyInt{math.MinInt16, 0, math.MaxInt16}))
	nan := math.NaN()
	negz := math.Copysign(0, -1)
	f64 := []float64{nan, nan, math.Inf(-1), math.Inf(1), 0, negz, 1, -1, 0.1, 1e-320, -1e-320, math.MaxFloat64, -math.MaxFloat64, math.SmallestNonzeroFloat64}
	for i := 0; i < 40; i++ {
		f64 = append(f64, math.Float64frombits(r.next()))
	}
	sweep("float64", f64)
	f32 := []float32{float32(nan), math.MaxFloat32, -math.MaxFloat32, float32(math.Inf(1)), float32(math.Inf(-1)), 0, float32(negz), 1, -1, 1e-40}
	for i := 0; i < 40; i++ {
		f32 = append(f32, math.Float32frombits(uint32(r.next())))
	}
	sweep("float32", f32)
	sweep("MyFloat", []MyFloat{MyFloat(nan), 0, 1.5, -2.5, MyFloat(math.Inf(1))})
	strs := []string{"", "a", "b", "ab", "ba", "abc", "A", "é", "☺", "\xff", "\x00", "a\x00", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab"}
	for i := 0; i < 30; i++ {
		b := make([]byte, r.next()%12)
		for j := range b {
			b[j] = byte(r.next())
		}
		strs = append(strs, string(b))
	}
	sweep("string", strs)
	sweep("MyStr", []MyStr{"", "x", "y", "xy", "☺"})
	// Or: the first non-zero value, for comparable types that are not ordered too
	fmt.Println(cmp.Or(0, 0, 3, 4), cmp.Or("", "", "z"), cmp.Or[int](), cmp.Or(0.0, negz, nan) != 0.0)
	type P struct{ a, b int }
	fmt.Println(cmp.Or(P{}, P{1, 2}, P{3, 4}), cmp.Or([2]string{}, [2]string{"", "q"}))
	var p1, p2 *int
	x := 5
	p2 = &x
	fmt.Println(cmp.Or(p1, p2) == p2, cmp.Or(p1, p1) == nil)
	var e1, e2 error
	fmt.Println(cmp.Or(e1, e2) == nil)
}

// Output:
// int8         4238812f33413b00 45
// int16        e6043d4cd036b39c 45
// int32        d4e379dd83fcaadc 45
// int64        a5f12e740c93dc50 45
// int          e05df835f6b99cf5 45
// uint8        bc5d53ecb0ce610a 43
// uint16       5922bb77cf2f011c 43
// uint32       30f31c44de664e18 43
// uint64       f54a470d6c73b162 43
// uint         989d399d2f0c37a1 43
// uintptr      c6a8d95f3aa46738 43
// MyInt        2701617ffeea2082 43
// float64      1a772a5ffe001bc7 54
// float32      0e6f1af3205b439d 50
// MyFloat      cd1c623f6b82e52a 5
// string       5910fa14f48926c7 44
// MyStr        f9c59af31b97374b 5
// 3 z 0 true
// {1 2} [ q]
// true true
// true
