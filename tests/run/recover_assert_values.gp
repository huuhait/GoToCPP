// phase: 05

// Recovered values and interfaces (steps 5.4/5.5 with 5.10): a failed assertion is an error whose
// text is Go's, a runtime error is an error, a custom error keeps its dynamic type and can be
// asserted back.
package main

import "fmt"

type Shape interface{ Area() int }
type Named interface{ Name() string }
type Sq int

func (s Sq) Area() int { return int(s * s) }

type MyErr struct{ Code int }

func (e *MyErr) Error() string { return fmt.Sprint("myerr ", e.Code) }

func try(f func()) (r any) {
	defer func() { r = recover() }()
	f()
	return nil
}

func show(r any) {
	err, ok := r.(error)
	if !ok {
		fmt.Println("not an error:", r == nil)
		return
	}
	fmt.Println("error:", err.Error())
}

func main() {
	var x any = "text"
	var s Shape = Sq(2)
	var nilShape Shape
	var arr []int
	idx := 3
	show(try(func() { _ = x.(int) }))
	show(try(func() { _ = s.(Named) }))
	show(try(func() { _ = nilShape.(Sq) }))
	show(try(func() { _ = arr[idx] }))
	show(try(func() { var m map[string]int; m["a"] = 1 }))
	show(try(func() { panic(&MyErr{7}) }))
	show(try(func() { panic("a string") }))
	show(try(func() {}))
	r := try(func() { panic(&MyErr{9}) })
	if me, ok := r.(*MyErr); ok {
		fmt.Println("code", me.Code)
	}
	var e error = r.(error)
	fmt.Println(e == r, e.(*MyErr).Code)
	r = try(func() { _ = x.(Shape) })
	_, isMy := r.(*MyErr)
	fmt.Println(isMy, r.(error).Error())
}

// Output:
// error: interface conversion: interface {} is string, not int
// error: interface conversion: main.Sq is not main.Named: missing method Name
// error: interface conversion: main.Shape is nil, not main.Sq
// error: runtime error: index out of range [3] with length 0
// error: assignment to entry in nil map
// error: myerr 7
// not an error: false
// not an error: true
// code 9
// true 9
// false interface conversion: string is not main.Shape: missing method Area
