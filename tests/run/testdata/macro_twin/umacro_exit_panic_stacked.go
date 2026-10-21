// The hand-written Go twin of umacro_exit_panic_stacked.gp: the macro lines on top of a function are
// applied from the last line to the first, so the first line is the outermost deferred call.

package main

import "fmt"

func Trace(gp_file string, gp_line int, gp_func string, tag string) func() {
	fmt.Println("enter", tag)
	return func() { fmt.Println("exit ", tag) }
}

func Safe(gp_file string, gp_line int, gp_func string) func() {
	return func() {
		if r := recover(); r != nil {
			fmt.Println("safe recovered:", r)
		}
	}
}

func guarded() int {
	if gp_defer1 := Trace("umacro_exit_panic_stacked.gp", 26, "main.guarded", "outer"); gp_defer1 != nil {
		defer gp_defer1()
	}
	if gp_defer2 := Safe("umacro_exit_panic_stacked.gp", 27, "main.guarded"); gp_defer2 != nil {
		defer gp_defer2()
	}
	if gp_defer3 := Trace("umacro_exit_panic_stacked.gp", 28, "main.guarded", "inner"); gp_defer3 != nil {
		defer gp_defer3()
	}
	defer fmt.Println("own defer of guarded")
	panic("boom")
}

func unguarded() int {
	if gp_defer1 := Trace("umacro_exit_panic_stacked.gp", 34, "main.unguarded", "outer"); gp_defer1 != nil {
		defer gp_defer1()
	}
	if gp_defer2 := Trace("umacro_exit_panic_stacked.gp", 35, "main.unguarded", "inner"); gp_defer2 != nil {
		defer gp_defer2()
	}
	defer fmt.Println("own defer of unguarded")
	panic("bang")
}

func main() {
	fmt.Println("guarded:", guarded())

	func() {
		defer func() { fmt.Println("main recovered:", recover()) }()
		unguarded()
	}()
}
