// phase: 07

package main

import "fmt"

func Cnt(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		defer fmt.Println("iterator cleanup")
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

func f() {
	defer fmt.Println("defer 1 (outer, before loop)")
	for x := range Cnt(3) {
		defer fmt.Println("defer in body", x)
		fmt.Println("body", x)
	}
	fmt.Println("after loop")
	defer fmt.Println("defer 2 (outer, after loop)")
}

func nested() {
	for x := range Cnt(2) {
		defer fmt.Println("outer body defer", x)
		for y := range Cnt(2) {
			defer fmt.Println("inner body defer", x, y)
		}
	}
	fmt.Println("nested loop done")
}

func closure() (res string) {
	defer func() { res += "!" }()
	for x := range Cnt(3) {
		defer func() { res += fmt.Sprint(x) }()
	}
	return "r"
}

func withBreak() {
	for x := range Cnt(5) {
		defer fmt.Println("kept", x)
		if x == 1 {
			break
		}
	}
	fmt.Println("withBreak end")
}

func main() {
	f()
	fmt.Println("--")
	nested()
	fmt.Println("--")
	fmt.Println(closure())
	fmt.Println("--")
	withBreak()
}

// Output:
// body 0
// body 1
// body 2
// iterator cleanup
// after loop
// defer 2 (outer, after loop)
// defer in body 2
// defer in body 1
// defer in body 0
// defer 1 (outer, before loop)
// --
// iterator cleanup
// iterator cleanup
// iterator cleanup
// nested loop done
// inner body defer 1 1
// inner body defer 1 0
// outer body defer 1
// inner body defer 0 1
// inner body defer 0 0
// outer body defer 0
// --
// iterator cleanup
// r210!
// --
// iterator cleanup
// withBreak end
// kept 1
// kept 0
