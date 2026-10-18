// phase: 05

// Maps print sorted by key (internal/fmtsort): the nested case, with %v %+v %#v and other verbs
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
	m := map[string]map[int]string{"x": {2: "b", 1: "a"}, "a": {}, "m": nil}
	fmt.Println(m, len(m))
	fmt.Printf("%v|%+v|%#v|%T\n", m, m, m, m)
	fmt.Printf("%d|%s|%x|%q|%5v|%-5v|\n", m, m, m, m, m, m)
	fmt.Println(fmt.Sprint(m) == fmt.Sprint(m), fmt.Sprint(m) == fmt.Sprintf("%v", m))
}

// Output:
// map[a:map[] m:map[] x:map[1:a 2:b]] 3
// map[a:map[] m:map[] x:map[1:a 2:b]]|map[a:map[] m:map[] x:map[1:a 2:b]]|map[string]map[int]string{"a":map[int]string{}, "m":map[int]string(nil), "x":map[int]string{1:"a", 2:"b"}}|map[string]map[int]string
// map[%!d(string=a):map[] %!d(string=m):map[] %!d(string=x):map[1:%!d(string=a) 2:%!d(string=b)]]|map[a:map[] m:map[] x:map[%!s(int=1):a %!s(int=2):b]]|map[61:map[] 6d:map[] 78:map[1:61 2:62]]|map["a":map[] "m":map[] "x":map['\x01':"a" '\x02':"b"]]|map[    a:map[]     m:map[]     x:map[    1:    a     2:    b]]|map[a    :map[] m    :map[] x    :map[1    :a     2    :b    ]]|
// true true
