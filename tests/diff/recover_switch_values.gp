// phase: 05

// A type switch on a recovered value: int, string, error, a custom error struct and a pointer,
// nil for no panic (steps 5.5 with 5.10).
package main

import "fmt"

type Code struct{ N int }

func (c Code) Error() string { return fmt.Sprint("code ", c.N) }

type Ptr struct{ s string }

func (p *Ptr) Error() string { return "ptr " + p.s }

func classify(f func()) string {
	var r any
	func() {
		defer func() { r = recover() }()
		f()
	}()
	switch v := r.(type) {
	case nil:
		return "no panic"
	case int:
		return fmt.Sprint("int ", v)
	case string:
		return "string " + v
	case Code:
		return "Code struct " + v.Error()
	case *Ptr:
		return "ptr " + v.s
	case error:
		return "error " + v.Error()
	default:
		return "other"
	}
}

func main() {
	var m map[int]int
	var p *Ptr
	fmt.Println(classify(func() {}))
	fmt.Println(classify(func() { panic(42) }))
	fmt.Println(classify(func() { panic("boom") }))
	fmt.Println(classify(func() { panic(Code{5}) }))
	fmt.Println(classify(func() { panic(&Ptr{"x"}) }))
	fmt.Println(classify(func() { m[1] = 2 }))
	fmt.Println(classify(func() { _ = p.s }))
	fmt.Println(classify(func() { panic(3.5) }))
	fmt.Println(classify(func() { var a any = 1; _ = a.(string) }))
}
