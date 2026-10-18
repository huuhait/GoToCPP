// phase: 05

// A verb applies to every field of a struct: width, precision and flags too; fields that don't
// fit the verb print Go's error form.
package main

import "fmt"

type M struct {
	N    int
	F    float64
	S    string
	B    bool
	P    struct{ A, B int }
}

func main() {
	m := M{255, 2.5, "str", true, struct{ A, B int }{1, 2}}
	for _, f := range []string{"%v", "%+v", "%#v", "%d", "%x", "%X", "%o", "%b", "%s", "%q", "%t", "%f", "%.2f", "%e", "%g", "%5v", "%-5v", "%05d", "%8.3v", "%c", "%U", "%p", "%08.3f", "%+d", "% d", "%+q", "%#x", "%6.2f"} {
		fmt.Printf("%-8s [", f)
		fmt.Printf(f, m)
		fmt.Printf("]\n")
	}
	fmt.Printf("%d %s\n", &m, &m)
}

// Output:
// %v       [{255 2.5 str true {1 2}}]
// %+v      [{N:255 F:2.5 S:str B:true P:{A:1 B:2}}]
// %#v      [main.M{N:255, F:2.5, S:"str", B:true, P:struct { A int; B int }{A:1, B:2}}]
// %d       [{255 %!d(float64=2.5) %!d(string=str) %!d(bool=true) {1 2}}]
// %x       [{ff 0x1.4p+01 737472 %!x(bool=true) {1 2}}]
// %X       [{FF 0X1.4P+01 737472 %!X(bool=true) {1 2}}]
// %o       [{377 %!o(float64=2.5) %!o(string=str) %!o(bool=true) {1 2}}]
// %b       [{11111111 5629499534213120p-51 %!b(string=str) %!b(bool=true) {1 10}}]
// %s       [{%!s(int=255) %!s(float64=2.5) str %!s(bool=true) {%!s(int=1) %!s(int=2)}}]
// %q       [{'ÿ' %!q(float64=2.5) "str" %!q(bool=true) {'\x01' '\x02'}}]
// %t       [{%!t(int=255) %!t(float64=2.5) %!t(string=str) true {%!t(int=1) %!t(int=2)}}]
// %f       [{%!f(int=255) 2.500000 %!f(string=str) %!f(bool=true) {%!f(int=1) %!f(int=2)}}]
// %.2f     [{%!f(int=255) 2.50 %!f(string=st) %!f(bool=true) {%!f(int=01) %!f(int=02)}}]
// %e       [{%!e(int=255) 2.500000e+00 %!e(string=str) %!e(bool=true) {%!e(int=1) %!e(int=2)}}]
// %g       [{%!g(int=255) 2.5 %!g(string=str) %!g(bool=true) {%!g(int=1) %!g(int=2)}}]
// %5v      [{  255   2.5   str  true {    1     2}}]
// %-5v     [{255   2.5   str   true  {1     2    }}]
// %05d     [{00255 %!d(float64=002.5) %!d(string=00str) %!d(bool=0true) {00001 00002}}]
// %8.3v    [{     255      2.5      str     true {     001      002}}]
// %c       [{ÿ %!c(float64=2.5) %!c(string=str) %!c(bool=true) { }}]
// %U       [{U+00FF %!U(float64=2.5) %!U(string=str) %!U(bool=true) {U+0001 U+0002}}]
// %p       [%!p(main.M={255 2.5 str true {1 2}})]
// %08.3f   [{%!f(int=     255) 0002.500 %!f(string=00000str) %!f(bool=0000true) {%!f(int=     001) %!f(int=     002)}}]
// %+d      [{+255 %!d(float64=+2.5) %!d(string=str) %!d(bool=true) {+1 +2}}]
// % d      [{ 255 %!d(float64= 2.5) %!d(string=str) %!d(bool=true) { 1  2}}]
// %+q      [{'\u00ff' %!q(float64=+2.5) "str" %!q(bool=true) {'\x01' '\x02'}}]
// %#x      [{0xff 0x1.4000p+01 0x737472 %!x(bool=true) {0x1 0x2}}]
// %6.2f    [{%!f(int=   255)   2.50 %!f(string=    st) %!f(bool=  true) {%!f(int=    01) %!f(int=    02)}}]
// &{255 %!d(float64=2.5) %!d(string=str) %!d(bool=true) {1 2}} &{%!s(int=255) %!s(float64=2.5) str %!s(bool=true) {%!s(int=1) %!s(int=2)}}
