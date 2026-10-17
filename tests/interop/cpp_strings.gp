// phase: 04

// gocpp strings and C++ strings (step 4.9): a gocpp string converts implicitly to a
// std::string_view parameter, and explicitly to a std::string (std.string(s)); string(x) copies a
// std::string or std::string_view into a gocpp string. A gocpp string that comes through C++ (a
// field of an element of a std.vector[*Node]) compares with a string constant, as in Go.
package main

import (
	"fmt"

	"<map>"
	"<optional>"
	"<vector>"
	"./cgostr.h"
)

type Node struct {
	Name string
}

const typed string = "typed"
const untyped = "untyped"

func main() {
	s := "banana"
	fmt.Println(int(cgostr.sv_count(s, 'a')), int(cgostr.sv_count("lit", 'l')))
	shout := string(cgostr.sv_shout(s))
	fmt.Println(shout, len(shout))
	fmt.Println(string(cgostr.sv_word()) + "s")
	cs := std.string(s)
	cs.append("!")
	fmt.Println(string(cs), int(cs.size()), s)
	fmt.Println(string(cgostr.str_join(std.string(s), std.string("x"))))
	empty := string(std.string(""))
	fmt.Println(empty == "", len(empty))

	var nodes std.vector[*Node]
	nodes.push_back(&Node{Name: "a"})
	nodes.push_back(&Node{Name: "b\x00c"})
	fmt.Println(bool(nodes[0].Name == "a"), bool(nodes[0].Name != "a"), bool("a" == nodes[0].Name))
	fmt.Println(bool(nodes[1].Name == "b\x00c"), bool(nodes[1].Name == "b"), bool(nodes[0].Name < nodes[1].Name), bool(nodes[1].Name > "b"))
	name := string(nodes[1].Name)
	fmt.Println(len(name), name[2:])

	// At C++ parameters both constants are C literals. A gp::string parameter copies them.
	var names std.vector[string]
	names.push_back(typed)
	names.push_back(untyped)
	names.emplace_back("a\x00b")
	fmt.Println(string(names[0]), string(names[1]), len(string(names[2])))
	var ages std.map[string, int]
	ages[typed] = 7
	ages[untyped] = 8
	fmt.Println(int(ages[typed]), int(ages[untyped]))
	var value std.optional[string]
	value.emplace(typed)
	fmt.Println(string(value.value()))
}

// Output:
// 3 1
// banana! 7
// words
// banana! 7 banana
// banana+x
// true 0
// true false true
// true false true true
// 3 c
// typed untyped 3
// 7 8
// typed
