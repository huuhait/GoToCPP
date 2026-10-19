package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/explain"
	"github.com/huuhait/gocpp/internal/lower"
	"github.com/huuhait/gocpp/stdlib"
)

// annotateFlags are the flags of gocpp emit that make the annotated view (Phase 13 step 13.5):
// -annotate, and the flags that only change it.
type annotateFlags struct {
	on       bool   // -annotate
	json     bool   // -json
	nochecks bool   // -nochecks
	kinds    string // -kinds
	output   string // -o
	std      bool   // -std
}

func (a *annotateFlags) register(flags *flag.FlagSet) {
	flags.BoolVar(&a.on, "annotate", false, "print the C++ with the .gp lines it comes from and the reasons for the lowering's choices (for reading, not compiling)")
	flags.BoolVar(&a.json, "json", false, "with -annotate: print the view as JSON")
	flags.BoolVar(&a.nochecks, "nochecks", false, "with -annotate: say that the bounds and nil checks are removed, as gocpp build -nochecks does")
	flags.StringVar(&a.kinds, "kinds", "", "with -annotate: only the reasons of these kinds (a comma-separated list, see gocpp explain -list)")
	flags.BoolVar(&a.std, "std", false, "with -annotate: also the C++ of the standard library's packages (the default is the program's own packages)")
	flags.StringVar(&a.output, "o", "", "with -annotate: write the view to this file instead of the standard output")
}

// validate reports a flag that needs -annotate used without it, or a kind that does not exist.
func (a *annotateFlags) validate(stderr io.Writer) (code int, bad bool) {
	if !a.on {
		for _, f := range []struct {
			name string
			set  bool
		}{{"-json", a.json}, {"-nochecks", a.nochecks}, {"-kinds", a.kinds != ""}, {"-o", a.output != ""}, {"-std", a.std}} {
			if f.set {
				fmt.Fprintf(stderr, "gocpp emit: %s needs -annotate\n", f.name)
				return 2, true
			}
		}
		return 0, false
	}
	if _, err := a.kindSet(); err != nil {
		fmt.Fprintf(stderr, "gocpp emit: %v\n", err)
		return 2, true
	}
	return 0, false
}

// kindSet returns the kinds of -kinds, or nil for all of them.
func (a *annotateFlags) kindSet() (map[lower.Kind]bool, error) {
	if a.kinds == "" {
		return nil, nil
	}
	set := make(map[lower.Kind]bool)
	for _, k := range strings.Split(a.kinds, ",") {
		k = strings.TrimSpace(k)
		if !explain.IsKind(k) {
			return nil, fmt.Errorf("-kinds: unknown kind %q (the kinds are: %s)", k, strings.Join(explain.KindNames(), ", "))
		}
		set[lower.Kind(k)] = true
	}
	return set, nil
}

// annotated builds the annotated view of the program set: it emits the C++ with the reason log
// on, in a folder that does not exist (explain.GenRoot), so that the lines without a .gp place
// can be told apart.
func (d *driver) annotated(set *buildSet, nochecks, std bool, kinds map[lower.Kind]bool) (*explain.View, int) {
	d.why = lower.NewWhy()
	d.why.NoChecks = nochecks
	files, code := d.emit(set, explain.GenRoot, nil)
	if files == nil {
		return nil, code
	}
	in := make([]explain.File, len(files.Files))
	for i, f := range files.Files {
		in[i] = explain.File{Path: f.Path, Data: f.Data, Std: f.Std}
	}
	return explain.Build(in, d.why.Reasons(), explain.Options{Source: readGP, Kinds: kinds, Std: std}), 0
}

// readGP returns the text of a .gp file as a #line directive names it: a file on disk, or a
// file of the embedded standard library ($GOCPP/stdlib/...). It returns nil if it can't.
func readGP(path string) []byte {
	if rest, ok := strings.CutPrefix(strings.ReplaceAll(path, `\`, "/"), stdlibDir+"/"); ok {
		data, err := fs.ReadFile(stdlib.FS(), rest)
		if err != nil {
			return nil
		}
		return data
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}

// emitAnnotated is gocpp emit -annotate.
func (a *annotateFlags) run(d *driver, set *buildSet, stdout, stderr io.Writer) int {
	kinds, _ := a.kindSet()
	v, code := d.annotated(set, a.nochecks, a.std, kinds)
	if v == nil {
		return code
	}
	var out io.Writer = stdout
	if a.output != "" {
		outAbs, _ := filepath.Abs(a.output)
		for _, in := range v.SourceFiles() {
			if samePath(outAbs, in, d.check.dir) {
				fmt.Fprintf(stderr, "gocpp emit: -o %s would overwrite a file that is being emitted\n", a.output)
				return 2
			}
		}
		f, err := os.Create(a.output)
		if err != nil {
			fmt.Fprintf(stderr, "gocpp emit: %v\n", err)
			return exitErrors
		}
		defer f.Close()
		out = f
	}
	var err error
	if a.json {
		err = v.Rel(d.showPath).WriteJSON(out)
	} else {
		err = v.WriteText(out, func(p string) string { return d.showPath(p) })
	}
	if err != nil {
		fmt.Fprintf(stderr, "gocpp emit: %v\n", err)
		return exitErrors
	}
	return 0
}

// showPath shortens the path of a .gp file the way the messages of gocpp do.
func (d *driver) showPath(p string) string {
	return diag.ShortFileName(d.check.dir, p)
}
