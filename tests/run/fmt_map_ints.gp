// phase: 05

// Maps print sorted by key (internal/fmtsort): the ints case, with %v %+v %#v and other verbs
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
	m := map[int]string{3: "c", -1: "a", 0: "z", 2: "b", 100: "d", -50: "e"}
	fmt.Println(m, len(m))
	fmt.Printf("%v|%+v|%#v|%T\n", m, m, m, m)
	fmt.Printf("%d|%s|%x|%q|%5v|%-5v|\n", m, m, m, m, m, m)
	fmt.Println(fmt.Sprint(m) == fmt.Sprint(m), fmt.Sprint(m) == fmt.Sprintf("%v", m))
}

// Output:
// map[-50:e -1:a 0:z 2:b 3:c 100:d] 6
// map[-50:e -1:a 0:z 2:b 3:c 100:d]|map[-50:e -1:a 0:z 2:b 3:c 100:d]|map[int]string{-50:"e", -1:"a", 0:"z", 2:"b", 3:"c", 100:"d"}|map[int]string
// map[-50:%!d(string=e) -1:%!d(string=a) 0:%!d(string=z) 2:%!d(string=b) 3:%!d(string=c) 100:%!d(string=d)]|map[%!s(int=-50):e %!s(int=-1):a %!s(int=0):z %!s(int=2):b %!s(int=3):c %!s(int=100):d]|map[-32:65 -1:61 0:7a 2:62 3:63 64:64]|map['�':"e" '�':"a" '\x00':"z" '\x02':"b" '\x03':"c" 'd':"d"]|map[  -50:    e    -1:    a     0:    z     2:    b     3:    c   100:    d]|map[-50  :e     -1   :a     0    :z     2    :b     3    :c     100  :d    ]|
// true true
