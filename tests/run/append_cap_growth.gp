// phase: 08

// The capacity that append gives a growing slice follows gc 1.25's growslice for every kind of
// element: nextslicecap, then the size class of the new array in GO's element size (a string is
// 16 bytes, an interface 16, a slice 24) and, for elements that hold pointers, with the 8-byte
// malloc header of a block over 512 bytes (roundupsize, noscan = false). A zero-size element
// gives cap == len (shown for 40 appends). Each line is the sequence of distinct capacities of 5000 appends one at a
// time. The slices escape (sink), so that gc allocates on the heap and uses growslice.
package main

import "fmt"

var sink any

type small struct{ a, b int8 }
type pair struct {
	a int
	p *int
}
type mixed struct {
	a int8
	s string
	b bool
}
type tail struct {
	a int
	z struct{}
}
type tail4 struct {
	y int32
	z [0]*int
}
type blank struct {
	_ int32
	x int16
}
type empty struct{}
type zero struct{ z [0]int }
type marker struct{ _ [0]func() }
type zeros [3]zero
type named []int
type iface interface{ M() }
type celsius float64
type grid [3][3]int8
type wrap struct{ p pair }

func growth[T any](name string, n int) {
	var s []T
	var zero T
	last := -1
	out := name + ":"
	for i := 0; i < n; i++ {
		s = append(s, zero)
		if cap(s) != last {
			last = cap(s)
			out += fmt.Sprintf(" %d", last)
		}
	}
	sink = s
	fmt.Println(out)
}

func main() {
	growth[int]("int", 5000)
	growth[int8]("int8", 5000)
	growth[int16]("int16", 5000)
	growth[int32]("int32", 5000)
	growth[int64]("int64", 5000)
	growth[uint8]("uint8", 5000)
	growth[uint]("uint", 5000)
	growth[uintptr]("uintptr", 5000)
	growth[float32]("float32", 5000)
	growth[float64]("float64", 5000)
	growth[complex64]("complex64", 5000)
	growth[complex128]("complex128", 5000)
	growth[bool]("bool", 5000)
	growth[rune]("rune", 5000)
	growth[string]("string", 5000)
	growth[any]("any", 5000)
	growth[error]("error", 5000)
	growth[iface]("iface", 5000)
	growth[*int]("*int", 5000)
	growth[[]int]("[]int", 5000)
	growth[map[string]int]("map", 5000)
	growth[chan int]("chan", 5000)
	growth[func()]("func", 5000)
	growth[pair]("pair", 5000)
	growth[[4]string]("[4]string", 5000)
	growth[[3]*int]("[3]*int", 5000)
	growth[struct{}]("struct{}", 40)
	growth[empty]("empty", 40)
	growth[[0]int]("[0]int", 40)
	growth[[2]empty]("[2]empty", 40)
	growth[zero]("zero", 40)
	growth[marker]("marker", 40)
	growth[zeros]("zeros", 40)
	growth[[2][0]int]("[2][0]int", 40)
	growth[[3]int8]("[3]int8", 5000)
	growth[small]("small", 5000)
	growth[mixed]("mixed", 5000)
	growth[tail]("tail", 5000)
	growth[tail4]("tail4", 5000)
	growth[blank]("blank", 5000)
	growth[named]("named", 5000)
	growth[celsius]("celsius", 5000)
	growth[grid]("grid", 5000)
	growth[wrap]("wrap", 5000)
	growth[[1]string]("[1]string", 5000)
	growth[struct {
		a int8
		b string
	}]("anon", 5000)
}

// Output:
// int: 1 2 4 8 16 32 64 128 256 512 848 1280 1792 2560 3408 5120
// int8: 8 16 32 64 128 256 512 896 1408 2048 3072 4096 5376
// int16: 4 8 16 32 64 128 256 512 896 1344 2048 3072 4096 5440
// int32: 2 4 8 16 32 64 128 256 512 864 1344 2048 3072 4096 5440
// int64: 1 2 4 8 16 32 64 128 256 512 848 1280 1792 2560 3408 5120
// uint8: 8 16 32 64 128 256 512 896 1408 2048 3072 4096 5376
// uint: 1 2 4 8 16 32 64 128 256 512 848 1280 1792 2560 3408 5120
// uintptr: 1 2 4 8 16 32 64 128 256 512 848 1280 1792 2560 3408 5120
// float32: 2 4 8 16 32 64 128 256 512 864 1344 2048 3072 4096 5440
// float64: 1 2 4 8 16 32 64 128 256 512 848 1280 1792 2560 3408 5120
// complex64: 1 2 4 8 16 32 64 128 256 512 848 1280 1792 2560 3408 5120
// complex128: 1 2 4 8 16 32 64 128 256 512 848 1280 1792 2560 3584 5120
// bool: 8 16 32 64 128 256 512 896 1408 2048 3072 4096 5376
// rune: 2 4 8 16 32 64 128 256 512 864 1344 2048 3072 4096 5440
// string: 1 2 4 8 16 32 71 143 303 591 1023 1535 2560 3584 5120
// any: 1 2 4 8 16 32 71 143 303 591 1023 1535 2560 3584 5120
// error: 1 2 4 8 16 32 71 143 303 591 1023 1535 2560 3584 5120
// iface: 1 2 4 8 16 32 71 143 303 591 1023 1535 2560 3584 5120
// *int: 1 2 4 8 16 32 64 143 287 607 1023 1535 2303 3071 4095 6144
// []int: 1 2 4 8 16 37 74 170 341 682 1135 1706 2389 3413 4778 6485
// map: 1 2 4 8 16 32 64 143 287 607 1023 1535 2303 3071 4095 6144
// chan: 1 2 4 8 16 32 64 143 287 607 1023 1535 2303 3071 4095 6144
// func: 1 2 4 8 16 32 64 143 287 607 1023 1535 2303 3071 4095 6144
// pair: 1 2 4 8 16 32 71 143 303 591 1023 1535 2560 3584 5120
// [4]string: 1 2 4 8 17 35 75 151 319 640 1024 1536 2176 2944 3968 5248
// [3]*int: 1 2 4 8 16 37 74 170 341 682 1135 1706 2389 3413 4778 6485
// struct{}: 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40
// empty: 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40
// [0]int: 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40
// [2]empty: 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40
// zero: 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40
// marker: 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40
// zeros: 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40
// [2][0]int: 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40
// [3]int8: 2 5 10 21 42 85 170 341 682 1066 1621 2261 3157 4522 6144
// small: 4 8 16 32 64 128 256 512 896 1344 2048 3072 4096 5440
// mixed: 1 2 4 8 16 35 71 151 303 575 1023 1536 2304 3072 4096 5376
// tail: 1 2 4 8 16 32 64 128 256 512 848 1280 1792 2560 3584 5120
// tail4: 1 2 4 8 16 32 64 128 256 512 848 1280 1792 2560 3584 5120
// blank: 1 2 4 8 16 32 64 128 256 512 848 1280 1792 2560 3408 5120
// named: 1 2 4 8 16 37 74 170 341 682 1135 1706 2389 3413 4778 6485
// celsius: 1 2 4 8 16 32 64 128 256 512 848 1280 1792 2560 3408 5120
// grid: 1 2 5 10 21 42 85 170 341 682 1052 1507 2119 3029 4551 6371
// wrap: 1 2 4 8 16 32 71 143 303 591 1023 1535 2560 3584 5120
// [1]string: 1 2 4 8 16 32 71 143 303 591 1023 1535 2560 3584 5120
// anon: 1 2 4 8 16 37 74 170 341 682 1135 1706 2389 3413 4778 6485
