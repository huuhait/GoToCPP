// phase: 09
// flags: -release

// The plain -release twin of release_identical_funcs.gp: two //go:noinline functions with the
// same body keep their own names for runtime.Caller and FuncForPC, which a plain -release program
// has too (from the symbol table). g++'s -O2 would fold them into one (-fipa-icf); gocpp builds
// every optimized g++ program with -fno-ipa-icf (ADR-136).
package main

import (
	"fmt"
	"runtime"
)

//go:noinline
func first(xs []int, i int) string {
	if i > 100 {
		fmt.Println("big", i, len(xs), xs[0], xs[len(xs)-1])
	}
	pc, _, _, _ := runtime.Caller(0)
	return fmt.Sprint(xs[i]*3, " ", runtime.FuncForPC(pc).Name())
}

//go:noinline
func second(xs []int, i int) string {
	if i > 100 {
		fmt.Println("big", i, len(xs), xs[0], xs[len(xs)-1])
	}
	pc, _, _, _ := runtime.Caller(0)
	return fmt.Sprint(xs[i]*3, " ", runtime.FuncForPC(pc).Name())
}

func main() {
	xs := []int{1, 2, 3}
	fmt.Println(first(xs, 1))
	fmt.Println(second(xs, 2))
}

// Output:
// 6 main.first
// 9 main.second
