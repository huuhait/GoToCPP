// phase: 06

// An unnamed struct type with an embedded field of another package (so with promoted methods in its method set) used by three packages: one C++ struct in the global namespace, its forwarders defined once (in the pkg_desc.cpp of the package that made it), and the same descriptor for all.

package main

import (
	"fmt"

	"example.com/anonemb/base"
	"example.com/anonemb/mk"
	"example.com/anonemb/other"
)

func main() {
	v := mk.Tagged(1)
	fmt.Println(mk.AsGetter(v), v.Get())
	var i base.Incer = &v
	i.Inc()
	fmt.Println(v.Get(), mk.AsGetter(&v))
	a := other.Wrap()
	fmt.Println(other.IsSame(a), other.IsSame(v), other.IsSame(1))
	g, ok := a.(base.Getter)
	fmt.Println(ok, g.Get())
	_, ok = a.(base.Incer)
	fmt.Println(ok)
}

// Output:
// 1 1
// 2 2
// true true false
// true 5
// false
