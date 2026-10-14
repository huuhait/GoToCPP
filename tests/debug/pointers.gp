// phase: 03

// A boxed variable (its address is taken, Phase 03 README §2.2) is a gp::ptr to its box in
// C++, under its Go name: "print *x" shows the value, as for a Go pointer p, and "print x"
// shows the gp::ptr, a std::shared_ptr: here (the test's gdb runs without the libstdc++
// pretty-printers) its raw fields, _M_ptr the address of the value; with the printers,
// "std::shared_ptr<long> (use count 2, weak count 0) = {get() = 0x...}". A boxed parameter
// keeps its name: the C++ parameter n is copied into its box, a variable n of a block around
// the body, so the frame line shows the value it came with (twice (n=21), like Go's
// tracebacks) and "print n" in the body shows the box. (A gdb pretty-printer that shows a box
// as its value is Phase 13 work.)
//
// twice returns p, not &n: g++ 15 at -O0 gives a variable of a nested block that is returned by
// name (NRVO, here the box n in the block around the body) a wrong DWARF location, so gdb
// would show garbage for it; clang++ is right. (-fno-elide-constructors avoids it.)
package main

import "fmt"

func main() {
	x := 41
	p := &x
	*p += 1
	fmt.Println(x, *p) // gdb: print *x ~ ^\(long &\) @0x[0-9a-f]+: 42$; print *p ~ ^\(long &\) @0x[0-9a-f]+: 42$; print x ~ ^\{gp_sp_ = .*_M_ptr = 0x[0-9a-f]+,
	fmt.Println(*twice(21), *exclaim("hi"))
}

func twice(n int) *int {
	n *= 2
	p := &n
	return p // gdb: print *n ~ ^\(long &\) @0x[0-9a-f]+: 42$; print n ~ ^\{gp_sp_ = .*_M_ptr = 0x[0-9a-f]+,
}

func exclaim(s string) *string {
	*(&s) += "!"
	p := &s
	return p // gdb: print *s ~ ^\(gp::string &\) @0x[0-9a-f]+: \{
}
