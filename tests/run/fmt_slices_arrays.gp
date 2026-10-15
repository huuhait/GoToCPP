// phase: 03

// fmt prints slices and arrays as Go does: [1 2 3], nested ones, empty and nil ones, slices of
// strings, bools, floats and structs, byte slices (numbers with %v, a string with %s and %q),
// each element formatted by the verb, with its width and precision.
package main

import "fmt"

type pt struct{ x, y int }

func main() {
	s := []int{1, 2, 3}
	var nilS []int
	empty := []string{}
	a := [3]float64{1.5, -0.25, 1e21}
	var zero [2]bool
	grid := [][]int{{1, 2}, {3}, nil, {}}
	agrid := [2][3]int{{1, 2, 3}, {4, 5, 6}}
	fmt.Println(s, nilS, empty, a, zero)
	fmt.Println(grid, agrid, len(grid))
	fmt.Println([]string{"a", "b c", ""}, []bool{true, false}, []pt{{1, 2}, {3, 4}})
	fmt.Println([0]int{}, [1][]int{nil}, []([2]string){{"x", "y"}})
	b := []byte("hi!")
	fmt.Println(b, [3]byte{'a', 'b', 'c'}, []rune("hé"))
	fmt.Printf("%s %q %v %d\n", b, b, b, b)
	fmt.Printf("%s %q\n", [2]byte{'o', 'k'}, []string{"a\n", "b"})
	fmt.Printf("%5d|%-4d|%03d|%+d\n", s, s, s, s)
	fmt.Printf("%6.2f|%.1f|%v|%8v\n", a, a[:2], a, s)
	fmt.Printf("%s|%d|%t\n", s, []string{"x"}, []int{1})
	fmt.Printf("%v %+v %d\n", []pt{{1, 2}}, []pt{{1, 2}}, [1]pt{{5, 6}})
	sub := s[1:]
	sub[0] = 20
	fmt.Println(s, sub, s[:0], s[3:])
	fmt.Print(s, a, "\n")
	fmt.Print("x", s, 1, nilS, "\n")
}

// Output:
// [1 2 3] [] [] [1.5 -0.25 1e+21] [false false]
// [[1 2] [3] [] []] [[1 2 3] [4 5 6]] 4
// [a b c ] [true false] [{1 2} {3 4}]
// [] [[]] [[x y]]
// [104 105 33] [97 98 99] [104 233]
// hi! "hi!" [104 105 33] [104 105 33]
// ok ["a\n" "b"]
// [    1     2     3]|[1    2    3   ]|[001 002 003]|[+1 +2 +3]
// [  1.50  -0.25 1000000000000000000000.00]|[1.5 -0.2]|[1.5 -0.25 1e+21]|[       1        2        3]
// [%!s(int=1) %!s(int=2) %!s(int=3)]|[%!d(string=x)]|[%!t(int=1)]
// [{1 2}] [{x:1 y:2}] [{5 6}]
// [1 20 3] [20 3] [] []
// [1 20 3] [1.5 -0.25 1e+21]
// x[1 20 3] 1 []
