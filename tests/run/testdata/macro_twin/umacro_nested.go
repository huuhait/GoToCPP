// The hand-written Go twin of umacro_nested.gp: a macro used inside a macro passes its own place
// (file, line, function) on, and gives the text of its own arguments.

package main

import "fmt"

const file = "umacro_nested.gp"

func Assert(gp_file string, gp_line int, gp_func string, gp_text_cond string, cond bool, msg func() string) {
	if !cond {
		fmt.Printf("assert %s at %s:%d in %s: %s\n", gp_text_cond, gp_file, gp_line, gp_func, msg())
	}
}

func Check(gp_file string, gp_line int, gp_func string, v int) {
	Assert(gp_file, gp_line, gp_func, "v > 0", v > 0, func() string { return "v must be positive" })
	Assert(gp_file, gp_line, gp_func, "v < 10", v < 10, func() string { return fmt.Sprint("v=", v, " is too big") })
}

func Check2(gp_file string, gp_line int, gp_func string, gp_text_v string, v int, name func() string) {
	Check(gp_file, gp_line, gp_func, v)
	ok := v != 5
	Assert(gp_file, gp_line, gp_func, "ok", ok, func() string { return name() })
	Assert(gp_file, gp_line, gp_func, "v%2 == 1", v%2 == 1, func() string { return fmt.Sprint(gp_text_v, " is even") })
}

func Deep(gp_file string, gp_line int, gp_func string, v int) {
	Check2(gp_file, gp_line, gp_func, "v+1", v+1, func() string { return "deep" })
}

func Outer(gp_file string, gp_line int, gp_func string) func() {
	Assert(gp_file, gp_line, gp_func, "false", false, func() string { return "from Outer" })
	return func() {
		Assert(gp_file, gp_line, gp_func, "false", false, func() string { return "exit of Outer" })
	}
}

func Spawn(gp_file string, gp_line int, gp_func string, v int) {
	done := make(chan bool)
	go func() {
		defer close(done)
		Check(gp_file, gp_line, gp_func, v)
	}()
	<-done
}

func work(n int) {
	if gp_defer1 := Outer(file, 53, "main.work"); gp_defer1 != nil {
		defer gp_defer1()
	}
	Check(file, 55, "main.work", n)
	fmt.Println("work", n)
}

func main() {
	Check(file, 60, "main.main", -1)
	Check(file, 61, "main.main", 10)
	Check2(file, 62, "main.main", "5", 5, func() string { return "name five" })
	Check2(file, 63, "main.main", "4",
		4,
		func() string { return "four" })
	Deep(file, 66, "main.main", 8)
	Deep(file, 67, "main.main", 3)
	Spawn(file, 68, "main.main", 11)
	work(0)
	func() {
		Check(file, 71, "main.main.func1", -5)
	}()
	fmt.Println("done")
}
