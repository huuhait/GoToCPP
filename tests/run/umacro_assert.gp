// phase: 10

// A macro with a lazy parameter (step 10.9): Assert builds its message only when the assertion
// fails. #file(), #line() and #text(cond) inside the macro describe the place and the text of the
// use, not of the macro. The twin (testdata/macro_twin/umacro_assert.go) is the same program
// with the macro and its uses written out by hand.

package main

import "fmt"

// built counts how often a message was built.
var built int

func note(format string, args ...any) string {
	built++
	return fmt.Sprintf(format, args...)
}

// Assert panics with the place, the text of the condition and the message when cond is false.
macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: assertion failed: %s: %s", #file(), #line(), #text(cond), msg()))
	}
}

func try(n int) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	#Assert(n > 0, note("n is %d", n))
	#Assert(n < 100 &&
		n != 50, // fifty is odd
		note("n=%d is too big", n))
	#Assert(len(fmt.Sprint(n)) > 0, "text")
	return nil
}

func main() {
	for _, n := range []int{5, -1, 500, 50} {
		fmt.Println(n, try(n))
	}
	fmt.Println("messages built:", built)

	// A passing assertion never builds its message, in a loop either.
	built = 0
	for i := 1; i < 1000; i++ {
		#Assert(i > 0, note("never %d", i))
	}
	fmt.Println("built:", built)

	// The message sees the variables at the time the macro calls it.
	x := 1
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		#Assert(x == 2, note("x was %d", x))
	}()
	fmt.Println("built:", built)
}

// Output:
// 5 <nil>
// -1 umacro_assert.gp:33: assertion failed: n > 0: n is -1
// 500 umacro_assert.gp:34: assertion failed: n < 100 && n != 50: n=500 is too big
// 50 umacro_assert.gp:34: assertion failed: n < 100 && n != 50: n=50 is too big
// messages built: 3
// built: 0
// recovered: umacro_assert.gp:58: assertion failed: x == 2: x was 1
// built: 1
