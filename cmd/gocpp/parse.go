package main

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"os"

	"github.com/huuhait/gocpp/internal/syntax/parser"
	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// parseUsage is the help text of "gocpp parse". "gocpp parse -h" prints
// it, followed by the flags.
const parseUsage = `usage: gocpp parse [-dump] file.gp ...

gocpp parse parses .gp files with gocpp's parser and prints their syntax
errors, one per line, to standard error; then it exits with status 1.
It is for debugging the parser: it only parses, so it finds no other
errors (gocpp check finds them all).

-dump prints the syntax tree of each file to standard output, as
go/ast's Print does: every node with its fields, and each position as
file:line:column. The tree is the parser's, before the loader rewrites
header and namespace uses (raylib.X stays raylib.X), with the comments
and without object resolution.

The flags are:
`

// runParse is "gocpp parse" (phase 01 README §2.12).
func runParse(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gocpp parse", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dump := flags.Bool("dump", false, "print the syntax tree of each file")
	flags.Usage = func() {
		fmt.Fprint(stderr, parseUsage)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() == 0 {
		fmt.Fprintln(stderr, "usage: gocpp parse [-dump] file.gp ...")
		return 2
	}
	exit := 0
	fset := token.NewFileSet()
	for _, name := range flags.Args() {
		src, err := os.ReadFile(name)
		if err != nil {
			fmt.Fprintln(stderr, err)
			exit = 1
			continue
		}
		f, err := parser.ParseFile(fset, name, src, parser.ParseComments|parser.SkipObjectResolution|parser.Phase10Syntax)
		if err != nil {
			scanner.PrintError(stderr, err)
			exit = 1
			continue
		}
		if *dump {
			if err := ast.Fprint(stdout, fset, f, ast.NotNilFilter); err != nil {
				fmt.Fprintln(stderr, err)
				exit = 1
			}
		}
	}
	return exit
}
