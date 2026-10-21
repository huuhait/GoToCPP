// phase: 05

// A wrap macro's exit part (a defer around #body()) runs when the function's code panics (exit
// criterion of Phase 10): the code's own defers run when #body() unwinds, then the macro's exit
// part, then the panic goes on to the caller, or to a Recover macro that was written above VM.
// The twin is testdata/macro_twin/umacro_wrap_exit_panic.go.
package main

import "fmt"

macro VM() {
	fmt.Println("vm enter", #func())
	defer fmt.Println("vm exit", #func())
	#body()
}

macro Recover() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recovered in", #func()+":", r)
		}
	}()
	#body()
}

#VM()
func crash(n int) int {
	defer fmt.Println("own defer of crash")
	if n > 0 {
		panic(fmt.Sprint("crash ", n))
	}
	return n
}

#Recover()
#VM()
func contained(n int) (int, bool) {
	defer fmt.Println("own defer of contained")
	panic(fmt.Sprint("contained ", n))
}

func main() {
	fmt.Println(crash(0))

	func() {
		defer func() { fmt.Println("main recovered:", recover()) }()
		crash(7)
	}()

	fmt.Println(contained(3))
}

// Output:
// vm enter main.crash
// own defer of crash
// vm exit main.crash
// 0
// vm enter main.crash
// own defer of crash
// vm exit main.crash
// main recovered: crash 7
// vm enter main.contained
// own defer of contained
// vm exit main.contained
// recovered in main.contained: contained 3
// 0 false
