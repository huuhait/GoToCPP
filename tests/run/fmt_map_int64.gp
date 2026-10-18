// phase: 05

// Maps print sorted by key (internal/fmtsort): the int64 case, with %v %+v %#v and other verbs
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
	m := map[int64]bool{1 << 40: true, -1 << 40: false, 5: true}
	fmt.Println(m, len(m))
	fmt.Printf("%v|%+v|%#v|%T\n", m, m, m, m)
	fmt.Printf("%d|%s|%x|%q|%5v|%-5v|\n", m, m, m, m, m, m)
	fmt.Println(fmt.Sprint(m) == fmt.Sprint(m), fmt.Sprint(m) == fmt.Sprintf("%v", m))
}

// Output:
// map[-1099511627776:false 5:true 1099511627776:true] 3
// map[-1099511627776:false 5:true 1099511627776:true]|map[-1099511627776:false 5:true 1099511627776:true]|map[int64]bool{-1099511627776:false, 5:true, 1099511627776:true}|map[int64]bool
// map[-1099511627776:%!d(bool=false) 5:%!d(bool=true) 1099511627776:%!d(bool=true)]|map[%!s(int64=-1099511627776):%!s(bool=false) %!s(int64=5):%!s(bool=true) %!s(int64=1099511627776):%!s(bool=true)]|map[-10000000000:%!x(bool=false) 5:%!x(bool=true) 10000000000:%!x(bool=true)]|map['�':%!q(bool=false) '\x05':%!q(bool=true) '�':%!q(bool=true)]|map[-1099511627776:false     5: true 1099511627776: true]|map[-1099511627776:false 5    :true  1099511627776:true ]|
// true true
