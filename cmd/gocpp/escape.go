package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/lower"
)

// The escape analysis flags of gocpp build, run and emit (Phase 15 step 15.2, ADR-114):
//
//	-gp:m      print the decisions of the escape analysis, like go build -gcflags=-m: what stays on
//	           the stack ("&x does not escape", "p does not escape") and what moves to the heap
//	           ("moved to heap: x", "&T{...} escapes to heap", "leaking param: p")
//	-gp:m=2    also say why
//	-gp:noescape  turn the analysis off: every variable whose address is taken is boxed and every
//	           &T{...} and new(T) allocates (for finding out whether a bug is the analysis's)

// An escapeLevel is the value of -gp:m: a flag that is a boolean (-gp:m) or a level (-gp:m=2).
type escapeLevel int

func (l *escapeLevel) String() string {
	if l == nil {
		return "0"
	}
	return strconv.Itoa(int(*l))
}

func (l *escapeLevel) IsBoolFlag() bool { return true }

func (l *escapeLevel) Set(s string) error {
	switch s {
	case "true", "1":
		*l = 1
	case "false", "0":
		*l = 0
	case "2":
		*l = 2
	default:
		return fmt.Errorf("must be 1 or 2")
	}
	return nil
}

// newEscapeLog returns the sink for the flags, or nil when -gp:m is off.
func (f *buildFlags) newEscapeLog() *lower.EscapeLog {
	if f.escape == 0 {
		return nil
	}
	return lower.NewEscapeLog(int(f.escape))
}

// printEscape writes the diagnostics of the sink to w, one section for each package, with the
// paths relative to the current folder as go build writes them.
func printEscape(w io.Writer, log *lower.EscapeLog) {
	if log == nil {
		return
	}
	wd, _ := os.Getwd()
	last := ""
	for _, l := range log.Lines() {
		if l.Pkg != last {
			fmt.Fprintf(w, "# %s\n", l.Pkg)
			last = l.Pkg
		}
		fmt.Fprintf(w, "%s:%d:%d: %s\n", escapePath(wd, l.File), l.Line, l.Col, l.Msg)
	}
}

// escapePath returns the path of file as go build prints it: relative to the working folder with a
// leading "./" when it is below it, else as it is.
func escapePath(wd, file string) string {
	if wd == "" || !filepath.IsAbs(file) {
		return file
	}
	rel, err := filepath.Rel(wd, file)
	if err != nil || strings.HasPrefix(rel, "..") {
		return file
	}
	return "./" + filepath.ToSlash(rel)
}

// registerEscape registers -gp:m and -gp:noescape.
func (f *buildFlags) registerEscape(flags *flag.FlagSet) {
	flags.Var(&f.escape, "gp:m",
		"print the decisions of the escape analysis, like go build -gcflags=-m (-gp:m=2: with the reasons)")
	flags.BoolVar(&f.noEscape, "gp:noescape", false,
		"turn the escape analysis off: every address-taken variable is boxed, every &T{...} allocates")
}
