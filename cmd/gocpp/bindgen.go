package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/typedinterop/bindgen"
)

// bindgenUsage is the help text of "gocpp bindgen".
const bindgenUsage = `usage: gocpp bindgen [flags] header

gocpp bindgen writes a readable gocpp view of a C or C++ header: a .gp file
with the types, functions, variables, enums, typedefs and namespaces of the
header, and the macros that are constants (a macro whose value is a struct is
a var). The header is <name.h> for a system header (found in the include path)
or the path of a file. The view is for reading and for editors; it uses clang
to know the declarations, so clang++ must be installed (GOCPP_CLANG names it).
A C header is read as C++, the way gocpp includes it. The view passes gocpp
check but its functions have no bodies: write it where you read it, not into a
package folder that you build. The types and values of the macros come from the
probe cache of gocpp (GOCPP_NOPROBECACHE=1 turns it off).

The flags are:
`

// A repeatFlag is a flag that can be given several times.
type repeatFlag []string

func (l *repeatFlag) String() string     { return strings.Join(*l, ",") }
func (l *repeatFlag) Set(s string) error { *l = append(*l, s); return nil }

// runBindgen is "gocpp bindgen" (Phase 12 §2.7, step 12.10). Exit status: 0
// on success, 1 when the header cannot be read or translated, 2 for a
// usage error, 3 when there is no clang++.
func runBindgen(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gocpp bindgen", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var cfg bindgen.Config
	var includes, defines, pkgs, include, exclude repeatFlag
	out := flags.String("o", "-", "write the view to this file ('-' is standard output)")
	flags.StringVar(&cfg.Package, "pkg", "", "the package name (default: the name of the header file, as an import would give it)")
	flags.Var(&includes, "I", "add a folder to the include path (repeatable)")
	flags.Var(&defines, "D", "define a macro: NAME or NAME=value (repeatable)")
	flags.StringVar(&cfg.Std, "std", "", "the language standard (default c++20)")
	flags.Var(&pkgs, "pkg-config", "add the compiler flags of a pkg-config package (repeatable)")
	flags.Var(&include, "include", "write only the declarations and macros whose C++ name matches this regular expression (repeatable); a type that a written declaration uses is written whatever -include and -exclude say")
	flags.Var(&exclude, "exclude", "leave out the declarations and macros whose C++ name matches this regular expression (repeatable); a type that a written declaration uses is written anyway")
	verbose := flags.Bool("v", false, "print what was written and where the time went to standard error")
	scope := flags.String("scope", "auto", "which included files are part of the view: header (the file alone), dir (and the headers of its folder that it includes), tree (and the folders below), or auto (dir, but a header that lies in a folder of the include path itself, such as /usr/include, is taken alone)")
	flags.Usage = func() {
		fmt.Fprint(stderr, bindgenUsage)
		flags.PrintDefaults()
	}
	args = splitCompilerFlags(args)
	// flags may come before and after the header, as in "gocpp bindgen <x.h> -I dir"
	var headers []string
	for {
		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if flags.NArg() == 0 {
			break
		}
		headers = append(headers, flags.Arg(0))
		args = flags.Args()[1:]
	}
	if len(headers) != 1 {
		fmt.Fprintln(stderr, "gocpp bindgen: expected one header")
		flags.Usage()
		return 2
	}
	cfg.Header = headers[0]
	cfg.Includes, cfg.Defines, cfg.Include, cfg.Exclude = includes, defines, include, exclude
	cfg.Scope = bindgen.Scope(*scope)
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "gocpp bindgen: %v\n", err)
		return 1
	}
	cfg.Dir = wd

	ctx, stopped, stop := signalContext()
	defer stop()
	if len(pkgs) > 0 {
		cflags, _, err := build.PkgConfig(ctx, wd, pkgs)
		if err != nil {
			fmt.Fprintf(stderr, "gocpp bindgen: %v\n", err)
			return 1
		}
		cfg.Flags = cflags
	}
	res, err := bindgen.Generate(ctx, cfg)
	if err != nil {
		if ctx.Err() != nil {
			fmt.Fprintf(stderr, "gocpp bindgen: interrupted\n")
			if sig := stopped(); sig != nil {
				return stopStatus(sig)
			}
			return exitErrors
		}
		fmt.Fprintf(stderr, "gocpp bindgen: %v\n", err)
		if errors.Is(err, bindgen.ErrNoClang) {
			return 3
		}
		return 1
	}
	if *verbose {
		st := res.Stats
		fmt.Fprintf(stderr, "gocpp bindgen: package %s: %d types, %d functions, %d variables, %d constants, %d referenced types, %d macros not translated; %d files in view, %d KB of AST; preprocess %v, AST %v, probe %v, total %v\n",
			res.Package, st.Types, st.Funcs, st.Vars, st.Consts, st.Referenced, st.Skipped, st.Files, st.JSONBytes>>10,
			st.Preprocess.Round(time.Millisecond), st.AST.Round(time.Millisecond), st.Probe.Round(time.Millisecond), st.Total.Round(time.Millisecond))
	}
	for _, w := range res.Warnings {
		fmt.Fprintf(stderr, "gocpp bindgen: warning: %s\n", w)
	}
	if *out == "-" || *out == "" {
		if _, err := stdout.Write(res.Source); err != nil {
			fmt.Fprintf(stderr, "gocpp bindgen: %v\n", err)
			return 1
		}
		return 0
	}
	if err := os.WriteFile(*out, res.Source, 0o666); err != nil {
		fmt.Fprintf(stderr, "gocpp bindgen: %v\n", err)
		return 1
	}
	return 0
}

// splitCompilerFlags turns -Ddir and -DNAME=value into the two arguments that
// package flag reads (-D NAME=value), so that the flags of the C compiler can
// be written as usual.
func splitCompilerFlags(args []string) []string {
	var out []string
	for _, a := range args {
		if len(a) > 2 && (strings.HasPrefix(a, "-D") || strings.HasPrefix(a, "-I")) {
			out = append(out, a[:2], a[2:])
			continue
		}
		out = append(out, a)
	}
	return out
}
