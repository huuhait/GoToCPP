// phase: 05

// Maps print sorted by key (internal/fmtsort): the floats case, with %v %+v %#v and other verbs
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
	m := map[float64]int{2.5: 1, -1: 2, 0: 3, 1e21: 4, -1e21: 5, 0.1: 6, inf: 7, ninf: 8, nan: 9}
	fmt.Println(m, len(m))
	fmt.Printf("%v|%+v|%#v|%T\n", m, m, m, m)
	fmt.Printf("%d|%s|%x|%q|%5v|%-5v|\n", m, m, m, m, m, m)
	fmt.Println(fmt.Sprint(m) == fmt.Sprint(m), fmt.Sprint(m) == fmt.Sprintf("%v", m))
}

// Output:
// map[NaN:9 -Inf:8 -1e+21:5 -1:2 0:3 0.1:6 2.5:1 1e+21:4 +Inf:7] 9
// map[NaN:9 -Inf:8 -1e+21:5 -1:2 0:3 0.1:6 2.5:1 1e+21:4 +Inf:7]|map[NaN:9 -Inf:8 -1e+21:5 -1:2 0:3 0.1:6 2.5:1 1e+21:4 +Inf:7]|map[float64]int{NaN:9, -Inf:8, -1e+21:5, -1:2, 0:3, 0.1:6, 2.5:1, 1e+21:4, +Inf:7}|map[float64]int
// map[%!d(float64=NaN):9 %!d(float64=-Inf):8 %!d(float64=-1e+21):5 %!d(float64=-1):2 %!d(float64=0):3 %!d(float64=0.1):6 %!d(float64=2.5):1 %!d(float64=1e+21):4 %!d(float64=+Inf):7]|map[%!s(float64=NaN):%!s(int=9) %!s(float64=-Inf):%!s(int=8) %!s(float64=-1e+21):%!s(int=5) %!s(float64=-1):%!s(int=2) %!s(float64=0):%!s(int=3) %!s(float64=0.1):%!s(int=6) %!s(float64=2.5):%!s(int=1) %!s(float64=1e+21):%!s(int=4) %!s(float64=+Inf):%!s(int=7)]|map[NaN:9 -Inf:8 -0x1.b1ae4d6e2ef5p+69:5 -0x1p+00:2 0x0p+00:3 0x1.999999999999ap-04:6 0x1.4p+01:1 0x1.b1ae4d6e2ef5p+69:4 +Inf:7]|map[%!q(float64=NaN):'\t' %!q(float64=-Inf):'\b' %!q(float64=-1e+21):'\x05' %!q(float64=-1):'\x02' %!q(float64=0):'\x03' %!q(float64=0.1):'\x06' %!q(float64=2.5):'\x01' %!q(float64=1e+21):'\x04' %!q(float64=+Inf):'\a']|map[  NaN:    9  -Inf:    8 -1e+21:    5    -1:    2     0:    3   0.1:    6   2.5:    1 1e+21:    4  +Inf:    7]|map[NaN  :9     -Inf :8     -1e+21:5     -1   :2     0    :3     0.1  :6     2.5  :1     1e+21:4     +Inf :7    ]|
// true true
