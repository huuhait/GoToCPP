package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/huuhait/gocpp/internal/typedinterop"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
)

// runTypedProbe is the hidden developer command "gocpp __typed-probe" (steps
// 12.1 and 12.2 of typed interop): it checks the packages of the patterns,
// collects the C/C++ references of each root package, probes them with clang
// and prints the reference table and the result table as JSON to standard
// output (-refs: only the references, nothing is probed). It exists for
// developing the typed pipeline and for its tests; it is not in the help.
func runTypedProbe(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	c := &checkCommand{stdout: io.Discard, stderr: stderr, kinds: true}
	flags := flag.NewFlagSet("gocpp __typed-probe", flag.ContinueOnError)
	flags.SetOutput(stderr)
	refsOnly := flags.Bool("refs", false, "only collect the references (no clang run)")
	decls := flags.Bool("decls", false, "also extract the declarations the results mention and map every result to the foreign view (steps 12.3 and 12.4)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	var err error
	if c.goos, c.goarch, err = target(); err != nil {
		fmt.Fprintf(stderr, "gocpp: %v\n", err)
		return 2
	}
	if c.dir, err = getwd(); err != nil {
		fmt.Fprintf(stderr, "gocpp __typed-probe: %v\n", err)
		return 1
	}
	var stop func()
	c.ctx, c.stopped, stop = signalContext()
	defer stop()
	prog, r, code := c.check(flags.Args())
	if r == nil || code != 0 {
		return code
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	for _, root := range prog.Roots {
		p := r.Package(root)
		if p == nil || !p.Checked() {
			continue
		}
		opts := typedinterop.Options{Decls: *decls, Target: ftype.LP64}
		if c.goos == "windows" {
			opts.Target = ftype.LLP64
		}
		if *refsOnly {
			rep := typedinterop.Collect(p, prog.Fset)
			if err := enc.Encode(rep); err != nil {
				fmt.Fprintf(stderr, "gocpp __typed-probe: %v\n", err)
				return 1
			}
			continue
		}
		rep, err := typedinterop.Analyze(c.ctx, p, prog.Fset, opts)
		if err != nil && rep == nil {
			if errors.Is(err, typedinterop.ErrNoClang) {
				fmt.Fprintf(stderr, "gocpp __typed-probe: %v\n", err)
				return 3
			}
			fmt.Fprintf(stderr, "gocpp __typed-probe: %v\n", err)
			return 1
		}
		if err != nil {
			fmt.Fprintf(stderr, "gocpp __typed-probe: %v\n", err)
		}
		if err := enc.Encode(rep); err != nil {
			fmt.Fprintf(stderr, "gocpp __typed-probe: %v\n", err)
			return 1
		}
	}
	return 0
}
