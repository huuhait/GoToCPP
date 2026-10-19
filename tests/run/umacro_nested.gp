// phase: 10

// Macros used inside macros (step 10.9): #file(), #line() and #func() describe the outermost use,
// the code that the programmer wrote, while #text(param) is the text of the innermost use. A lazy
// argument can be passed on to another macro's lazy parameter. The twin
// (testdata/macro_twin/umacro_nested.go) is the same program with the macros written out.

package main

import "fmt"

// Assert reports a false condition with the place of the outermost use.
macro Assert(cond bool, msg lazy string) {
	if !cond {
		fmt.Printf("assert %s at %s:%d in %s: %s\n", #text(cond), #file(), #line(), #func(), msg())
	}
}

macro Check(v int) {
	#Assert(v > 0, "v must be positive")
	#Assert(v < 10, fmt.Sprint("v=", v, " is too big"))
}

// Check2 passes its lazy parameter on, and its own text.
macro Check2(v int, name lazy string) {
	#Check(v)
	ok := v != 5
	#Assert(ok, name())
	#Assert(v%2 == 1, fmt.Sprint(#text(v), " is even"))
}

macro Deep(v int) {
	#Check2(v+1, "deep")
}

// Outer is used on top of a function and uses macros in both parts.
macro Outer() func() {
	#Assert(false, "from Outer")
	return func() {
		#Assert(false, "exit of Outer")
	}
}

macro Spawn(v int) {
	done := make(chan bool)
	go func() {
		defer close(done)
		#Check(v)
	}()
	<-done
}

#Outer()
func work(n int) {
	#Check(n)
	fmt.Println("work", n)
}

func main() {
	#Check(-1)
	#Check(10)
	#Check2(5, "name five")
	#Check2(
		4,
		"four")
	#Deep(8)
	#Deep(3)
	#Spawn(11)
	work(0)
	func() {
		#Check(-5)
	}()
	fmt.Println("done")
}

// Output:
// assert v > 0 at umacro_nested.gp:60 in main.main: v must be positive
// assert v < 10 at umacro_nested.gp:61 in main.main: v=10 is too big
// assert ok at umacro_nested.gp:62 in main.main: name five
// assert v%2 == 1 at umacro_nested.gp:63 in main.main: 4 is even
// assert v%2 == 1 at umacro_nested.gp:67 in main.main: v+1 is even
// assert v < 10 at umacro_nested.gp:68 in main.main: v=11 is too big
// assert false at umacro_nested.gp:53 in main.work: from Outer
// assert v > 0 at umacro_nested.gp:55 in main.work: v must be positive
// work 0
// assert false at umacro_nested.gp:53 in main.work: exit of Outer
// assert v > 0 at umacro_nested.gp:71 in main.main.func1: v must be positive
// done
