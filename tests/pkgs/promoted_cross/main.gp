// phase: 06

// Promoted methods across packages (step 5.8's method tables with step 6.2's packages): a type promotes the exported and the unexported methods of an embedded type of another package and declares an unexported method of the same name (two different methods, so two members and two thunks in C++), is converted to interfaces of both packages, and is embedded in turn through a pointer and a struct in the main package.

package main

import (
	"fmt"

	"example.com/prom/base"
	"example.com/prom/mid"
)

type Local struct {
	mid.Cat
}

func main() {
	c := mid.Cat{Base: base.Base{N: "tom"}, Lives: 9}
	var a base.Animal = c
	fmt.Println(base.Describe(a), base.Describe(&c), c.Hello())
	var g base.Greeter = c
	base.Greet(g)
	base.Greet(&c)
	w := mid.Wrapper{&c, "t"}
	w.Rename("max")
	base.Greet(w)
	fmt.Println(base.Describe(w), c.N)
	l := Local{c}
	var ll base.Greeter = &l
	l.Rename("loc")
	fmt.Println(ll.Hello(), base.Describe(l))
	_, isAnimal := ll.(base.Animal)
	fmt.Println(isAnimal)
	f := c.Hello
	c.Rename("changed")
	fmt.Println(f(), c.Hello())
	b := mid.Both{g, a}
	fmt.Println(b.Hello(), b.Sound())
}

// Output:
// base-tom:meow base-tom:meow hello tom
// hello tom
// hello tom
// hello max
// base-max:meow max
// hello loc base-loc:meow
// true
// hello max hello changed
// hello tom meow
