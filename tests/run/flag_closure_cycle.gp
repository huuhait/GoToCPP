// phase: 09
// lsan: cycle

// Closures that hold their FlagSet while the FlagSet holds them (fs.Usage set to a func literal
// that calls fs.Name and fs.PrintDefaults, a Func flag whose function uses fs, and a Usage that
// wraps the default one): a reference cycle that reference counting never frees (D1), so the
// program leaks on purpose; the output is Go's.

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	fs := flag.NewFlagSet("tool", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.Int("n", 1, "the `count`")
	var seen []string
	fs.Func("tag", "add a tag", func(s string) error {
		if s == "" {
			return fmt.Errorf("%s: empty tag", fs.Name())
		}
		seen = append(seen, s)
		return nil
	})
	old := fs.Usage
	fs.Usage = func() {
		fmt.Printf("usage of %s (%d args):\n", fs.Name(), fs.NArg())
		old()
	}
	fmt.Println(fs.Parse([]string{"-tag", "a", "-tag", "b", "x"}), seen, fs.Args())
	fmt.Println(fs.Parse([]string{"-tag="}))
	fmt.Println(fs.Parse([]string{"-nope"}))

	// A second set whose Usage is a plain closure over the set.
	gs := flag.NewFlagSet("other", flag.ContinueOnError)
	var out strings.Builder
	gs.SetOutput(&out)
	gs.Bool("v", false, "verbose")
	gs.Usage = func() {
		fmt.Fprintf(gs.Output(), "%s takes:\n", gs.Name())
		gs.PrintDefaults()
	}
	fmt.Println(gs.Parse([]string{"-h"}))
	fmt.Print(out.String())
}

// Output:
// <nil> [a b] [x]
// invalid value "" for flag -tag: tool: empty tag
// usage of tool (0 args):
// Usage of tool:
//   -n count
//     	the count (default 1)
//   -tag value
//     	add a tag
// invalid value "" for flag -tag: tool: empty tag
// flag provided but not defined: -nope
// usage of tool (0 args):
// Usage of tool:
//   -n count
//     	the count (default 1)
//   -tag value
//     	add a tag
// flag provided but not defined: -nope
// flag: help requested
// other takes:
//   -v	verbose
