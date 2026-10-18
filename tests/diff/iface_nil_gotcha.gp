// phase: 05

// A nil pointer inside an interface is not a nil interface (Go's famous gotcha); methods of a
// nil pointer still run (step 5.4).
package main

import "fmt"

type T struct{ n int }

func (t *T) Name() string {
	if t == nil {
		return "nil T"
	}
	return "T"
}

type Namer interface{ Name() string }

type MyErr struct{ msg string }

func (e *MyErr) Error() string { return e.msg }

func mayFail(fail bool) error {
	var e *MyErr
	if fail {
		e = &MyErr{"failed"}
	}
	return e // a typed nil when !fail
}

func mayFailRight(fail bool) error {
	if fail {
		return &MyErr{"failed"}
	}
	return nil
}

func main() {
	var p *T
	var n Namer = p
	fmt.Println(p == nil, n == nil, n != nil)
	fmt.Println(n.Name())
	var n2 Namer
	fmt.Println(n2 == nil)
	n2 = p
	fmt.Println(n2 == nil)
	n2 = nil
	fmt.Println(n2 == nil)
	err := mayFail(false)
	fmt.Println(err == nil, err != nil)
	err = mayFailRight(false)
	fmt.Println(err == nil)
	err = mayFailRight(true)
	fmt.Println(err == nil, err.Error())
	var a any = p
	fmt.Println(a == nil)
	pp, ok := a.(*T)
	fmt.Println(pp == nil, ok)
	a = nil
	_, ok = a.(*T)
	fmt.Println(ok)
}
