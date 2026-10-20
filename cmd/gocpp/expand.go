package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/expand"
	"github.com/huuhait/gocpp/internal/load"
)

// expandUsage is the help text of "gocpp expand". "gocpp expand -h" prints it, followed by the flags.
const expandUsage = `usage: gocpp expand [-full] [-tags list] [file.gp... | package]

gocpp expand prints the plain Go that the compiler works with: the .gp
files of the package with the macros, #Derive lines and ! forms written
out, so that you can see what they do. It checks the package like gocpp
check (the same errors, with exit status 1) and prints to standard
output.

What it shows:

	#Name(args) uses      the call of the function that stands for the macro
	                      (its hidden arguments are the file, line and
	                      function of the use and the text of the arguments),
	                      a lazy argument as a func literal, an on-top line as
	                      the first statement, or the code of the function
	                      moved into a func literal for the wrap style
	macro declarations    the generic function that the compiler makes of them
	#Derive(...) lines    the methods, with their bodies
	f! functions, F!(x)   the explicit "if err != nil" code: must_e1, must_t1
	                      are the names of the temporaries; main! becomes
	                      must_main with a main that calls gp_must.Exit, which
	                      prints "<program>: <error>" and exits with 1

Every declaration that has none of these is printed as you wrote it,
byte for byte, comments included; a declaration that has one is printed
by gofmt's printer with its comments, and has the names above. Names that
start with gp_ and must_ belong to the compiler. A declaration that uses
C or C++ values, and has a macro or a ! in it, shows the compiler's own
forms for them (C.std.vector, gp_cgoh.As[int](x)).

The bytes of a file that #embed puts into a declaration that is printed
from the tree are shortened to their first 40 and a comment with the
length; -full prints them whole. A //go:embed variable is printed as you
wrote it.

The output is your file with the expanded declarations replaced: the rest
(comments, blank lines, spacing, line ends) is yours, byte for byte. With
more than one file, each file starts with a line "//// name" (slashes in
the path, also on Windows). The _test.gp files are not loaded, so they are
not printed. The output is Go-like text for reading, not a program that can
be compiled.
` + interopHelp + `The flags are:
`

// runExpand is "gocpp expand" (Phase 10 step 10.11).
func runExpand(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	c := &checkCommand{stdout: stdout, stderr: stderr}
	flags := flag.NewFlagSet("gocpp expand", flag.ContinueOnError)
	c.diagOpts.register(flags)
	flags.Var(tagsFlag{&c.tags}, "tags", "a comma-separated `list` of build tags that are satisfied, like go build -tags")
	var cross crossFlags
	cross.register(flags)
	c.interop.register(flags)
	full := false
	flags.BoolVar(&full, "full", false, "print the bytes of embedded files in full, not their first 40")
	if code, done := parseFlags("expand", expandUsage, flags, args, stderr); done {
		return code
	}
	var err error
	if c.goos, c.goarch, err = cross.resolve(); err != nil {
		fmt.Fprintf(stderr, "gocpp: %v\n", err)
		return 2
	}
	if c.dir, err = os.Getwd(); err != nil {
		fmt.Fprintf(stderr, "gocpp expand: %v\n", err)
		return exitErrors
	}
	var stop func()
	c.ctx, c.stopped, stop = signalContext()
	defer stop()
	// The check rewrites the files, so the nodes that the loader has made by now are the nodes of
	// the user's text for printExpanded (together with the text of the file, which tells what is a use).
	known := make(map[ast.Node]bool)
	c.afterLoad = func(prog *load.Program) {
		for _, root := range prog.Roots {
			for _, f := range root.Files {
				ast.Inspect(f.AST, func(n ast.Node) bool {
					if n != nil {
						known[n] = true
					}
					return true
				})
			}
		}
	}
	prog, r, code := c.check(flags.Args())
	if r == nil || code != 0 {
		return code
	}
	type unit struct {
		pkg  *check.Package
		file *load.File
	}
	var files []unit
	for _, root := range prog.Roots {
		if p := r.Package(root); p != nil && !c.dropped[root] {
			for _, f := range p.Load.Files {
				files = append(files, unit{p, f})
			}
		}
	}
	var b bytes.Buffer
	for _, u := range files {
		if len(files) > 1 {
			fmt.Fprintf(&b, "//// %s\n", relPath(c.dir, u.file.Path))
		}
		if expandTestHook != nil {
			expandTestHook(u.file)
		}
		if err := printExpanded(&b, prog.Fset, u.pkg, u.file, known, full); err != nil {
			fmt.Fprintf(stderr, "gocpp expand: %s: %v\n", relPath(c.dir, u.file.Path), err)
			return exitInternal
		}
	}
	if _, err := stdout.Write(b.Bytes()); err != nil {
		fmt.Fprintf(stderr, "gocpp expand: %v\n", err)
		return exitErrors
	}
	return 0
}

// relPath is path relative to dir, with slashes, when it is inside dir.
func relPath(dir, path string) string {
	if rel, err := filepath.Rel(dir, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

// expandTestHook, if not nil, is called by gocpp expand with each file before it is printed (tests).
var expandTestHook func(f *load.File)

// printExpanded writes the file f as Go source (package expand does the work, also for the language server).
func printExpanded(w *bytes.Buffer, fset *token.FileSet, pkg *check.Package, f *load.File, known map[ast.Node]bool, full bool) error {
	out, _, err := expand.File(fset, pkg, f, known, full, readGP)
	if err != nil {
		return err
	}
	w.Write(out)
	return nil
}

// An edit replaces src[start:end] by text (an insertion has start == end).
type edit struct {
	start, end int
	text       string
	insert     bool // text is declarations to put in, not a replacement
}

// applyEdits returns src with the edits made (expand.ApplyEdits; the tests of the command use it).
func applyEdits(src []byte, edits []edit) ([]byte, error) {
	es := make([]expand.Edit, len(edits))
	for i, e := range edits {
		es[i] = expand.Edit{Start: e.start, End: e.end, Text: e.text, Insert: e.insert}
	}
	return expand.ApplyEdits(src, es)
}
