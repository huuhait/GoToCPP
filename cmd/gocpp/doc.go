package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/huuhait/gocpp/internal/doc"
)

// docUsage is the help text of "gocpp doc". "gocpp doc -h" prints it,
// followed by the flags.
const docUsage = `usage: gocpp doc [flags] [package|[package.]symbol[.methodOrField]]

gocpp doc prints the documentation comments of the item that its arguments
name (a package, const, func, type, var, method, or struct field), followed
by a one-line summary of each first-level item under it: the package-level
declarations of a package, the methods of a type. It works like go doc, and
for the same source it prints the same text.

It takes zero, one or two arguments. With none, it shows the package in the
current folder (a command, package main, shows only its package comment,
unless -cmd is given). With one, the argument is one of:

	gocpp doc <pkg>
	gocpp doc <sym>[.<methodOrField>]
	gocpp doc [<pkg>.]<sym>[.<methodOrField>]
	gocpp doc [<pkg>.][<sym>.]<methodOrField>

and the first form that matches is the one that is shown. A package is an
import path or a folder (./geom); a path may be partial (json for
encoding/json, geom for example.com/app/geom): the standard library, the
main module and the modules that it requires are searched, in that order,
and when the symbol is not in the first match, the next match is tried.
An argument that starts with a capital letter is a symbol of the package in
the current folder. With two arguments, the first is a full package path
and the second is the symbol: gocpp doc encoding/json Marshal.

A symbol that has no capital letter matches in either case (gocpp doc
json.marshal); -c makes the match exact. Symbols with a ! (func f!() T) are
found by their name with or without the ! (f or f!), and a macro by #Name
(or Name): quote it for the shell, 'gocpp doc app.#Assert'.

A C or C++ header is an opaque package: gocpp doc '<stdio.h>' (or a header
file, ./physics.h, or a header of a module) says where the header is, which
name it gets in a .gp file, and lists the declarations that have a Doxygen
comment (/// //! /** */ /*! */) in front of them; with a name, gocpp doc
'<vector>' std.vector, it asks the C++ compiler what the name is. gocpp doc
std explains the C++ namespace std.

The source is the .gp files that the build constraints of the target (GOOS
and GOARCH) keep; the standard library is documented from its own .gp
files, embedded in gocpp.

The flags are:
`

// runDoc is "gocpp doc" (phase 13 README §2.4): the flags, the folder and
// the target, then internal/doc.
func runDoc(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var opts doc.Options
	var chdir string
	flags := flag.NewFlagSet("gocpp doc", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&chdir, "C", "", "change to `dir` before running command")
	flags.BoolVar(&opts.Unexported, "u", false, "show unexported symbols as well as exported")
	flags.BoolVar(&opts.MatchCase, "c", false, "symbol matching honors case (paths not affected)")
	flags.BoolVar(&opts.All, "all", false, "show all documentation for package")
	flags.BoolVar(&opts.Cmd, "cmd", false, "show symbols with package docs even if package is a command")
	flags.BoolVar(&opts.Src, "src", false, "show source code for symbol")
	flags.BoolVar(&opts.Short, "short", false, "one-line representation for each symbol")
	flags.BoolVar(&opts.Examples, "examples", false, "list the examples of the _test.gp files")
	flags.Usage = func() {
		fmt.Fprint(stderr, docUsage)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	goos, goarch, err := target()
	if err != nil {
		fmt.Fprintf(stderr, "gocpp: %v\n", err)
		return 2
	}
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "gocpp doc: %v\n", err)
		return 1
	}
	if chdir != "" {
		if err := os.Chdir(chdir); err != nil {
			fmt.Fprintf(stderr, "gocpp doc: %v\n", err)
			return 1
		}
		if dir, err = os.Getwd(); err != nil {
			fmt.Fprintf(stderr, "gocpp doc: %v\n", err)
			return 1
		}
	}
	ctx, stopped, stop := signalContext()
	defer stop()
	env := &doc.Env{Dir: dir, GOOS: goos, GOARCH: goarch, StdlibDir: stdlibDir, Context: ctx, Warn: os.Stderr}
	err = doc.Run(stdout, env, opts, flags.Args())
	if err == nil {
		return 0
	}
	if sig := stopped(); sig != nil {
		return stopStatus(sig)
	}
	var usage *doc.UsageError
	if errors.As(err, &usage) {
		fmt.Fprintf(stderr, "gocpp doc: %v\n", err)
		fmt.Fprint(stderr, "usage: gocpp doc [flags] [package|[package.]symbol[.methodOrField]]\nFor more information run\n\tgocpp doc -h\n")
		return 2
	}
	fmt.Fprintf(stderr, "gocpp doc: %v\n", err)
	return 1
}
