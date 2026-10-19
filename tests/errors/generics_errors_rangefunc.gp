// phase: 07

// Range over a function: the shapes that are not iterators, too many iteration variables, and a
// body that does not fit. The messages are those of go/types and gc.
package main

import (
	"fmt"
	"iter"
)

func noYield(f func()) {}

func manyParams(yield func(int, int, int) bool) {}

func noBool(yield func(int)) {}

func wrongResult(yield func(int) int) {}

func hasResult(yield func(int) bool) int { return 0 }

func good(yield func(int) bool) {}

func good2(yield func(int, string) bool) {}

func main() {
	for range noYield { // ERROR "cannot range over noYield \(value of type func\(f func\(\)\)\): func must be func\(yield func\(...\) bool\): yield func does not return bool"
	}
	for range manyParams { // ERROR "cannot range over manyParams \(value of type func\(yield func\(int, int, int\) bool\)\): func must be func\(yield func\(...\) bool\): yield func has too many parameters"
	}
	for range noBool { // ERROR "cannot range over noBool \(value of type func\(yield func\(int\)\)\): func must be func\(yield func\(...\) bool\): yield func does not return bool"
	}
	for range wrongResult { // ERROR "cannot range over wrongResult \(value of type func\(yield func\(int\) int\)\): func must be func\(yield func\(...\) bool\): yield func does not return bool"
	}
	for range hasResult { // ERROR "cannot range over hasResult \(value of type func\(yield func\(int\) bool\) int\): func must be func\(yield func\(...\) bool\): unexpected results"
	}
	for range 3.5 { // ERROR "cannot range over 3.5 \(untyped float constant\)"
	}
	for a, b := range good { // ERROR "range over good \(value of type func\(yield func\(int\) bool\)\) permits only one iteration variable"
		fmt.Println(a, b)
	}
	for i := range good2 { // fine: the first only
		var s string = i // ERROR "cannot use i \(variable of type int\) as string value in variable declaration"
		_ = s
	}
	var seq iter.Seq[string]
	for x := range seq {
		var n int = x // ERROR "cannot use x \(variable of type string\) as int value in variable declaration"
		_ = n
	}
	var seq2 iter.Seq2[int, string]
	for k, v := range seq2 {
		_, _ = v, k
	}
	for k, v := range seq { // ERROR "range over seq \(variable of func type iter.Seq\[string\]\) permits only one iteration variable"
		_, _ = k, v
	}
	var nilf func(func(int) bool)
	for x := range nilf { // a nil func compiles; it panics when it runs
		_ = x
	}
	for i := range 10 {
		for x := range good {
			fmt.Println(i, x)
			break i // ERROR "invalid break label i|break label not defined: i"
		}
	}
}
