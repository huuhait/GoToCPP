package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/token"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/vet"
)

// vetUsage is the help text of "gocpp vet".
const vetUsage = `usage: gocpp vet [-json] [-v] [-list] [-<analyzer>[=false]] [packages]

gocpp vet reports likely mistakes in the packages: the checks of go vet
that make sense for gocpp programs (printf, copylocks, unusedresult, nilness,
...) and checks of gocpp's own (cycle: reference cycles that reference
counting never frees; reserved: names that C++ reserves; panicbang: a
placeholder for Phase 10). "gocpp vet -list" shows them all.

Each finding is one line, "file:line:col: message", at the place in the
.gp source, under a line "# <import path>" for its package, on standard
error. A package is a folder, a list of .gp files, an import path or a
pattern with "...", as for gocpp check. Without packages, it vets the
package in the current folder. The _test.gp files are vetted too, like go vet
does.

Exit status: 0 when nothing is found, 1 when there is a finding, 2 when
the packages can't be vetted (a load or type error: gocpp vet prints the
errors like gocpp check), or when an analyzer crashed (a gocpp bug). A
feature that the front-end does not have yet, like macros and must
functions (Phase 10), stops vet like a type error; a feature that it
can type but not build yet (a phase gate of gocpp check) does not.

Like go vet, -<analyzer> (or -<analyzer>=true) runs only the analyzers
named so, and -<analyzer>=false leaves one out. -json prints the findings
as JSON on standard output, by package and analyzer ("posn", "end" when
the analyzer gives one, "message"), and exits with 0 when it could vet.

Code that uses C/C++ names has no complete types for go/types. The
analyzers that can't cope with that (nilness, lostcancel, ...) skip such a
package; the others run, and the analyzers of gocpp check what they touch.
-v lists what was skipped and why. An analyzer that crashes or fails is
reported on standard error ("vet: analyzer X failed on package Y"): that
is a gocpp bug.

The flags are:
`

// A triState is the flag of one analyzer: not set, true or false.
type triState struct {
	set, val bool
}

func (t *triState) String() string   { return "" }
func (t *triState) IsBoolFlag() bool { return true }
func (t *triState) Set(s string) error {
	switch s {
	case "true", "1", "t", "T", "TRUE", "True":
		t.set, t.val = true, true
	case "false", "0", "f", "F", "FALSE", "False":
		t.set, t.val = true, false
	default:
		return errors.New("parse error")
	}
	return nil
}

// runVet is "gocpp vet" (phase 13 README §2.3): it loads and type-checks the
// packages like gocpp check, then runs the analyzers of internal/vet.
func runVet(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gocpp vet", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var jsonOut, verbose, list bool
	flags.BoolVar(&jsonOut, "json", false, "print the findings as JSON on standard output")
	flags.BoolVar(&verbose, "v", false, "also list what was skipped or left out on packages with C/C++ names")
	flags.BoolVar(&list, "list", false, "list the analyzers and exit")
	states := make(map[string]*triState)
	for _, a := range vet.Analyzers {
		st := new(triState)
		states[a.Name] = st
		flags.Var(st, a.Name, firstLine(a.Doc))
	}
	flags.Usage = func() {
		fmt.Fprint(stderr, vetUsage)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if list {
		for _, a := range vet.Analyzers {
			kind := "go vet"
			switch {
			case a.Placeholder:
				kind = "gocpp, placeholder"
			case a.Gocpp:
				kind = "gocpp"
			}
			fmt.Fprintf(stdout, "%-14s %-19s %s\n", a.Name, kind, firstLine(a.Doc))
		}
		return 0
	}
	on, off := make(map[string]bool), make(map[string]bool)
	for name, st := range states {
		switch {
		case st.set && st.val:
			on[name] = true
		case st.set:
			off[name] = true
		}
	}
	analyzers := vet.Select(on, off)

	var errBuf bytes.Buffer
	c := &checkCommand{stdout: stdout, stderr: &errBuf, tests: true}
	var err error
	if c.goos, c.goarch, err = target(); err != nil {
		fmt.Fprintf(stderr, "gocpp: %v\n", err)
		return 2
	}
	if c.dir, err = os.Getwd(); err != nil {
		fmt.Fprintf(stderr, "gocpp vet: %v\n", err)
		return 2
	}
	var stop func()
	c.ctx, c.stopped, stop = signalContext()
	defer stop()

	prog, r, code := c.check(flags.Args())
	if c.ctx.Err() != nil && c.stopped() != nil {
		return code
	}
	// Only errors that stop the types make vet fail. A phase gate ("not
	// supported yet (Phase 03)") is an error of check, because the build
	// can't make C++ of the feature yet, but the packages have types and
	// can be vetted. The warnings are shown.
	if r == nil || !c.vettable(prog, r) {
		stderr.Write(errBuf.Bytes())
		return exitInternal
	}
	if code == 0 {
		stderr.Write(errBuf.Bytes())
	}

	var roots []*load.Package
	for _, p := range prog.Roots {
		if !c.dropped[p] {
			roots = append(roots, p)
		}
	}
	rep, err := vet.Run(prog, r, vet.Config{Roots: roots, Analyzers: analyzers, GOARCH: c.goarch})
	if err != nil {
		fmt.Fprintf(stderr, "gocpp vet: %v\n", err)
		return exitInternal
	}
	for _, n := range rep.Notes {
		switch {
		case n.Skipped && verbose:
			fmt.Fprintf(stderr, "vet: %s: analyzer %s skipped: %s%s\n", n.Package, n.Analyzer, n.Text, c.vetWhere(n.Where))
		case !n.Skipped:
			text := n.Text
			if !verbose {
				text, _, _ = strings.Cut(text, "\n")
			}
			fmt.Fprintf(stderr, "vet: analyzer %s failed on package %s: %s (this is a gocpp bug, please report it)\n", n.Analyzer, n.Package, text)
		}
	}
	code = 0
	if jsonOut {
		code = c.printVetJSON(stdout, stderr, roots, rep)
	} else {
		code = c.printVet(stderr, roots, rep)
	}
	if len(rep.Crashed()) > 0 && code != exitInternal {
		// An analyzer crashed: its findings are missing, so a CI must not
		// see "clean" (or "findings only").
		return exitInternal
	}
	return code
}

// vettable reports whether every package has types: no package is
// unchecked or has an error other than a phase gate.
func (c *checkCommand) vettable(prog *load.Program, r *check.Result) bool {
	for _, lp := range packageList(prog) {
		p := r.Package(lp)
		if c.dropped[lp] || p == nil {
			continue
		}
		if !p.Checked() || slices.ContainsFunc(p.Errors.Diagnostics(), isBreaking) {
			return false
		}
	}
	return true
}

// printVet prints the findings, a "# path" line before the findings of each
// package, and returns the exit status: 1 if there is a finding.
func (c *checkCommand) printVet(w io.Writer, roots []*load.Package, rep *vet.Report) int {
	var b bytes.Buffer
	last := ""
	for _, f := range rep.Findings {
		if f.Package != last {
			fmt.Fprintf(&b, "# %s\n", f.Package)
			last = f.Package
		}
		fmt.Fprintf(&b, "%s: %s\n", c.vetPos(f.Pos.Filename, f.Pos.Line, f.Pos.Column), f.Message)
		for _, r := range f.Related {
			fmt.Fprintf(&b, "\t%s: %s\n", c.vetPos(r.Pos.Filename, r.Pos.Line, r.Pos.Column), r.Message)
		}
	}
	w.Write(b.Bytes())
	if len(rep.Findings) > 0 {
		return 1
	}
	return 0
}

// vetWhere formats the places of a note: " (at a.gp:3:1, b.gp:9:2, and 4 more)".
func (c *checkCommand) vetWhere(where []token.Position) string {
	const most = 5
	var b strings.Builder
	for i, w := range where {
		if i == most {
			fmt.Fprintf(&b, ", and %d more", len(where)-most)
			break
		}
		if i == 0 {
			b.WriteString(" (at ")
		} else {
			b.WriteString(", ")
		}
		b.WriteString(c.vetPos(w.Filename, w.Line, w.Column))
	}
	if b.Len() > 0 {
		b.WriteString(")")
	}
	return b.String()
}

func (c *checkCommand) vetPos(name string, line, col int) string {
	return fmt.Sprintf("%s:%d:%d", diag.ShortFileName(c.dir, name), line, col)
}

// A jsonFinding is a finding in the output of -json, like go vet's:
// {"pkg": {"analyzer": [{"posn": "file:line:col", "message": "..."}]}}.
type jsonFinding struct {
	Posn    string `json:"posn"`
	End     string `json:"end,omitempty"`
	Message string `json:"message"`
}

// printVetJSON prints the findings as JSON, by package and analyzer, and
// returns 0 (go vet -json exits with 0 too when it could vet).
func (c *checkCommand) printVetJSON(stdout, stderr io.Writer, roots []*load.Package, rep *vet.Report) int {
	out := make(map[string]map[string][]jsonFinding)
	for _, f := range rep.Findings {
		if out[f.Package] == nil {
			out[f.Package] = make(map[string][]jsonFinding)
		}
		jf := jsonFinding{Posn: c.vetPos(f.Pos.Filename, f.Pos.Line, f.Pos.Column), Message: f.Message}
		if f.End.IsValid() {
			jf.End = c.vetPos(f.End.Filename, f.End.Line, f.End.Column)
		}
		out[f.Package][f.Analyzer] = append(out[f.Package][f.Analyzer], jf)
	}
	data, err := json.MarshalIndent(out, "", "\t")
	if err != nil {
		fmt.Fprintf(stderr, "gocpp vet: %v\n", err)
		return exitInternal
	}
	fmt.Fprintf(stdout, "%s\n", data)
	return 0
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
