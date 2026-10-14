// phase: 03

// A function with defers (Phase 03 README §2.8): its body is in a C++ try block and each
// return jumps to its own exit, at the return's line, where the deferred calls run. gdb still
// stops at the body's lines and steps over them with next; a breakpoint in a deferred function
// stops when the function returns, and bt shows the function whose defer it is, at the line of
// its return (the runtime frames between are not the program's). In a function without
// results, next from the last statement goes to the closing brace, where the deferred calls
// run (the end of the body's exit).
package main

import "fmt"

func named() (n int) {
	defer set(&n, 42)
	n = 1 // gdb: print *n ~ ^\(long &\) @0x[0-9a-f]+: 0$; next 17
	return 7 // gdb: print *n ~ ^\(long &\) @0x[0-9a-f]+: 1$
}

func set(p *int, v int) {
	*p = v // gdb: print v = 42; print *p ~ ^\(long &\) @0x[0-9a-f]+: 7$; bt gpkg_main::set gpkg_main::named gpkg_main::main
}

func loop() {
	for i := range 2 {
		defer fmt.Println("loop", i) // gdb: print i = 0
	}
	fmt.Println("body") // gdb: next 29
}

func main() {
	fmt.Println(named())
	loop()
}
