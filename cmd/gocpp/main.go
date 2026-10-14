// Gocpp turns Go-syntax .gp files into readable C++20 and builds them with
// clang++ or g++.
//
// Usage:
//
//	gocpp <command> [arguments]
//
// Run "gocpp help" for the list of commands. A command that is not built yet
// says which phase of the plan builds it.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/huuhait/gocpp/internal/version"
)

// A command is one "gocpp <name>" command.
type command struct {
	name    string
	summary string // one line for "gocpp help"
	phase   int    // the phase of the plan that builds the command

	// run runs the command with the arguments after its name and returns the
	// exit code. It is nil while the command is not built yet. stdin is the
	// standard input, which only some commands read (gocpp fmt with no
	// paths).
	run func(args []string, stdin io.Reader, stdout, stderr io.Writer) int
}

// commands lists every planned command, in the order "gocpp help" shows
// them. A phase that builds a command sets its run field.
var commands = []command{
	{name: "build", phase: 2, summary: "compile packages and their C/C++ files"},
	{name: "run", phase: 2, summary: "compile and run a program"},
	{name: "test", phase: 9, summary: "run the tests in _test.gp files"},
	{name: "check", phase: 1, summary: "parse and type-check packages, print errors"},
	{name: "emit", phase: 2, summary: "print the C++ generated for a .gp file"},
	{name: "expand", phase: 10, summary: "print code after macro, #Derive and ! rewriting"},
	{name: "fmt", phase: 1, summary: "format .gp files like gofmt", run: runFmt},
	{name: "lsp", phase: 11, summary: "run the language server for editors"},
	{name: "vet", phase: 13, summary: "report likely mistakes in packages"},
	{name: "doc", phase: 13, summary: "show documentation for a package or symbol"},
	{name: "debug", phase: 13, summary: "build a program and start gdb or lldb on it"},
	{name: "explain", phase: 13, summary: "show the C++ generated for one .gp line"},
	{name: "env", phase: 6, summary: "print the C++ compiler, cache dir and flags"},
	{name: "list", phase: 6, summary: "list packages, like go list"},
	{name: "clean", phase: 6, summary: "remove object files and cached files"},
	{name: "get", phase: 15, summary: "add dependencies to gocpp.mod and download them"},
	{name: "bindgen", phase: 12, summary: "write a readable gocpp view of a C/C++ header"},
	{name: "export", phase: 6, summary: "write build files for other tools (cmake)"},
	{name: "version", phase: 0, summary: "print the gocpp version", run: runVersion},
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run runs gocpp with the arguments after the program name and returns the
// exit code. A usage error and a command that is not built yet exit with 2;
// otherwise the command decides.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr) // like "go" with no arguments
		return 2
	}
	name, args := args[0], args[1:]
	switch name {
	case "help", "-h", "-help", "--help":
		if len(args) > 0 {
			fmt.Fprintln(stderr, "usage: gocpp help")
			return 2
		}
		usage(stdout)
		return 0
	}
	cmd := lookup(name)
	if cmd == nil {
		fmt.Fprintf(stderr, "gocpp %s: unknown command\nRun 'gocpp help' for usage.\n", name)
		return 2
	}
	if cmd.run == nil {
		fmt.Fprintf(stderr, "gocpp %s: not available yet (Phase %02d)\n", name, cmd.phase)
		return 2
	}
	return cmd.run(args, stdin, stdout, stderr)
}

// lookup returns the command with the given name, or nil.
func lookup(name string) *command {
	for i := range commands {
		if commands[i].name == name {
			return &commands[i]
		}
	}
	return nil
}

// usage writes the help text: what gocpp is and one line per command.
func usage(w io.Writer) {
	fmt.Fprint(w, `gocpp turns Go-syntax .gp files into readable C++20 and builds them
with clang++ or g++.

Usage:

	gocpp <command> [arguments]

The commands are:

`)
	// Line up the "(Phase NN)" marks of the commands that are not built yet.
	width := 0
	for _, c := range commands {
		if c.run == nil {
			width = max(width, len(c.summary))
		}
	}
	for _, c := range commands {
		if c.run == nil {
			fmt.Fprintf(w, "\t%-8s %-*s  (Phase %02d)\n", c.name, width, c.summary, c.phase)
		} else {
			fmt.Fprintf(w, "\t%-8s %s\n", c.name, c.summary)
		}
	}
	fmt.Fprint(w, `
A command marked (Phase NN) is planned but not built yet: it comes in
that phase of the plan. Run "gocpp <command> -h" to see how a command
that is built works, for example "gocpp fmt -h".
`)
}

// runVersion is "gocpp version".
func runVersion(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: gocpp version")
		return 2
	}
	fmt.Fprintln(stdout, version.String())
	return 0
}
