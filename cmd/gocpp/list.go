package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/token"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// gocpp list (Phase 06 step 6.12): the packages that the patterns name, like go list.

// listUsage is the help text of "gocpp list".
const listUsage = `usage: gocpp list [-deps] [-e] [-f format] [-json] [-tags list] [packages]

gocpp list prints the packages that the arguments name, one import path per line, like go list.
A package is a folder, a list of .gp files of one folder, an import path, or a pattern with "..."
(the same as gocpp check takes). Without arguments it lists the package in the current folder.
The build constraints are for the system that runs gocpp, or for GOOS and GOARCH, and -tags.

-f prints each package with a text/template (Go's syntax) over this structure, and a newline
after each; the functions are join (strings.Join) and json. -json prints each package as a JSON
object with the same fields instead. They are the fields of go list that mean something here
(Go files are .gp files), and a few more for C and C++:

	Dir            the folder
	ImportPath     the import path ("command-line-arguments" for .gp files named on the command line)
	Name           the package name
	Standard       a package of the standard library
	DepOnly        a dependency that no pattern named (with -deps)
	Module         the module: Path, Version, Dir, GoMod (the gocpp.mod file), Main, Vendored
	GpFiles        the .gp files that the build constraints keep, by name
	IgnoredGpFiles the .gp files that the build constraints leave out
	TestGpFiles    the _test.gp files (internal and external tests)
	CFiles         the C files (.c) that the build compiles
	CXXFiles       the C++ files (.cc, .cpp, .cxx) that the build compiles
	CImports       the C and C++ headers that the files import, as written ("<vector>", "./physics.h",
	               "github.com/x/lib/include/x.h"), sorted, each once
	CFlags         the arguments of the //gocpp:cflags lines, as written
	LDFlags        the arguments of the //gocpp:ldflags lines, as written
	Link           the arguments of the //gocpp:link lines
	PkgConfig      the arguments of the //gocpp:pkg-config lines
	Imports        the import paths of the gocpp packages that the files import, sorted
	Deps           all of them, directly or not, sorted
	Incomplete     the package or one that it imports has an error
	Error          the first error of the package: Pos and Err

An empty field is left out of the JSON. -deps lists the dependencies too, every package after the
packages that it imports, like go list -deps. -e keeps going when a package has an error (it is in
the Error field, and also on standard error) instead of failing with status 1: the status is 0, like go list -e.
The patterns "std" and "all" of go list are not supported (the standard library is in the compiler,
not in a folder).

The flags are:
`

// A listPackage is what gocpp list prints for a package: the fields of go list's Package that
// mean something for .gp files, and CImports (listUsage has the list).
type listPackage struct {
	Dir            string
	ImportPath     string
	Name           string      `json:",omitempty"`
	Standard       bool        `json:",omitempty"`
	DepOnly        bool        `json:",omitempty"`
	Module         *listModule `json:",omitempty"`
	GpFiles        []string    `json:",omitempty"`
	IgnoredGpFiles []string    `json:",omitempty"`
	TestGpFiles    []string    `json:",omitempty"`
	CFiles         []string    `json:",omitempty"`
	CXXFiles       []string    `json:",omitempty"`
	CImports       []string    `json:",omitempty"`
	CFlags         []string    `json:",omitempty"`
	LDFlags        []string    `json:",omitempty"`
	Link           []string    `json:",omitempty"`
	PkgConfig      []string    `json:",omitempty"`
	Imports        []string    `json:",omitempty"`
	Deps           []string    `json:",omitempty"`
	Incomplete     bool        `json:",omitempty"`
	Error          *listError  `json:",omitempty"`
}

// A listModule is the module of a listed package.
type listModule struct {
	Path     string
	Version  string `json:",omitempty"`
	Dir      string `json:",omitempty"`
	GoMod    string `json:",omitempty"`
	Main     bool   `json:",omitempty"`
	Vendored bool   `json:",omitempty"`
}

// A listError is the first error of a package.
type listError struct {
	Pos string `json:",omitempty"`
	Err string
}

// runList is "gocpp list".
func runList(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var (
		deps, keepGoing, asJSON bool
		format                  string
		tags                    []string
	)
	flags := flag.NewFlagSet("gocpp list", flag.ContinueOnError)
	flags.BoolVar(&deps, "deps", false, "also list the packages that the named ones import, directly or not")
	flags.BoolVar(&keepGoing, "e", false, "go on when a package has an error: the error is in its Error field")
	flags.BoolVar(&asJSON, "json", false, "print each package as a JSON object")
	flags.StringVar(&format, "f", "", "print each package with this text/template (see above)")
	flags.Var(tagsFlag{&tags}, "tags", "a comma-separated `list` of build tags that are satisfied, like go build -tags")
	if code, done := parseFlags("list", listUsage, flags, args, stderr); done {
		return code
	}
	if asJSON && format != "" {
		fmt.Fprintln(stderr, "gocpp list: -f cannot be used with -json")
		return 2
	}
	var tmpl *template.Template
	if format != "" {
		var err error
		tmpl, err = template.New("").Funcs(template.FuncMap{
			"join": strings.Join,
			"json": func(v any) (string, error) { b, err := json.Marshal(v); return string(b), err },
		}).Option("missingkey=error").Parse(format)
		if err != nil {
			fmt.Fprintf(stderr, "gocpp list: -f: %v\n", err)
			return 2
		}
	}
	c := &checkCommand{stdout: stdout, stderr: stderr, tags: tags}
	var err error
	if c.goos, c.goarch, err = target(); err != nil {
		fmt.Fprintf(stderr, "gocpp: %v\n", err)
		return 2
	}
	if c.dir, err = os.Getwd(); err != nil {
		fmt.Fprintf(stderr, "gocpp list: %v\n", err)
		return exitErrors
	}
	pkgs, code := c.listPackages(flags.Args(), deps, keepGoing)
	if pkgs == nil {
		return code
	}
	for _, p := range pkgs {
		var werr error
		switch {
		case asJSON:
			var data []byte
			if data, werr = json.MarshalIndent(p, "", "\t"); werr == nil {
				_, werr = fmt.Fprintf(stdout, "%s\n", data)
			}
		case tmpl != nil:
			if werr = tmpl.Execute(stdout, p); werr == nil {
				_, werr = fmt.Fprintln(stdout)
			}
		default:
			_, werr = fmt.Fprintln(stdout, p.ImportPath)
		}
		if werr != nil {
			fmt.Fprintf(stderr, "gocpp list: %v\n", werr)
			return exitErrors
		}
	}
	return code
}

// listPackages loads the packages of the patterns and returns what gocpp list prints. Without
// keepGoing, errors in the packages are printed like gocpp check prints them (nothing is listed,
// exit status 1); with it they are in the Error fields and the status is 0.
func (c *checkCommand) listPackages(patterns []string, deps, keepGoing bool) ([]*listPackage, int) {
	m := matchPatterns(c.dir, patterns)
	for _, w := range m.warnings {
		fmt.Fprintln(c.stderr, w)
	}
	prog := &load.Program{Fset: token.NewFileSet(), Errors: new(diag.List)}
	if len(patterns) == 0 || len(m.patterns) > 0 {
		prog = load.Load(load.Config{
			Dir:       c.dir,
			Stdlib:    stdlib.FS(),
			StdlibDir: stdlibDir,
			GoStd:     stdlib.IsStd,
			GOOS:      c.goos,
			GOARCH:    c.goarch,
			Tags:      c.tags,
			AllErrors: true,
		}, m.patterns...)
	}
	c.fset = prog.Fset
	c.files = m.files
	c.headers = make(map[string]*header)
	c.parserErrors = make(map[string]*fileErrors)
	c.dropped = droppedPackages(prog, m)
	c.warnNoMatch(m)
	failed := len(m.errors) > 0 || c.loadFailed(prog)
	if failed && !keepGoing {
		c.printLoadErrors(prog, m)
		return nil, exitErrors
	}
	if failed {
		for _, e := range m.errors {
			fmt.Fprintln(c.stderr, e)
		}
		c.print(c.stderr, prog.Errors, true)
	}
	var list []*load.Package
	if deps {
		for _, p := range packageList(prog) {
			if !c.dropped[p] {
				list = append(list, p)
			}
		}
	} else {
		for _, p := range prog.Roots {
			if !c.dropped[p] {
				list = append(list, p)
			}
		}
	}
	root := make(map[*load.Package]bool)
	for _, p := range prog.Roots {
		root[p] = true
	}
	var out []*listPackage
	code := 0
	if failed {
		code = exitErrors
	}
	for _, p := range list {
		lp := c.listPackage(p)
		lp.DepOnly = !root[p]
		if lp.Error != nil {
			code = exitErrors
		}
		out = append(out, lp)
	}
	if code != 0 && !keepGoing {
		// Errors that gocpp check shows but go list's header read does not (a misplaced
		// directive): a list that fails says why.
		for _, p := range list {
			c.print(c.stderr, p.Errors, true)
		}
		return nil, code
	}
	if keepGoing {
		code = 0 // like go list -e: the errors are in the output, the status is 0
	}
	return out, code
}

// listPackage returns the description of the loaded package p.
func (c *checkCommand) listPackage(p *load.Package) *listPackage {
	lp := &listPackage{
		Dir:        p.Dir,
		ImportPath: p.ImportPath,
		Name:       p.Name,
		Standard:   p.Std,
		CFiles:     slices.Clone(p.CFiles),
		CXXFiles:   slices.Clone(p.CXXFiles),
		Incomplete: p.Incomplete,
	}
	if m := p.Module; m != nil && m.Path != "" {
		lm := &listModule{Path: m.Path, Version: m.Version, Dir: m.Dir, Vendored: m.Vendored, Main: m.Version == "" && !m.Vendored}
		if m.Dir != "" && !m.Vendored && m.File() != nil {
			lm.GoMod = filepath.Join(m.Dir, "gocpp.mod")
		}
		lp.Module = lm
	}
	have := make(map[string]bool)
	includes := make(map[string]bool)
	for _, f := range p.Files {
		name := filepath.Base(f.Path)
		have[name] = true
		lp.GpFiles = append(lp.GpFiles, name)
		if f.Info != nil {
			for _, inc := range f.Info.Includes {
				includes[inc.Path] = true
			}
		}
		for _, d := range f.Directives {
			switch d.Name {
			case "cflags":
				lp.CFlags = append(lp.CFlags, d.Args...)
			case "ldflags":
				lp.LDFlags = append(lp.LDFlags, d.Args...)
			case "link":
				lp.Link = append(lp.Link, d.Args...)
			case "pkg-config":
				lp.PkgConfig = append(lp.PkgConfig, d.Args...)
			}
		}
	}
	lp.CImports = slices.Sorted(maps.Keys(includes))
	lp.Imports = slices.Sorted(maps.Keys(p.Imports))
	seen := make(map[string]bool)
	var visit func(q *load.Package)
	visit = func(q *load.Package) {
		for path, dep := range q.Imports {
			if !seen[path] {
				seen[path] = true
				visit(dep)
			}
		}
	}
	visit(p)
	lp.Deps = slices.Sorted(maps.Keys(seen))
	if !p.Std {
		// The files of the folder that the package does not have: those of the build constraints,
		// and the tests (a package of .gp files named on the command line has the files it names).
		if entries, err := os.ReadDir(p.Dir); err == nil && p.ImportPath != "command-line-arguments" {
			for _, e := range entries {
				n := e.Name()
				switch {
				case e.IsDir() || !strings.HasSuffix(n, ".gp") || strings.HasPrefix(n, ".") || strings.HasPrefix(n, "_"):
				case strings.HasSuffix(n, "_test.gp"):
					lp.TestGpFiles = append(lp.TestGpFiles, n)
				case !have[n]:
					lp.IgnoredGpFiles = append(lp.IgnoredGpFiles, n)
				}
			}
		}
	}
	// The first error (not a warning) of the package.
	for _, d := range p.Errors.Diagnostics() {
		if d.Severity == diag.Error {
			le := &listError{Err: d.Msg}
			if d.Pos.IsValid() {
				le.Pos = fmt.Sprintf("%s:%d:%d", diag.ShortFileName(c.dir, d.Pos.Filename), d.Pos.Line, d.Pos.Column)
			}
			lp.Error = le
			break
		}
	}
	return lp
}
