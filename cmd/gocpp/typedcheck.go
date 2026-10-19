package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/typedinterop"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/icheck"
)

// runTypedCheck is the hidden developer command "gocpp __typed-check" (step
// 12.5 of typed interop): it checks the packages of the patterns, then runs
// the interop checker (internal/typedinterop/icheck) on each package of the
// program that is not a standard package, and prints its errors and warnings
// like gocpp check does. The exit status is 1 when there is an error, 3 when
// there is no clang++ (typed mode would stay opaque). It is the integration point
// of the checker until the mode switch of step 12.11: nothing else in gocpp runs
// it, and opaque mode is unchanged.
//
//	-e        report all errors, not only the first 10 of each package
//	-v        also list what the checker left to the C++ compiler, and why
//	-build X  the compiler that builds the program: g++ makes the errors warnings
//	          (the default is the compiler that gocpp build would use)
func runTypedCheck(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	c := &checkCommand{stdout: io.Discard, stderr: stderr, kinds: true}
	flags := flag.NewFlagSet("gocpp __typed-check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.BoolVar(&c.all, "e", false, "report all errors, not only the first 10 of each package")
	verbose := flags.Bool("v", false, "also list what is left to the C++ compiler, and why")
	buildCXX := flags.String("build", "", "the compiler that builds the program: g++ or clang++ (default: the one gocpp build uses)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	fam := build.Family(0)
	switch *buildCXX {
	case "":
	case "g++", "gcc":
		fam = build.GCC
	case "clang++", "clang":
		fam = build.Clang
	default:
		fmt.Fprintf(stderr, "gocpp __typed-check: -build must be g++ or clang++\n")
		return 2
	}
	var err error
	if c.goos, c.goarch, err = target(); err != nil {
		fmt.Fprintf(stderr, "gocpp: %v\n", err)
		return 2
	}
	if c.dir, err = os.Getwd(); err != nil {
		fmt.Fprintf(stderr, "gocpp __typed-check: %v\n", err)
		return 1
	}
	var stop func()
	c.ctx, c.stopped, stop = signalContext()
	defer stop()
	prog, r, code := c.check(flags.Args())
	if r == nil || code != 0 {
		return code
	}
	failed := false
	for _, lp := range packageList(prog) {
		p := r.Package(lp)
		if p == nil || !p.Checked() || lp.Std {
			continue
		}
		opts := icheck.Options{Build: fam, Cross: c.goos != runtime.GOOS || c.goarch != runtime.GOARCH}
		opts.Analyze.Target = ftype.LP64
		if c.goos == "windows" {
			opts.Analyze.Target = ftype.LLP64
		}
		res, err := icheck.Check(c.ctx, p, prog.Fset, opts)
		if c.ctx.Err() != nil {
			// Interrupted (also when the probe's own error was turned into a skipped package).
			fmt.Fprintf(stderr, "gocpp __typed-check: interrupted\n")
			if sig := c.stopped(); sig != nil {
				return stopStatus(sig)
			}
			return exitErrors
		}
		if err != nil {
			if errors.Is(err, typedinterop.ErrNoClang) {
				fmt.Fprintf(stderr, "gocpp __typed-check: %v\n", err)
				return 3
			}
			fmt.Fprintf(stderr, "gocpp __typed-check: %v\n", err)
			return 1
		}
		if *verbose {
			if res.Skipped != "" {
				fmt.Fprintf(stderr, "# %s: skipped: %s\n", pkgHeader(lp), res.Skipped)
			}
			st := res.Stats
			fmt.Fprintf(stderr, "# stats %s: %d references, %d probed (%d compiled, %d failed), %d uses, %d silent, %d diagnostics\n", pkgHeader(lp), st.Refs, st.Probed, st.Compiled, st.Failed, st.Sites, len(res.Silent), len(res.Diags))
			for _, s := range res.Silent {
				fmt.Fprintf(stderr, "# silent %s:%d:%d: %s\n", s.Pos.Filename, s.Pos.Line, s.Pos.Column, s.Why)
			}
		}
		if len(res.Diags) == 0 {
			continue
		}
		list := new(diag.List)
		for _, d := range res.Diags {
			list.Add(d)
		}
		fmt.Fprintf(stderr, "# %s\n", pkgHeader(lp))
		c.print(stderr, list, c.all)
		failed = failed || res.Errors() > 0
	}
	if failed {
		return 1
	}
	return 0
}
