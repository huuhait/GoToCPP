// phase: 05

// Maps print sorted by key (internal/fmtsort): the arrays case, with %v %+v %#v and other verbs
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
	m := map[[2]int]string{{1, 2}: "a", {1, 1}: "b", {0, 9}: "c", {2, 0}: "d"}
	fmt.Println(m, len(m))
	fmt.Printf("%v|%+v|%#v|%T\n", m, m, m, m)
	fmt.Printf("%d|%s|%x|%q|%5v|%-5v|\n", m, m, m, m, m, m)
	fmt.Println(fmt.Sprint(m) == fmt.Sprint(m), fmt.Sprint(m) == fmt.Sprintf("%v", m))
}

// Output:
// map[[0 9]:c [1 1]:b [1 2]:a [2 0]:d] 4
// map[[0 9]:c [1 1]:b [1 2]:a [2 0]:d]|map[[0 9]:c [1 1]:b [1 2]:a [2 0]:d]|map[[2]int]string{[2]int{0, 9}:"c", [2]int{1, 1}:"b", [2]int{1, 2}:"a", [2]int{2, 0}:"d"}|map[[2]int]string
// map[[0 9]:%!d(string=c) [1 1]:%!d(string=b) [1 2]:%!d(string=a) [2 0]:%!d(string=d)]|map[[%!s(int=0) %!s(int=9)]:c [%!s(int=1) %!s(int=1)]:b [%!s(int=1) %!s(int=2)]:a [%!s(int=2) %!s(int=0)]:d]|map[[0 9]:63 [1 1]:62 [1 2]:61 [2 0]:64]|map[['\x00' '\t']:"c" ['\x01' '\x01']:"b" ['\x01' '\x02']:"a" ['\x02' '\x00']:"d"]|map[[    0     9]:    c [    1     1]:    b [    1     2]:    a [    2     0]:    d]|map[[0     9    ]:c     [1     1    ]:b     [1     2    ]:a     [2     0    ]:d    ]|
// true true
