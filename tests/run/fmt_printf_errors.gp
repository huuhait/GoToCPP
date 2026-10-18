// phase: 05

// Printf's error forms: %!d(string=x) wrong type, %!d(MISSING), %!(EXTRA int=2, string=y),
// %!(NOVERB), %!z(int=1) unknown verb, %!(BADWIDTH), %!(BADPREC), %!d(BADINDEX), nil operands.
package main

import "fmt"

type S string

type T struct{ A int }

func main() {
	fmt.Printf("%d %s\n", "x", 5)
	fmt.Printf("%d %d\n", 1)
	fmt.Printf("%d\n", 1, 2, "y", nil, S("s"), T{3})
	fmt.Printf("%z %!\n", 1)
	fmt.Printf("%d %")
	fmt.Println()
	fmt.Printf("%")
	fmt.Println()
	fmt.Printf("%*d|%-*d|%.*f\n", 5, 42, 5, 42, 2, 3.14159)
	fmt.Printf("%*d\n", "x", 42)
	fmt.Printf("%.*d\n", "x", 42)
	fmt.Printf("%*d|\n", -5, 42)
	fmt.Printf("%.*d\n", -1, 42)
	fmt.Printf("%*d\n")
	fmt.Printf("%.*d %d\n", 3)
	fmt.Printf("%[3]d %[1]d\n", 1, 2)
	fmt.Printf("%[0]d %[x]d %[1\n", 1)
	fmt.Printf("%[1]d %[1]d %[2]d\n", 1, 2)
	fmt.Printf("%[2]d %d %d\n", 1, 2, 3)
	fmt.Printf("%d %[1]d %d\n", 10, 20)
	fmt.Printf("%[1]*d|%[3]*.[2]*[1]f|\n", 7, 2, 12.0)
	fmt.Printf("%[2]*[1]d|\n", 5, 6)
	fmt.Printf("%d %d %[1]d\n", 1, 2, 3)
	fmt.Printf("%[2]d\n", 1, 2, 3)
	fmt.Printf("%d%%\n", 100)
	fmt.Printf("%5%|%-5%|%05%\n")
	fmt.Printf("%s %v %d\n", nil, nil, nil)
	fmt.Printf("%!d %!%\n", 1)
	fmt.Printf("%q\n", fmt.Sprintf("\xff%d\xc3", 1))
	fmt.Printf("%é %日\n", 1, 2)
	fmt.Printf("%.d|%.0d|%5.d|\n", 0, 0, 0)
	fmt.Printf("%q %q\n", fmt.Sprintf("%99999999d", 1), fmt.Sprintf("%.99999999f", 1.0))
	fmt.Printf("%-+ 08.3d|\n", 5)
	fmt.Printf("%#+ -0x|\n", 255)
}

// Output:
// %!d(string=x) %!s(int=5)
// 1 %!d(MISSING)
// 1
// %!(EXTRA int=2, string=y, <nil>, main.S=s, main.T={3})%!z(int=1) %!!(MISSING)
// %!d(MISSING) %!(NOVERB)
// %!(NOVERB)
//    42|42   |3.14
// %!(BADWIDTH)42
// %!(BADPREC)42
// 42   |
// %!(BADPREC)42
// %!(BADWIDTH)%!d(MISSING)
// %!d(MISSING) %!d(MISSING)
// %!d(BADINDEX) 1
// %!d(BADINDEX) %!d(BADINDEX) %!
// (BADINDEX)1 1 2
// 2 3 %!d(MISSING)
// 10 10 20
//       2|%!(BADWIDTH)%!f(int=07)|
//      5|
// 1 2 1
// 2
// 100%
// %|%|%
// %!s(<nil>) <nil> %!d(<nil>)
// %!!(int=1)d %!!(MISSING)%!
// (MISSING)"\xff1\xc3"
// %!é(int=1) %!日(int=2)
// ||     |
// "%!(NOVERB)%!(EXTRA int=1)" "%!(NOVERB)%!(EXTRA float64=1)"
// +005    |
// +0xff|
