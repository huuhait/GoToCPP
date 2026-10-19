package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/huuhait/gocpp/internal/diag"
)

// This file has the options that change how diagnostics look (Phase 13 README §2.1, step 13.1):
// the source excerpt with a caret under the column, and color.
//
//	-color=auto|always|never       ANSI colors (auto: when standard error is a terminal)
//	-excerpt=auto|always|never     the source line and a caret under each error
//
// The environment sets the default of the flags: GOCPP_COLOR and GOCPP_EXCERPT (always, never or
// auto), and for color also NO_COLOR and CLICOLOR_FORCE (diag.ColorEnabled has the order of the
// rules). Without a terminal (a pipe, a file, a test, an editor's build task), the output is the
// plain Go style "file:line:col: message" that tools read, with no excerpt and no color.

// excerptWidth is the most terminal cells of a source line in an excerpt; a longer line is cut
// around the column (diag.Printer.MaxWidth).
const excerptWidth = 100

// A diagOptions holds the -color and -excerpt flags.
type diagOptions struct {
	color   modeFlag
	excerpt modeFlag
}

// A modeFlag is the value of a flag that is auto, always or never.
type modeFlag string

func (m *modeFlag) String() string { return string(*m) }

func (m *modeFlag) Set(s string) error {
	v, ok := diag.ParseColorMode(s)
	if !ok {
		return fmt.Errorf("want auto, always or never")
	}
	*m = modeFlag(v)
	return nil
}

// register adds the flags to flags.
func (o *diagOptions) register(flags *flag.FlagSet) {
	o.color, o.excerpt = "auto", "auto"
	flags.Var(&o.color, "color", "color the diagnostics: auto (a terminal), always or never (GOCPP_COLOR, NO_COLOR, CLICOLOR_FORCE)")
	flags.Var(&o.excerpt, "excerpt", "show the source line and a caret under each error: auto (a terminal), always or never (GOCPP_EXCERPT)")
}

// colorOn reports whether the diagnostics written to w are colored.
func (o diagOptions) colorOn(w io.Writer) bool {
	return diag.ColorEnabled(string(o.color), w, os.Getenv)
}

// excerptOn reports whether the diagnostics written to w come with source excerpts.
func (o diagOptions) excerptOn(w io.Writer) bool {
	switch string(o.excerpt) {
	case "always":
		return true
	case "never":
		return false
	}
	switch os.Getenv("GOCPP_EXCERPT") {
	case "always":
		return true
	case "never":
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return diag.IsTerminal(w)
}

// printer returns the printer of the diagnostics written to w, for the current folder dir.
// all prints every error (-e).
func (o diagOptions) printer(w io.Writer, dir string, all bool) diag.Printer {
	p := diag.Printer{All: all, Dir: dir, Color: o.colorOn(w)}
	if o.excerptOn(w) {
		p.Source, p.Related, p.WholeLine = true, true, true
		p.Indent, p.MaxWidth = "\t", excerptWidth
	}
	return p
}
