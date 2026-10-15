// phase: 03

// fmt.Sprint, Sprintln and Sprintf return what Print, Println and Printf print: Sprint puts a
// space between two arguments only when neither is a string, Sprintln between all of them and a
// newline at the end; Printf's widths, precisions and flags (- + space 0) with %v %d %s %q %t %f,
// and Go's error forms for wrong, missing and extra arguments.
package main

import "fmt"

func pad(s string, n int) string {
	for len(s) < n {
		s += "."
	}
	return s
}

func main() {
	a := fmt.Sprint("a", "b", 1, 2, "c", 3.5, true, false, []int{1}, "d")
	b := fmt.Sprintln("a", "b", 1, 2, true)
	c := fmt.Sprintf("%d-%s-%v", 1, "two", 3.0)
	fmt.Println(a)
	fmt.Print(b)
	fmt.Println(c, len(c))
	fmt.Println(fmt.Sprint(), fmt.Sprintln() == "\n", fmt.Sprintf("") == "")
	fmt.Println(pad(fmt.Sprint(42), 6), pad("", 2))
	fmt.Printf("[%5d][%-5d][%05d][%+d][% d][%+05d][%-+5d]\n", 42, 42, -42, 42, 42, 42, 42)
	fmt.Printf("[%8.3f][%-8.2f][%.0f][%08.2f][%+.1f][% .2f][%.3f]\n", 3.14159, 2.5, 2.5, -1.5, 2.25, 1.0, 1e-7)
	fmt.Printf("[%6s][%-6s][%.2s][%6.1s][%q][%8q][%-8q|][%.1q]\n", "ab", "ab", "abc", "xyz", "q\"", "a", "b", "xyz")
	fmt.Printf("[%5t][%-6t][%v][%5v][%.1v][%-4v|]\n", true, false, 1 < 2, 12, "abc", 'x')
	fmt.Printf("[%3d][%10.4v][%.2v]\n", 123456, 3.14159265, 1234.5678)
	fmt.Printf("%d %s %t %f %q\n", "s", 1, 2, "x", 3.5)
	fmt.Printf("%d %d\n", 1)
	fmt.Printf("%d\n", 1, "extra", 2.5)
	fmt.Printf("%!|%z|%é|%\n", 1, 2, 3)
	fmt.Printf("100%%|%5%|%-5%|\n")
	var u8 uint8 = 200
	var i16 int16 = -300
	var f32 float32 = 0.1
	fmt.Printf("%s %s %s %s %s\n", u8, i16, f32, 'r', uint(9))
	s := fmt.Sprintf("%v|%v", []string{"a", "b"}, map[string][]int{"k": {1}})
	fmt.Println(s)
	fmt.Println(fmt.Sprintf("%-4d|%4s", 7, "go") + "!")
}

// Output:
// ab1 2c3.5 true false [1]d
// a b 1 2 true
// 1-two-3 7
//  true true
// 42.... ..
// [   42][42   ][-0042][+42][ 42][+0042][+42  ]
// [   3.142][2.50    ][2][-0001.50][+2.2][ 1.00][0.000]
// [    ab][ab    ][ab][     x]["q\""][     "a"]["b"     |]["x"]
// [ true][false ][true][   12][a][120 |]
// [123456][     3.142][1.2e+03]
// %!d(string=s) %!s(int=1) %!t(int=2) %!f(string=x) %!q(float64=3.5)
// 1 %!d(MISSING)
// 1
// %!(EXTRA string=extra, float64=2.5)%!!(int=1)|%!z(int=2)|%!é(int=3)|%!
// (MISSING)100%|%|%|
// %!s(uint8=200) %!s(int16=-300) %!s(float32=0.1) %!s(int32=114) %!s(uint=9)
// [a b]|map[k:[1]]
// 7   |  go!
