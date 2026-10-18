// phase: 05

// Maps print sorted by key (internal/fmtsort): the strings case, with %v %+v %#v and other verbs
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
	m := map[string]int{"b": 2, "a": 1, "": 0, "ab": 5, "B": 3, "é": 6, "z": 7, "aa": 8}
	fmt.Println(m, len(m))
	fmt.Printf("%v|%+v|%#v|%T\n", m, m, m, m)
	fmt.Printf("%d|%s|%x|%q|%5v|%-5v|\n", m, m, m, m, m, m)
	fmt.Println(fmt.Sprint(m) == fmt.Sprint(m), fmt.Sprint(m) == fmt.Sprintf("%v", m))
}

// Output:
// map[:0 B:3 a:1 aa:8 ab:5 b:2 z:7 é:6] 8
// map[:0 B:3 a:1 aa:8 ab:5 b:2 z:7 é:6]|map[:0 B:3 a:1 aa:8 ab:5 b:2 z:7 é:6]|map[string]int{"":0, "B":3, "a":1, "aa":8, "ab":5, "b":2, "z":7, "é":6}|map[string]int
// map[%!d(string=):0 %!d(string=B):3 %!d(string=a):1 %!d(string=aa):8 %!d(string=ab):5 %!d(string=b):2 %!d(string=z):7 %!d(string=é):6]|map[:%!s(int=0) B:%!s(int=3) a:%!s(int=1) aa:%!s(int=8) ab:%!s(int=5) b:%!s(int=2) z:%!s(int=7) é:%!s(int=6)]|map[:0 42:3 61:1 6161:8 6162:5 62:2 7a:7 c3a9:6]|map["":'\x00' "B":'\x03' "a":'\x01' "aa":'\b' "ab":'\x05' "b":'\x02' "z":'\a' "é":'\x06']|map[     :    0     B:    3     a:    1    aa:    8    ab:    5     b:    2     z:    7     é:    6]|map[     :0     B    :3     a    :1     aa   :8     ab   :5     b    :2     z    :7     é    :6    ]|
// true true
