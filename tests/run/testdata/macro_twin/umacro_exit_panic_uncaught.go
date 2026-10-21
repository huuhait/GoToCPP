// The hand-written Go twin of umacro_exit_panic_uncaught.gp: the macro is a function with the place
// of its use as arguments, and its use on top of a function is the first statement of the function
// (a func() result is deferred unless it is nil).

package main

import "fmt"

func Trace(gp_file string, gp_line int, gp_func string) func() {
	fmt.Println("enter", gp_func)
	return func() { fmt.Println("exit ", gp_func) }
}

func explode() {
	if gp_defer1 := Trace("umacro_exit_panic_uncaught.gp", 19, "main.explode"); gp_defer1 != nil {
		defer gp_defer1()
	}
	defer fmt.Println("own defer of explode")
	panic("boom")
}

func main() {
	fmt.Println("before")
	explode()
	fmt.Println("not reached")
}
