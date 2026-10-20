// phase: 08

// Pointer methods promoted through embedded values and embedded pointers: the receiver is the field of an object that the method (or its callee) drops (ADR-086).

package main

import "fmt"

type Inner struct {
	n    int
	name string
}

type Outer struct {
	Inner
	k int
}

type OutP struct {
	*Inner
	k int
}

type Deep struct {
	OutP
	z int
}

var go1 *Outer
var go2 *OutP
var go3 *Deep
var gin *Inner

type T struct{ n int; name string }

func junk() int {
	t := 0
	for i := 0; i < 100; i++ {
		x := &T{n: i, name: "junkjunkjunk"}
		t += x.n
	}
	return t
}

func (i *Inner) drop() int {
	go1 = nil
	go2 = nil
	go3 = nil
	gin = nil
	junk()
	return i.n + len(i.name)
}

func (i *Inner) dropInner() int {
	if go2 != nil {
		go2.Inner = nil
	}
	if go3 != nil {
		go3.Inner = nil
	}
	junk()
	return i.n + len(i.name)
}

func main() {
	go1 = &Outer{Inner{1, "one"}, 1}
	fmt.Println(go1.drop())
	go2 = &OutP{&Inner{2, "two"}, 2}
	fmt.Println(go2.drop())
	go2 = &OutP{&Inner{3, "three"}, 3}
	fmt.Println(go2.dropInner())
	go3 = &Deep{OutP{&Inner{4, "four"}, 4}, 4}
	fmt.Println(go3.dropInner())
	go3 = &Deep{OutP{&Inner{5, "five"}, 5}, 5}
	fmt.Println(go3.drop())
	go1 = &Outer{Inner{6, "six"}, 6}
	gin = &go1.Inner
	fmt.Println(go1.Inner.drop())
	go1 = &Outer{Inner{7, "seven"}, 7}
	gin = &go1.Inner
	go1 = nil
	fmt.Println(gin.drop())
}

// Output:
// 4
// 5
// 8
// 8
// 9
// 9
// 12
