// phase: 05

// Macros of another package (step 10.9): what the use can write, and what it can call.

package main

import "example.com/mcross/util"

type name string

func (n name) String() string { return string(n) }

func main() {
	#util.hidden()            // ERROR "macro hidden is not exported by package util: it can only be used there"
	_ = #util.Take(nil)       // ERROR "the type secret of the lazy parameter f is not exported by package util"
	_ = #util.Show(name("x")) // ERROR "the lazy parameter f has the type fmt.Stringer, and this file does not import \x22fmt\x22"
	_ = #util.Fine(util.Public{N: 3})
	_ = #nothing.Fine(util.Public{}) // ERROR "#nothing.Fine: nothing is not an imported package"
}
