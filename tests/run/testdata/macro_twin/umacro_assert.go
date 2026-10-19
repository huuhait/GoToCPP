// The hand-written Go twin of umacro_assert.gp: Assert is a function with the place of the use
// as arguments, and its lazy message is a func literal.

package main

import "fmt"

var built int

func note(format string, args ...any) string {
	built++
	return fmt.Sprintf(format, args...)
}

// Assert is the macro: gp_file, gp_line and gp_text_cond are what the compiler passes.
func Assert(gp_file string, gp_line int, gp_text_cond string, cond bool, msg func() string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: assertion failed: %s: %s", gp_file, gp_line, gp_text_cond, msg()))
	}
}

func try(n int) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	Assert("umacro_assert.gp", 33, "n > 0", n > 0, func() string { return note("n is %d", n) })
	Assert("umacro_assert.gp", 34, "n < 100 && n != 50", n < 100 &&
		n != 50, // fifty is odd
		func() string { return note("n=%d is too big", n) })
	Assert("umacro_assert.gp", 37, "len(fmt.Sprint(n)) > 0", len(fmt.Sprint(n)) > 0, func() string { return "text" })
	return nil
}

func main() {
	for _, n := range []int{5, -1, 500, 50} {
		fmt.Println(n, try(n))
	}
	fmt.Println("messages built:", built)

	built = 0
	for i := 1; i < 1000; i++ {
		Assert("umacro_assert.gp", 50, "i > 0", i > 0, func() string { return note("never %d", i) })
	}
	fmt.Println("built:", built)

	x := 1
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		Assert("umacro_assert.gp", 58, "x == 2", x == 2, func() string { return note("x was %d", x) })
	}()
	fmt.Println("built:", built)
}
