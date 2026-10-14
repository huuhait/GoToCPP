// fmt.Printf with characters that are no verb of Go's fmt: the phase gate allows
// them in Phase 02, and gp::fmtshim prints Go's error form for them, %!z(int=5),
// with every argument type except int64, uint64 and uintptr (fmtshim names them int
// and uint until Phase 03's type tags), missing and extra arguments, a verb of several bytes,
// and a . or a % at the end.
package main

import "fmt"

func main() {
	n, s, ok, f := 5, "str", true, 2.5
	var b uint8 = 200
	var i8 int8 = -3
	var u16 uint16 = 7
	var u uint = 9
	var f32 float32 = 0.1
	r := 'a'
	fmt.Printf("%z\n", 5)
	fmt.Printf("%z %z %z %z\n", n, s, ok, f)
	fmt.Printf("%z %z %z %z %z %z\n", b, i8, u16, u, f32, r)
	fmt.Printf("%! %y|%é|%世\n", n, s, ok, f)
	fmt.Printf("%z %d\n", n)
	fmt.Printf("%z\n", n, s)
	fmt.Printf("%v %Z %s\n", n, n, s)
	fmt.Printf("end %.", n)
	fmt.Println()
	fmt.Printf("end %", n)
	fmt.Println()
}
