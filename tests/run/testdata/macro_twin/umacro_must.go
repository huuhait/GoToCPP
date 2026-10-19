// The hand-written Go twin of umacro_must.gp: the macros and the ! forms written out.

package main

import (
	"fmt"
	"os"
	"strconv"
)

const file = "umacro_must.gp"

func Trace(gp_file string, gp_line int, gp_func string) func() {
	fmt.Println("enter", gp_func)
	return func() { fmt.Println("exit ", gp_func) }
}

// Parse has no error result: the ! of Atoi! panics with the error.
func Parse(gp_file string, gp_line int, gp_func string, s string) int {
	n, must_e := strconv.Atoi(s)
	if must_e != nil {
		panic(must_e)
	}
	return n
}

func load(s string) (int, error) {
	if gp_defer1 := Trace(file, 24, "main.load"); gp_defer1 != nil {
		defer gp_defer1()
	}
	n, must_e := strconv.Atoi(s)
	if must_e != nil {
		return 0, must_e
	}
	return n + Parse(file, 27, "main.load", "1"), nil
}

func must_main() error {
	if gp_defer1 := Trace(file, 30, "main.main"); gp_defer1 != nil {
		defer gp_defer1()
	}
	v, must_e := load("41")
	if must_e != nil {
		return must_e
	}
	fmt.Println(v)
	func() {
		defer func() { fmt.Println("recovered:", recover() != nil) }()
		_, err := load("x")
		fmt.Println(err != nil)
		fmt.Println(Parse(file, 37, "main.main.func1", "nope"))
	}()
	fmt.Println(Parse(file, 39, "main.main", "7"))
	return nil
}

func main() {
	if err := must_main(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", "umacro_must", err)
		os.Exit(1)
	}
}
