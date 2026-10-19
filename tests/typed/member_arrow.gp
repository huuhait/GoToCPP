// phase: 12
// opaque: reject

// Member access is chosen from the C++ type: `.` for a class value, a reference and the own
// members of a smart pointer, `->` for a raw pointer (with Go's nil check) and for the pointee's
// members of a smart pointer or an optional.
package main

import (
	"fmt"

	"<memory>"
	"<optional>"
	"<vector>"
	"./arrow.hpp"
)

func nilCall() {
	defer func() { fmt.Println("recovered:", recover() != nil) }()
	var q *arrow.Node
	q.bump()
}

// A raw pointer that C++ gave back: the nil check is gp::nonnull's.
func nilRaw() {
	defer func() { fmt.Println("recovered raw:", recover() != nil) }()
	p := arrow.make_list(0)
	p.bump()
}

// A smart pointer with nothing in it: s->m panics like a nil dereference.
func nilSmart() {
	defer func() { fmt.Println("recovered smart:", recover() != nil) }()
	var u std.unique_ptr[arrow.Counter]
	u.inc()
}

func nilOptional() {
	defer func() { fmt.Println("recovered optional:", recover() != nil) }()
	var o std.optional[arrow.Counter]
	o.inc()
}

func main() {
	// A raw pointer: p.value is (*p).value, and the walk ends at nil.
	for p := arrow.make_list(4); p != nil; p = p.next {
		p.bump()
		fmt.Println(p.value, p.doubled(), p.next != nil)
	}

	// A struct value, its fields, a field that is a pointer.
	h := arrow.Holder{}
	h.items.push_back(7)
	h.items.push_back(9)
	h.head = arrow.make_list(2)
	h.head.bump()
	fmt.Println(h.items.size(), h.items[1], h.head.value, h.head.next.value)

	// A shared_ptr: use_count and get are its own, inc and get() of Counter are the pointee's.
	h.shared = std.make_shared[arrow.Counter]()
	h.shared.inc()
	h.shared.inc()
	fmt.Println(h.shared.use_count(), h.shared.n, h.shared.get().n)

	// A unique_ptr and an optional.
	up := std.make_unique[arrow.Counter]()
	up.inc()
	fmt.Println(up.n, up.get().n, up.get() != nil)
	opt := std.optional[arrow.Counter]{arrow.Counter{}}
	opt.inc()
	opt.inc()
	opt.inc()
	fmt.Println(opt.has_value(), opt.n, opt.value().n)

	// A container of values reached through a pointer to it.
	vp := &h.items
	vp.push_back(11)
	fmt.Println(vp.size(), vp.back())
	nilCall()
	nilRaw()
	nilSmart()
	nilOptional()
}

// Output:
// 11 22 true
// 21 42 true
// 31 62 true
// 41 82 false
// 2 9 11 20
// 1 2 2
// 1 1 true
// true 3 3
// 3 11
// recovered: true
// recovered raw: true
// recovered smart: true
// recovered optional: true
