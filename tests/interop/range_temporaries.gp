// phase: 04

// The range expression of a range over a C/C++ value is evaluated once (step 4.7), and what it
// refers to lives as long as the loop, which an ASan run checks: a temporary container (held
// inside gp::range by value), a member of a temporary (make_box().items: the box goes into a
// named temporary before the loop), a member of a gocpp object whose only owner is the
// temporary pointer that a call returns (only().items), and a reference that a C++ function
// returns into its temporary arguments (longer(squares(2), squares(3))).
package main

import (
	"fmt"

	"./crange.h"
	"<array>"
	"<string>"
	"<string_view>"
	"<vector>"
)

type Node struct {
	items std.vector[std.string]
}

// only returns a new Node: the caller's temporary is its only owner.
func only() *Node {
	n := new(Node)
	n.items.push_back(std.string("a long string, on the heap"))
	n.items.push_back(std.string("another long string, on the heap"))
	return n
}

var calls = 0

func counted() std.vector[C.int] {
	calls++
	return crange.squares(3)
}

func main() {
	fmt.Print("squares")
	for i, x := range crange.squares(4) {
		fmt.Print(" ", int(i), ":", int(x))
	}
	fmt.Println()
	fmt.Print("members")
	for _, s := range crange.make_box().items {
		fmt.Print(" ", int(s.size()))
	}
	for _, s := range only().items {
		fmt.Print(" ", int(s.size()))
	}
	fmt.Println()
	fmt.Print("others")
	for _, x := range crange.longer(crange.squares(2), crange.squares(3)) {
		fmt.Print(" ", int(x))
	}
	for _, x := range std.vector[C.int](2, 7) {
		fmt.Print(" ", int(x))
	}
	for i := range crange.make_list() {
		fmt.Print(" ", int(i))
	}
	for _, x := range crange.evens() {
		crange.evens().push_back(x + 6) // a reference to one vector: grows it, size read once
	}
	fmt.Println("", int(crange.evens().size()))
	for range counted() {
	}
	fmt.Println(calls) // evaluated once
	// Retain a dereference's unique_ptr owner and a temporary callable returning a member.
	sum := 0
	for _, x := range *crange.vector_owner() {
		sum += int(x)
	}
	for _, x := range crange.make_reader()() {
		sum += int(x)
	}
	// The string_view constructor does not own its temporary string argument.
	for _, c := range std.string_view(std.string("a long heap-backed string that outlives every iteration")) {
		sum += int(c)
	}
	// The first constructor is hoisted by Go's argument order. Retain its backing
	// string before creating that temporary view, not after its statement ends.
	for _, c := range crange.longer_view(std.string_view(std.string("this long heap-backed first argument is longer than the second one")), std.string_view(std.string("a long heap-backed second argument"))) {
		sum += int(c)
	}
	// The integrated braced constructor syntax has the same backing-lifetime rule.
	for _, c := range (std.string_view{std.string("braced constructor with long heap-backed storage")}) {
		sum += int(c)
	}
	for _, c := range (std.array[C.char, 3]{1, 2, 3}) {
		sum += int(c) // keep constant narrowing legal in the braced initializer
	}
	fmt.Println("owners", sum)
	owner := crange.vector_owner()
	for i, x := range *owner {
		(*owner)[i] = x + 1 // no container copy, and no move from the lvalue owner
	}
	fmt.Println("lvalue", int((*owner)[0]), int((*owner).size()))
}

// Output:
// squares 0:0 1:1 2:4 3:9
// members 20 30 26 32
// others 0 1 4 7 7 0 1 2 6
// 1
// owners 16547
// lvalue 8 20
