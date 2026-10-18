// phase: 06

// Two packages with the same name (util) at different import paths, each with a type T, a function F, a variable and an interface: the namespaces and descriptors are apart, so == of interface values, assertions, type switches and map keys tell the two T apart, and %T-like names are Go's.

package main

import (
	"fmt"

	xu "example.com/same/x/util"
	yu "example.com/same/y/util"
)

func main() {
	fmt.Println(xu.F(), yu.F(), xu.T{1}.Name(), yu.T{2}.Name())
	var a, b any = xu.T{1}, yu.T{1}
	fmt.Println(a == b, a == any(xu.T{1}))
	_, ok := a.(yu.T)
	_, ok2 := a.(xu.T)
	_, ok3 := b.(xu.I)
	_, ok4 := a.(yu.I)
	fmt.Println(ok, ok2, ok3, ok4)
	switch b.(type) {
	case xu.T:
		fmt.Println("x")
	case yu.T:
		fmt.Println("y")
	}
	fmt.Println(xu.Reg, yu.Reg)
	m := map[any]string{a: "x", b: "y"}
	fmt.Println(len(m), m[xu.T{1}], m[yu.T{1}])
}

// Output:
// init x/util
// init y/util
// x.F y.F x.T 1 y.T 2
// false true
// false true true true
// y
// map[x:1] map[y:1]
// 2 x y
