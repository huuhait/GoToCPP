// phase: 05

// What go/types says about macros (step 10.9), in the words of the macro: a macro is a function with
// hidden parameters, and the messages name it as #Name.

package main

import "fmt"

macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(fmt.Sprint(#text(cond), msg()))
	}
}

macro Dbg[T any](v T) T { return v }

macro Both[A, B any](a A, b B) (A, B) { return a, b }

macro Show(n int) int { return n }

macro Min(a, b int) int { return min(a, b) }

func shadow(Show int) int {
	return #Show(1) // ERROR "the macro Show is hidden here by a variable \(or a parameter\) with the same name: rename it"
}

type Dup struct{}

func Dup2() {}

macro Dup2() {} // ERROR "Dup2 redeclared in this block \(a macro and a func, or two macros, can't have the same name\)"

func use() {
	Assert(true, "x") // ERROR "Assert is a macro: write #Assert\(...\)"
	f := Dbg          // ERROR "Dbg is a macro: write #Dbg\(...\)"
	_ = f
	_ = Dbg[int](1)     // ERROR "Dbg\[int\] is a macro: write #Dbg\[int\]\(...\)"
	#Assert(1, "x")     // ERROR "(?s)cannot use 1 \(untyped int constant\) as bool value in argument to #Assert.*macro Assert is declared at"
	#Assert(true, 5)    // ERROR "cannot use 5 \(untyped int constant\) as string value in return statement"
	_ = #Dbg[string](1) // ERROR "cannot use 1 \(untyped int constant\) as string value in argument to #Dbg\[string\]"
	_ = #Dbg(nil)       // ERROR "in call to #Dbg, cannot infer T"
	_, _ = #Both[int](1, "x")
	var str string = #Min(1, 2) // ERROR "cannot use #Min\(...\) \(value of type int\) as string value in variable declaration"
	_ = str
	_ = #Assert(true, "x") // ERROR "#Assert\(...\) has no value: the macro has no result"
}
