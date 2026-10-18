// phase: 05

// Slices and arrays of numbers, strings and bools with the usual verbs: a verb applies to each
// element, with its width and precision; nil and empty slices; nested slices.
package main

import "fmt"

func main() {
	ints := []int{1, -2, 300}
	strs := []string{"a", "b c", "é"}
	flts := []float64{1, 2.5, -0.125, 1e21}
	bs := []bool{true, false}
	var nilS []int
	empty := []int{}
	for _, f := range []string{"%v", "%+v", "%#v", "%d", "%x", "%X", "%o", "%b", "%c", "%q", "%U", "%5d", "%-5d|", "%05d", "%+d", "% d", "%.2d", "%s", "%t", "%f", "%8.2f"} {
		fmt.Printf("%-6s", f)
		fmt.Printf(f, ints)
		fmt.Printf(" ")
		fmt.Printf(f, strs)
		fmt.Printf(" ")
		fmt.Printf(f, flts)
		fmt.Printf(" ")
		fmt.Printf(f, bs)
		fmt.Printf(" ")
		fmt.Printf(f, nilS)
		fmt.Printf(" ")
		fmt.Printf(f, empty)
		fmt.Printf("\n")
	}
	fmt.Println(nilS == nil, empty == nil, len(nilS), nilS, empty)
	fmt.Printf("%v %+v %#v %T\n", [][]int{{1}, {2, 3}, nil}, [][]int{{1}, {2, 3}, nil}, [][]int{{1}, {2, 3}, nil}, [][]int{})
	fmt.Printf("%v %#v %T %v\n", [0]int{}, [0]int{}, [0]string{}, [3][2]int{})
	fmt.Printf("%v %v\n", [3]string{"a", "", "c"}, [2][]string{{"x"}, nil})
}

// Output:
// %v    [1 -2 300] [a b c é] [1 2.5 -0.125 1e+21] [true false] [] []
// %+v   [1 -2 300] [a b c é] [1 2.5 -0.125 1e+21] [true false] [] []
// %#v   []int{1, -2, 300} []string{"a", "b c", "é"} []float64{1, 2.5, -0.125, 1e+21} []bool{true, false} []int(nil) []int{}
// %d    [1 -2 300] [%!d(string=a) %!d(string=b c) %!d(string=é)] [%!d(float64=1) %!d(float64=2.5) %!d(float64=-0.125) %!d(float64=1e+21)] [%!d(bool=true) %!d(bool=false)] [] []
// %x    [1 -2 12c] [61 622063 c3a9] [0x1p+00 0x1.4p+01 -0x1p-03 0x1.b1ae4d6e2ef5p+69] [%!x(bool=true) %!x(bool=false)] [] []
// %X    [1 -2 12C] [61 622063 C3A9] [0X1P+00 0X1.4P+01 -0X1P-03 0X1.B1AE4D6E2EF5P+69] [%!X(bool=true) %!X(bool=false)] [] []
// %o    [1 -2 454] [%!o(string=a) %!o(string=b c) %!o(string=é)] [%!o(float64=1) %!o(float64=2.5) %!o(float64=-0.125) %!o(float64=1e+21)] [%!o(bool=true) %!o(bool=false)] [] []
// %b    [1 -10 100101100] [%!b(string=a) %!b(string=b c) %!b(string=é)] [4503599627370496p-52 5629499534213120p-51 -4503599627370496p-55 7629394531250000p+17] [%!b(bool=true) %!b(bool=false)] [] []
// %c    [ � Ĭ] [%!c(string=a) %!c(string=b c) %!c(string=é)] [%!c(float64=1) %!c(float64=2.5) %!c(float64=-0.125) %!c(float64=1e+21)] [%!c(bool=true) %!c(bool=false)] [] []
// %q    ['\x01' '�' 'Ĭ'] ["a" "b c" "é"] [%!q(float64=1) %!q(float64=2.5) %!q(float64=-0.125) %!q(float64=1e+21)] [%!q(bool=true) %!q(bool=false)] [] []
// %U    [U+0001 U+FFFFFFFFFFFFFFFE U+012C] [%!U(string=a) %!U(string=b c) %!U(string=é)] [%!U(float64=1) %!U(float64=2.5) %!U(float64=-0.125) %!U(float64=1e+21)] [%!U(bool=true) %!U(bool=false)] [] []
// %5d   [    1    -2   300] [%!d(string=    a) %!d(string=  b c) %!d(string=    é)] [%!d(float64=    1) %!d(float64=  2.5) %!d(float64=-0.125) %!d(float64=1e+21)] [%!d(bool= true) %!d(bool=false)] [] []
// %-5d| [1     -2    300  ]| [%!d(string=a    ) %!d(string=b c  ) %!d(string=é    )]| [%!d(float64=1    ) %!d(float64=2.5  ) %!d(float64=-0.125) %!d(float64=1e+21)]| [%!d(bool=true ) %!d(bool=false)]| []| []|
// %05d  [00001 -0002 00300] [%!d(string=0000a) %!d(string=00b c) %!d(string=0000é)] [%!d(float64=00001) %!d(float64=002.5) %!d(float64=-0.125) %!d(float64=1e+21)] [%!d(bool=0true) %!d(bool=false)] [] []
// %+d   [+1 -2 +300] [%!d(string=a) %!d(string=b c) %!d(string=é)] [%!d(float64=+1) %!d(float64=+2.5) %!d(float64=-0.125) %!d(float64=+1e+21)] [%!d(bool=true) %!d(bool=false)] [] []
// % d   [ 1 -2  300] [%!d(string=a) %!d(string=b c) %!d(string=é)] [%!d(float64= 1) %!d(float64= 2.5) %!d(float64=-0.125) %!d(float64= 1e+21)] [%!d(bool=true) %!d(bool=false)] [] []
// %.2d  [01 -02 300] [%!d(string=a) %!d(string=b ) %!d(string=é)] [%!d(float64=1) %!d(float64=2.5) %!d(float64=-0.12) %!d(float64=1e+21)] [%!d(bool=true) %!d(bool=false)] [] []
// %s    [%!s(int=1) %!s(int=-2) %!s(int=300)] [a b c é] [%!s(float64=1) %!s(float64=2.5) %!s(float64=-0.125) %!s(float64=1e+21)] [%!s(bool=true) %!s(bool=false)] [] []
// %t    [%!t(int=1) %!t(int=-2) %!t(int=300)] [%!t(string=a) %!t(string=b c) %!t(string=é)] [%!t(float64=1) %!t(float64=2.5) %!t(float64=-0.125) %!t(float64=1e+21)] [true false] [] []
// %f    [%!f(int=1) %!f(int=-2) %!f(int=300)] [%!f(string=a) %!f(string=b c) %!f(string=é)] [1.000000 2.500000 -0.125000 1000000000000000000000.000000] [%!f(bool=true) %!f(bool=false)] [] []
// %8.2f [%!f(int=      01) %!f(int=     -02) %!f(int=     300)] [%!f(string=       a) %!f(string=      b ) %!f(string=       é)] [    1.00     2.50    -0.12 1000000000000000000000.00] [%!f(bool=    true) %!f(bool=   false)] [] []
// true false 0 [] []
// [[1] [2 3] []] [[1] [2 3] []] [][]int{[]int{1}, []int{2, 3}, []int(nil)} [][]int
// [] [0]int{} [0]string [[0 0] [0 0] [0 0]]
// [a  c] [[x] []]
