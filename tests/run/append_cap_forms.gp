// phase: 07

// The capacity after the other forms of append, and after make and the conversions between
// strings and slices, is gc 1.25's too: growslice for many elements at once (newLen above twice
// the old capacity gives newLen, then the size class), for elements with and without pointers,
// and for a zero-size element; rawbyteslice and rawruneslice for []byte(s) and []rune(s) (the
// size class of the length; a constant string gives cap == len). The results escape (sink), so
// that gc uses the heap and growslice.
package main

import "fmt"

var sink any

type pair struct {
	a int
	p *int
}

func once[T any](name string) {
	var zero T
	out := name + ":"
	for _, start := range []int{0, 1, 7, 33, 300} {
		for _, k := range []int{1, 2, 5, 33, 100, 513} {
			s := make([]T, start)
			src := make([]T, k)
			s = append(s, src...)
			sink = s
			out += fmt.Sprintf(" %d", cap(s))
		}
	}
	a := append([]T(nil), zero, zero, zero)
	b := append([]T(nil), zero, zero, zero, zero, zero)
	c := append(a, zero, zero, zero, zero, zero, zero, zero)
	sink = []any{a, b, c}
	out += fmt.Sprintf(" | %d %d %d", cap(a), cap(b), cap(c))
	d := append([]T(nil), make([]T, 5)...)
	e := append(d[:0:0], d...)
	sink = []any{d, e}
	out += fmt.Sprintf(" | %d %d", cap(d), cap(e))
	f := make([]T, 3, 10)
	f = append(f, zero)
	g := append(f[:1], f[2:]...)
	sink = []any{f, g}
	out += fmt.Sprintf(" | %d %d %d", cap(f), cap(g), len(g))
	fmt.Println(out)
}

func repeat(s string, n int) string {
	r := ""
	for i := 0; i < n; i++ {
		r += s
	}
	return r
}

func conversions() {
	for _, n := range []int{0, 1, 5, 7, 8, 9, 31, 32, 33, 100, 600, 5000} {
		s := repeat("é", n/2) + repeat("x", n%2)
		b := []byte(s)
		sink = b
		r := []rune(s)
		sink = r
		t := string(b)
		fmt.Println("convert", len(s), cap(b), len(r), cap(r), len(t))
	}
	// The conversion of a constant string has cap == len.
	cb := []byte("hello")
	sink = cb
	cr := []rune("héllo wörld")
	sink = cr
	eb := []byte("")
	sink = eb
	fmt.Println("const", len(cb), cap(cb), len(cr), cap(cr), len(eb), cap(eb))
	const k = "abc"
	kb := append([]byte(k), "defghi"...)
	sink = kb
	fmt.Println("const append", len(kb), cap(kb))
	var nb []byte
	nb = append(nb, "hello, world"...)
	sink = nb
	fmt.Println("append string", len(nb), cap(nb))
}

func makes() {
	a := make([]string, 0, 5)
	b := make([]any, 3)
	c := make([]byte, 10, 33)
	d := make([]struct{}, 4, 9)
	e := make([]int, 0)
	sink = []any{a, b, c, d, e}
	fmt.Println("make", cap(a), cap(b), cap(c), cap(d), cap(e))
	a = append(a, "1", "2", "3", "4", "5", "6")
	b = append(b, 1)
	c = append(c, c...)
	d = append(d, d...)
	e = append(e, 1, 2, 3)
	sink = []any{a, b, c, d, e}
	fmt.Println("grown", cap(a), cap(b), cap(c), cap(d), cap(e))
}

type zero struct{ z [0]int }
type marker struct{ _ [0]func() }

// zeroMany appends a zero-size element 300000 times: cap == len each time, and it must not take
// quadratic time (a zero-size element has no memory, in gc too).
func zeroMany[T any](name string) {
	var s []T
	var zero T
	ok := true
	for i := 0; i < 300000; i++ {
		s = append(s, zero)
		if cap(s) != len(s) {
			ok = false
		}
	}
	s = append(s, s...)
	sink = s
	fmt.Println(name, len(s), cap(s), ok)
}

func main() {
	zeroMany[struct{}]("struct{}")
	zeroMany[zero]("zero")
	zeroMany[marker]("marker")
	zeroMany[[2][0]int]("[2][0]int")
	zeroMany[[3]zero]("[3]zero")
	once[int]("int")
	once[byte]("byte")
	once[int32]("int32")
	once[string]("string")
	once[any]("any")
	once[*int]("*int")
	once[[]int]("[]int")
	once[pair]("pair")
	once[struct{}]("struct{}")
	once[[3]byte]("[3]byte")
	once[[2]string]("[2]string")
	conversions()
	makes()
}

// Output:
// struct{} 600000 600000 true
// zero 600000 600000 true
// marker 600000 600000 true
// [2][0]int 600000 600000 true
// [3]zero 600000 600000 true
// int: 1 2 6 36 112 608 2 3 6 36 112 608 14 14 14 40 112 608 72 72 72 72 144 608 608 608 608 608 608 816 | 3 6 10 | 6 6 | 10 10 3
// byte: 8 8 8 48 112 576 8 8 8 48 112 576 16 16 16 48 112 576 80 80 80 80 144 576 576 576 576 576 576 896 | 8 8 16 | 8 8 | 10 10 3
// int32: 2 2 6 36 104 576 2 4 6 36 104 576 16 16 16 40 112 576 72 72 72 72 144 576 576 576 576 576 576 864 | 4 6 12 | 6 6 | 10 10 3
// string: 1 2 5 35 111 591 2 3 6 35 111 591 14 14 14 43 111 591 71 71 71 71 143 591 591 591 591 591 591 847 | 3 5 10 | 5 5 | 10 10 3
// any: 1 2 5 35 111 591 2 3 6 35 111 591 14 14 14 43 111 591 71 71 71 71 143 591 591 591 591 591 591 847 | 3 5 10 | 5 5 | 10 10 3
// *int: 1 2 6 36 111 607 2 3 6 36 111 607 14 14 14 40 111 607 71 71 71 71 143 607 607 607 607 607 607 815 | 3 6 10 | 6 6 | 10 10 3
// []int: 1 2 5 37 111 565 2 3 6 37 111 565 14 14 14 42 111 565 74 74 74 74 133 565 597 597 597 597 597 853 | 3 5 10 | 5 5 | 10 10 3
// pair: 1 2 5 35 111 591 2 3 6 35 111 591 14 14 14 43 111 591 71 71 71 71 143 591 591 591 591 591 591 847 | 3 5 10 | 5 5 | 10 10 3
// struct{}: 1 2 5 33 100 513 2 3 6 34 101 514 8 9 12 40 107 520 34 35 38 66 133 546 301 302 305 333 400 813 | 3 5 10 | 5 5 | 10 10 3
// [3]byte: 2 2 5 37 106 597 2 5 8 37 106 597 16 16 16 42 117 597 69 69 69 69 138 597 597 597 597 597 597 896 | 5 5 10 | 5 5 | 10 10 3
// [2]string: 1 2 5 35 107 575 2 3 6 35 107 575 14 14 14 43 107 575 71 71 71 71 151 575 575 575 575 575 575 851 | 3 5 10 | 5 5 | 10 10 3
// convert 0 0 0 0 0
// convert 1 8 1 2 1
// convert 5 8 3 4 5
// convert 7 8 4 4 7
// convert 8 8 4 4 8
// convert 9 16 5 6 9
// convert 31 32 16 16 31
// convert 32 32 16 16 32
// convert 33 48 17 20 33
// convert 100 112 50 52 100
// convert 600 640 300 320 600
// convert 5000 5376 2500 2560 5000
// const 5 5 11 11 0 0
// const append 9 16
// append string 12 16
// make 5 3 33 9 0
// grown 10 6 33 9 3
