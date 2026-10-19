// phase: 10

// The rules for using a macro (step 10.9): where it can be used, what it gets, and what it
// can not be.

package main

import "fmt"

macro Plain(a int, b string) int { return a }

macro Rest(a int, xs ...int) int { return a + len(xs) }

macro Cleanup() func() { return func() {} }

macro Wrapper() { #body() }

macro Lazy(s lazy string) string { return s() }

func must() (int, error) { return 1, nil }

type T struct{}

#Plain(1, "x") // ERROR "only #Derive can be used there"
type Typed struct{}

#Derive(Missing) // ERROR "#Derive: Missing is not the name of a generator"
type Level int

#Wrapper() // ERROR "#Wrapper: the func noBody has no body"
func noBody()

#Plain(1) // ERROR "#Plain: not enough arguments \(have 1, want 2\)"
func wrongOnTop() {}

#fmt.Println(1) // ERROR "undefined macro #fmt.Println: package fmt declares no macro Println"
func notAMacro() {}

#Missing() // ERROR "undefined macro #Missing"
func unknownOnTop() {}

#file() // ERROR "#file is a built-in macro for use inside code, not on top of a func"
func builtinOnTop() {}

#Wrapper() // ERROR "#Wrapper is a wrap macro: it can't be used on top of the must function mustWrap!"
func mustWrap!() int {
	return 1
}

#Wrapper()
func direct() {
	defer func() { recover() }() // fine: recover in a deferred func literal
	recover()                    // ERROR "recover\(\) called directly by direct, which has the wrap macro #Wrapper"
}

func main() {
	#Plain(1)         // ERROR "#Plain: not enough arguments \(have 1, want 2\)"
	#Plain(1, "x", 3) // ERROR "#Plain: too many arguments \(have 3, want 2\)"
	#Rest()           // ERROR "#Rest: not enough arguments \(have 0, want at least 1\)"
	_ = #Rest(1, 2, 3)
	_ = #Rest(1, []int{2}...)
	_ = #Rest(1, 2, []int{3}...) // ERROR "#Rest: the arguments with ... must be 2, one for each parameter \(have 3\)"
	_ = #Plain(1, "x"...)        // ERROR "#Plain: cannot use ... in the arguments: the macro is not variadic"
	_ = #Plain[int](1, "") // ERROR "macro Plain is not generic"

	#Cleanup()         // ERROR "#Cleanup returns a func\(\): as a statement"
	defer #Plain(1, "") // ERROR "#Plain can't be the call of a defer or go statement"
	go #Plain(1, "")    // ERROR "#Plain can't be the call of a defer or go statement"
	_ = #Wrapper        // ERROR "#Wrapper must be called"
	#Wrapper()          // ERROR "#Wrapper uses #body\(\): it is a wrap macro"

	_ = #Lazy(fmt.Sprint(must!())) // ERROR "must!: ! can't be used in a lazy macro argument"
	_ = #Lazy(func() string { return fmt.Sprint(must!()) }())

	#Undefined(1)        // ERROR "undefined macro #Undefined"
	#fmt.Sprint("x")     // ERROR "undefined macro #fmt.Sprint"
	#nothing.Do()        // ERROR "#nothing.Do: nothing is not an imported package"
	#a.b.c()             // ERROR "a macro is used as #Name\(...\) or #pkg.Name\(...\), not #a.b.c"
	#Derive(String)      // ERROR "#Derive is for types"
	_ = #body()          // ERROR "#body\(\) can only be used inside a macro declaration"
}
