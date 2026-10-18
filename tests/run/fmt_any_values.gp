// phase: 05

// Values of every kind inside an any: the dynamic type decides; %v %T %d %s on the same values;
// nil interfaces; typed nil pointers; named types.
package main

import "fmt"

type Celsius float64

type Pt struct{ X, Y int }

type Color int

func (c Color) String() string { return "color" + fmt.Sprint(int(c)) }

func main() {
	var np *Pt
	var ns []int
	var nm map[string]int
	var nf func()
	var nerr error
	vals := []any{
		nil, true, 1, int8(-8), int16(16), int32(32), int64(64), uint(1), uint8(8), uint16(16), uint32(32), uint64(64), uintptr(9),
		1.5, float32(2.5), complex(1, 2), complex64(complex(3, 4)), "str", 'x', byte('y'),
		[]int{1, 2}, [2]string{"a", "b"}, map[string]int{"k": 1}, Pt{1, 2}, &Pt{3, 4},
		Celsius(21.5), Color(2), []Color{1, 2}, struct{ A int }{5}, np, ns, nm, nf == nil, nerr,
	}
	for i, v := range vals {
		fmt.Printf("%2d %-16T %v | %+v | %#v | %d | %s\n", i, v, v, v, v, v, v)
	}
	fmt.Println(vals[0], vals[1], vals[len(vals)-3])
	fmt.Println(vals[:5], len(vals))
}

// Output:
//  0 <nil>            <nil> | <nil> | <nil> | %!d(<nil>) | %!s(<nil>)
//  1 bool             true | true | true | %!d(bool=true) | %!s(bool=true)
//  2 int              1 | 1 | 1 | 1 | %!s(int=1)
//  3 int8             -8 | -8 | -8 | -8 | %!s(int8=-8)
//  4 int16            16 | 16 | 16 | 16 | %!s(int16=16)
//  5 int32            32 | 32 | 32 | 32 | %!s(int32=32)
//  6 int64            64 | 64 | 64 | 64 | %!s(int64=64)
//  7 uint             1 | 1 | 0x1 | 1 | %!s(uint=1)
//  8 uint8            8 | 8 | 0x8 | 8 | %!s(uint8=8)
//  9 uint16           16 | 16 | 0x10 | 16 | %!s(uint16=16)
// 10 uint32           32 | 32 | 0x20 | 32 | %!s(uint32=32)
// 11 uint64           64 | 64 | 0x40 | 64 | %!s(uint64=64)
// 12 uintptr          9 | 9 | 0x9 | 9 | %!s(uintptr=9)
// 13 float64          1.5 | 1.5 | 1.5 | %!d(float64=1.5) | %!s(float64=1.5)
// 14 float32          2.5 | 2.5 | 2.5 | %!d(float32=2.5) | %!s(float32=2.5)
// 15 complex128       (1+2i) | (1+2i) | (1+2i) | %!d(complex128=(1+2i)) | %!s(complex128=(1+2i))
// 16 complex64        (3+4i) | (3+4i) | (3+4i) | %!d(complex64=(3+4i)) | %!s(complex64=(3+4i))
// 17 string           str | str | "str" | %!d(string=str) | str
// 18 int32            120 | 120 | 120 | 120 | %!s(int32=120)
// 19 uint8            121 | 121 | 0x79 | 121 | %!s(uint8=121)
// 20 []int            [1 2] | [1 2] | []int{1, 2} | [1 2] | [%!s(int=1) %!s(int=2)]
// 21 [2]string        [a b] | [a b] | [2]string{"a", "b"} | [%!d(string=a) %!d(string=b)] | [a b]
// 22 map[string]int   map[k:1] | map[k:1] | map[string]int{"k":1} | map[%!d(string=k):1] | map[k:%!s(int=1)]
// 23 main.Pt          {1 2} | {X:1 Y:2} | main.Pt{X:1, Y:2} | {1 2} | {%!s(int=1) %!s(int=2)}
// 24 *main.Pt         &{3 4} | &{X:3 Y:4} | &main.Pt{X:3, Y:4} | &{3 4} | &{%!s(int=3) %!s(int=4)}
// 25 main.Celsius     21.5 | 21.5 | 21.5 | %!d(main.Celsius=21.5) | %!s(main.Celsius=21.5)
// 26 main.Color       color2 | color2 | 2 | 2 | color2
// 27 []main.Color     [color1 color2] | [color1 color2] | []main.Color{1, 2} | [1 2] | [color1 color2]
// 28 struct { A int } {5} | {A:5} | struct { A int }{A:5} | {5} | {%!s(int=5)}
// 29 *main.Pt         <nil> | <nil> | (*main.Pt)(nil) | 0 | %!s(*main.Pt=<nil>)
// 30 []int            [] | [] | []int(nil) | [] | []
// 31 map[string]int   map[] | map[] | map[string]int(nil) | map[] | map[]
// 32 bool             true | true | true | %!d(bool=true) | %!s(bool=true)
// 33 <nil>            <nil> | <nil> | <nil> | %!d(<nil>) | %!s(<nil>)
// <nil> true map[]
// [<nil> true 1 -8 16] 34
