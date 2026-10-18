// phase: 04
// Stderr: ^7 97 5 200 bob view 0x0 true \+1\.500000e\+000\nok\n$

// fmt prints C/C++ values by their C++ type (Phase 04 README §2.12, step 4.12): C numbers as
// the Go types of their kind and size (a C.char is a number, as in cgo), enums as numbers,
// C++ strings as strings, class types through their std::formatter (Money) or operator<<
// (Point), and raw pointers, also a char*, as addresses (<nil> for null): a char* is never read
// as a C string. The builtin println prints the numbers, enums, strings and pointers too.
package main

import (
	"fmt"

	"<vector>"
	"./cxxvals.hpp"
)

type Shape struct {
	name   string
	corner cxxvals.Point
	n      C.int
}

func main() {
	var n C.int = 7
	var c C.char = 'a'
	p := cxxvals.make_point(1, 2)
	fmt.Println(n, c, C.double(1.5), C.size_t(3), cxxvals.big(), C.uchar(200))
	fmt.Println(cxxvals.green(), cxxvals.high(), cxxvals.name(), cxxvals.view())
	fmt.Println(p, cxxvals.make_money(125), cxxvals.null_text())
	fmt.Printf("%v|%s|%q|%8v|%-8s|%d\n", p, p, p, p, p, p)
	fmt.Printf("%d %v %s|%q %5s|%d\n", cxxvals.green(), cxxvals.green(), cxxvals.green(), cxxvals.name(), cxxvals.view(), cxxvals.name())
	fmt.Printf("%v %d %s\n", cxxvals.null_text(), cxxvals.null_text(), cxxvals.null_text())
	fmt.Print(cxxvals.name(), cxxvals.view(), n, n, "\n")

	// Raw pointers print their addresses, as Go prints pointers.
	addr := fmt.Sprint(cxxvals.hello())
	cb := fmt.Sprint(cxxvals.get_callback())
	fmt.Println(len(addr) > 2 && addr[:2] == "0x", len(cb) > 2 && cb[:2] == "0x")

	// C/C++ values inside gocpp values.
	s := Shape{name: "sq", n: 4}
	s.corner = p
	fmt.Println(s)
	fmt.Printf("%+v\n", s)
	fmt.Println(&s)
	fmt.Println(fmt.Sprint(s.corner, "!", s.n), fmt.Sprintln(cxxvals.name(), n))

	println(n, c, cxxvals.green(), cxxvals.high(), cxxvals.name(), cxxvals.view(), cxxvals.null_text(), true, C.double(1.5))
	println("ok")
}

// Output:
// 7 97 1.5 3 1099511627776 200
// 5 200 bob view
// (1, 2) $1.25 <nil>
// (1, 2)|(1, 2)|"(1, 2)"|  (1, 2)|(1, 2)  |%!d(Point=(1, 2))
// 5 5 %!s(Color=5)|"bob"  view|%!d(std::string=bob)
// <nil> 0 %!s(char*=<nil>)
// bobview7 7
// true true
// {sq (1, 2) 4}
// {name:sq corner:(1, 2) n:4}
// &{sq (1, 2) 4}
// (1, 2)!4 bob 7
