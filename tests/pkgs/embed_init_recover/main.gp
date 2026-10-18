// phase: 06

// Embedding across packages, the init order of the packages (variables, then init functions, imports first: the order of Go 1.21 and later), and recover in a function of another package: defer pkg.F() with a recover inside F, a deferred method of a type of another package, and a panic that crosses package boundaries.

package main

import (
	"fmt"

	"example.com/app/base"
	"example.com/app/leaf"
	"example.com/app/mid"
	"example.com/app/safe"
)

func boom() {
	defer safe.Recover()
	panic("boom")
}

func boom2() {
	c := &safe.Closer{"c1"}
	defer c.Close()
	panic("boom2")
}

func boom3() {
	f := safe.Recover // a func value of an aware function of another package
	defer f()
	panic("boom3")
}

func boom4() {
	var i interface{ Close() } = &safe.Closer{"c4"} // a method called through an interface
	defer i.Close()
	panic("boom4")
}

func boom5() (err error) {
	defer func() { // not aware of the callee: recover in a function called by the deferred one does not count
		safe.Recover() // called, not deferred: recover returns nil here
		if r := recover(); r != nil {
			err = &safe.RecErr{V: r}
		}
	}()
	panic("boom5")
}

func main() {
	fmt.Println(base.Trace, mid.B, leaf.CC)
	d := base.Dog{1, "rex"}
	fmt.Println(d.Sound(), base.Describe(d))
	d.SetName("max")
	fmt.Println(d.Name, base.Describe(d))
	c := mid.Cat{base.Base{2, "tom"}, 9}
	fmt.Println(c.Hello(), c.Sound(), c.Name, c.ID)
	pp := base.Puppy{base.Base{3, "pup"}}
	p := mid.Pet{&pp, "t"}
	p.SetName("pip")
	fmt.Println(p.Hello(), p.Tag, p.Name, pp.Name)
	var a base.Animal = d
	fmt.Println(a.Sound())
	fmt.Println(leaf.Happy, leaf.Sad)
	boom()
	boom2()
	boom3()
	boom4()
	fmt.Println(boom5())
	err := safe.Try(func() { var m map[string]int; m["a"] = 1 })
	fmt.Println(err)
}

// Output:
// [base.A base.init leaf.init mid.init] 11 101
// woof rex:woof
// max max:woof
// hello tom meow tom 2
// hello pip t pip pip
// woof
// happy sad
// safe.Recover got boom
// closer c1 recovered boom2
// safe.Recover got boom3
// closer c4 recovered boom4
// recovered: boom5
// recovered: assignment to entry in nil map
