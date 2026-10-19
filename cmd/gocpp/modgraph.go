package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/huuhait/gocpp/internal/modfetch"
	"github.com/huuhait/gocpp/internal/modgraph"
)

// runModGraph is "gocpp mod graph" and "gocpp mod why".
func runModGraph(cmd string, args []string, stdout, stderr io.Writer) int {
	var whyModule bool
	usage := "usage: gocpp mod graph\n\nPrints the requirement graph of the main module: one line for each requirement,\n\"module requirement\", the main module without a version, in the order of\n\"go mod graph\". Nothing is written: a module file that gocpp.sum does not\nlist is checked against nothing.\n"
	if cmd == "why" {
		usage = "usage: gocpp mod why [-m] module ...\n\nPrints, for each module, the shortest chain of requirements that puts it in\nthe build list, starting with the main module. -m is accepted for\ncompatibility with go mod why: gocpp has no package-level answer.\n"
	}
	flags, code, ok := modFlags("gocpp mod "+cmd, args, stderr, usage, func(fs *flag.FlagSet) {
		if cmd == "why" {
			fs.BoolVar(&whyModule, "m", false, "treat the arguments as modules (always)")
		}
	})
	if !ok {
		return code
	}
	if cmd == "graph" && flags.NArg() > 0 {
		fmt.Fprintf(stderr, "gocpp mod graph: no arguments expected\n")
		return 2
	}
	if cmd == "why" && flags.NArg() == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	m, code := openMainModule("gocpp mod "+cmd, stderr)
	if m == nil {
		return code
	}
	f, ok := m.fetcher("gocpp mod " + cmd)
	if !ok {
		return 1
	}
	ctx, stop := interruptContext()
	defer stop()
	// Module files that gocpp.sum does not list are accepted but not recorded:
	// these commands write nothing.
	src := &modgraph.FetcherSource{F: f, MainDir: m.dir, Pending: &modfetch.PendingSums{}}
	reqs := modgraph.New(ctx, &modgraph.Config{MainDir: m.dir, File: m.file, Source: src})
	_, g, err := reqs.BuildList()
	if err != nil {
		fmt.Fprintf(stderr, "gocpp mod %s: %v\n", cmd, err)
		return 1
	}
	if cmd == "graph" {
		for _, e := range modgraph.Edges(g) {
			fmt.Fprintf(stdout, "%s %s\n", e.From, e.To)
		}
		return 0
	}
	main := reqs.Main()
	for i, path := range flags.Args() {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		fmt.Fprintf(stdout, "# %s\n", path)
		chain := modgraph.Why(g, main, path)
		if chain == nil {
			fmt.Fprintf(stdout, "(main module does not need module %s)\n", path)
			continue
		}
		for _, c := range chain {
			fmt.Fprintln(stdout, c.Path)
		}
	}
	return 0
}
