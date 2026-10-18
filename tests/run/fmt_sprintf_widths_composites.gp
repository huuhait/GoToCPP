// phase: 05

// Width, precision and flags apply to each element of a composite value, to each key and value of
// a map, and to each field of a struct, not to the whole.
package main

import "fmt"

type P struct {
	X float64
	Y string
	Z []int
}

func main() {
	p := P{3.14159, "ab", []int{1, 22}}
	fmt.Printf("[%8.2v]\n[%-8v]\n[%08v]\n[%+v]\n[%+.1f]\n", p, p, p, p, p)
	fmt.Printf("[%5v]\n[%-5v]\n[%05d]\n[%6.3v]\n", []int{1, 22}, []int{1, 22}, []int{1, 22}, []float64{1.23456, 2})
	fmt.Printf("[%5v]\n[%-5v]\n[%5s]\n", map[string]int{"a": 1, "bb": 22}, map[string]int{"a": 1}, map[string]string{"k": "v"})
	fmt.Printf("[%5v]\n[%-7v]\n[%3d]\n", [2][2]int{{1, 2}, {3, 4}}, struct{ A, B int }{1, 2}, &struct{ A, B int }{1, 2})
	fmt.Printf("[%6.2f]\n[%6.2f]\n[%x]\n[%X]\n", []float64{1.5, 2.25}, [2]float64{3, 4}, []int{255, 16}, []string{"hi", "yo"})
	fmt.Printf("[%10v]\n[%-10v]\n", "str", "str")
	fmt.Printf("[%10v]\n[%-10v]\n", true, nil)
	fmt.Printf("[%q]\n[%5q]\n[%x]\n[% x]\n[%#x]\n", []string{"a", "b"}, []string{"a"}, "hello", "hello", "hello")
	fmt.Printf("[% d]\n[%+d]\n[%+.1e]\n", []int{1, -2}, []int{1, -2}, []float64{1234.5})
}

// Output:
// [{     3.1       ab [      01       22]}]
// [{3.14159  ab       [1        22      ]}]
// [{03.14159 000000ab [00000001 00000022]}]
// [{X:3.14159 Y:ab Z:[1 22]}]
// [{+3.1 %!f(string=a) [%!f(int=+1) %!f(int=+22)]}]
// [[    1    22]]
// [[1     22   ]]
// [[00001 00022]]
// [[  1.23      2]]
// [map[    a:    1    bb:   22]]
// [map[a    :1    ]]
// [map[    k:    v]]
// [[[    1     2] [    3     4]]]
// [{1       2      }]
// [&{  1   2}]
// [[  1.50   2.25]]
// [[  3.00   4.00]]
// [[ff 10]]
// [[6869 796F]]
// [       str]
// [str       ]
// [      true]
// [<nil>     ]
// [["a" "b"]]
// [[  "a"]]
// [68656c6c6f]
// [68 65 6c 6c 6f]
// [0x68656c6c6f]
// [[ 1 -2]]
// [[+1 -2]]
// [[+1.2e+03]]
