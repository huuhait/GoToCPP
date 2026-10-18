// phase: 05

// Maps print sorted by key (internal/fmtsort): the float32 case, with %v %+v %#v and other verbs
// applied to keys and values.
package main

import "fmt"

type pt struct{ x, y int }

type Color int

const (
	Red Color = iota
	Green
	Blue
)

func (c Color) String() string { return [...]string{"red", "green", "blue"}[c] }

type Level uint8

func main() {
	zero := 0.0
	inf := 1 / zero
	ninf := -1 / zero
	nan := zero / zero
	nzero := -zero
	_, _, _, _ = inf, ninf, nan, nzero
	m := map[float32]string{0.5: "h", 0.25: "q", -3: "n"}
	fmt.Println(m, len(m))
	fmt.Printf("%v|%+v|%#v|%T\n", m, m, m, m)
	fmt.Printf("%d|%s|%x|%q|%5v|%-5v|\n", m, m, m, m, m, m)
	fmt.Println(fmt.Sprint(m) == fmt.Sprint(m), fmt.Sprint(m) == fmt.Sprintf("%v", m))
}

// Output:
// map[-3:n 0.25:q 0.5:h] 3
// map[-3:n 0.25:q 0.5:h]|map[-3:n 0.25:q 0.5:h]|map[float32]string{-3:"n", 0.25:"q", 0.5:"h"}|map[float32]string
// map[%!d(float32=-3):%!d(string=n) %!d(float32=0.25):%!d(string=q) %!d(float32=0.5):%!d(string=h)]|map[%!s(float32=-3):n %!s(float32=0.25):q %!s(float32=0.5):h]|map[-0x1.8p+01:6e 0x1p-02:71 0x1p-01:68]|map[%!q(float32=-3):"n" %!q(float32=0.25):"q" %!q(float32=0.5):"h"]|map[   -3:    n  0.25:    q   0.5:    h]|map[-3   :n     0.25 :q     0.5  :h    ]|
// true true
