// phase: 05

// Embedded types and promoted names that are C++ words, or like names of the runtime: they are
// escaped the way fields and methods are (class_, new_), and gp_fields still shows the Go names.
package main

import "fmt"

type class struct{ new int }

func (c class) Do() int { return c.new }
func (c class) delete() int { return -c.new }

type string_ struct{ s string }

type Holder struct {
	class
	string_
	int  int
	auto bool
}

func main() {
	h := Holder{class{4}, string_{"s"}, 1, true}
	fmt.Println(h.Do(), h.new, h.delete(), h.class, h.s, h.string_, h.int, h.auto)
	fmt.Printf("%v %+v\n", h, h)
}

// Output:
// 4 4 -4 {4} s {s} 1 true
// {{4} {s} 1 true} {class:{new:4} string_:{s:s} int:1 auto:true}
