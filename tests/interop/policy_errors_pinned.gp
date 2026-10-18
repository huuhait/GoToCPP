// phase: 04

// The policy table of docs/spec/interop.md ("Types that can't be copied"), the column of a type
// that C++ can neither copy nor move (a std::mutex, a lock): where a construct needs a copy or a
// move, the program doesn't compile, with an error at the .gp line.
// others-ok: 3 the pinned types also fail in the generated defaults and copies that follow the seeded errors (gocpp's own follow-on errors, counted in the last output line)
package main

import (
	"./objects.h"
	"<utility>"
	"<vector>"
)

type S struct {
	mu objects.Lock
}

func param(p objects.Pinned) {}

func rows() {
	x := objects.make_pinned(1)
	y := std.move(x) // ERROR "C\+\+: .*(deleted|move)"
	x = objects.make_pinned(2) // ERROR "C\+\+: .*(deleted|assignment)"
	param(x) // ERROR "C\+\+: .*(deleted|copy)"
	var s S
	t := s // ERROR "C\+\+: .*(deleted|copy)"
	_, _ = y, t
}

func deferred() {
	var z objects.Lock
	defer objects.use_pinned(objects.make_pinned(4)) // ERROR "C\+\+: defer stores the arguments of the deferred call in a frame, and this C\+\+ type can be neither moved nor copied"
	defer param(objects.make_pinned(6)) // ERROR "C\+\+: .*deleted"
	defer objects.use_lock(std.move(z)) // ERROR "C\+\+: .*(deleted|stores the arguments of the deferred call in a frame)"
}

func loops() {
	for p := objects.make_pinned(5); p.get() < 7; p.bump() { // ERROR "C\+\+: a closure that uses the variable of a 3-clause for loop needs a copy"
		f := func() { _ = p.get() }
		f()
	}
	var ps std.vector[objects.Pinned]
	for _, p := range ps { // ERROR "C\+\+: .*(deleted|copy)"
		_ = p
	}
}

func main() {
	rows()
	deferred()
	loops()
}
