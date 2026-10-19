// phase: 09

// PanicOnError panics with the error: here it is recovered; the message goes to the set's output first.

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

func try(args []string) {
	defer func() {
		r := recover()
		if err, ok := r.(error); ok {
			fmt.Printf("recovered: %v (help: %v)\n", err, errors.Is(err, flag.ErrHelp))
		} else {
			fmt.Println("recovered:", r)
		}
	}()
	fs := flag.NewFlagSet("tool", flag.PanicOnError)
	fs.SetOutput(os.Stdout)
	fs.Int("n", 0, "count")
	fmt.Println("parse:", fs.Parse(args))
}

func main() {
	try([]string{"-n", "1"})
	try([]string{"-zz"})
	try([]string{"-n", "x"})
	try([]string{"-h"})
	try([]string{"-n"})
}

// Output:
// parse: <nil>
// recovered: <nil>
// flag provided but not defined: -zz
// Usage of tool:
//   -n int
//     	count
// recovered: flag provided but not defined: -zz (help: false)
// invalid value "x" for flag -n: parse error
// Usage of tool:
//   -n int
//     	count
// recovered: invalid value "x" for flag -n: parse error (help: false)
// Usage of tool:
//   -n int
//     	count
// recovered: flag: help requested (help: true)
// flag needs an argument: -n
// Usage of tool:
//   -n int
//     	count
// recovered: flag needs an argument: -n (help: false)
