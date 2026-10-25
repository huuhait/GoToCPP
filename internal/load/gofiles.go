package load

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// The GC engine's loader: a folder of .go files, parsed with the stock go/parser (phase-16
// README §2.3 rule 1 and §2.7, job 16.1c). It is a separate, small path: the .gp loader
// (loader.go) parses gocpp's own syntax, rewrites macros and directives, and resolves gocpp
// modules, and none of that applies to Go source. Job 16.7 replaces this with the full Go
// loader: build constraints, _test.go files, //go:embed, cgo files, modules and imports.

// A GoPackage is a folder of .go files of one Go package, loaded for the GC engine.
type GoPackage struct {
	Dir   string // the absolute folder
	Fset  *token.FileSet
	Files []*ast.File // the parsed files, in the order of Paths
	Paths []string    // the absolute names of the files, in the same order
	Name  string      // the package name from the package clauses
}

// LoadGoDir loads the .go files of the folder dir. A folder that holds both .gp and .go files
// is an error (phase-16 README §2.3 rule 1: a package is built by exactly one engine), and so
// is a folder with only .gp files: the ARC engine builds those. _test.go files are ignored
// until job 16.7 (gocpp test for Go packages is step 16.12). Only a package main is built in
// job 16.1c: a program is what the walking skeleton proves.
func LoadGoDir(dir string) (*GoPackage, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("%s is not a folder: the GC engine builds a folder of .go files (job 16.1c)", dir)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	var names []string
	gp := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		switch {
		case strings.HasSuffix(name, ".gp"):
			gp = true
		case strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go"):
			names = append(names, name)
		}
	}
	switch {
	case gp && len(names) > 0:
		return nil, fmt.Errorf("%s has both .gp and .go files: a package is built by exactly one engine (phase-16 README §2.3 rule 1)", abs)
	case gp:
		return nil, fmt.Errorf("%s has no .go files: a package of .gp files is built by the ARC engine (phase-16 README §2.3 rule 1)", abs)
	case len(names) == 0:
		return nil, fmt.Errorf("no .go files in %s", abs)
	}
	slices.Sort(names)

	p := &GoPackage{Dir: abs, Fset: token.NewFileSet()}
	for _, name := range names {
		path := filepath.Join(abs, name)
		f, err := parser.ParseFile(p.Fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		if p.Name == "" {
			p.Name = f.Name.Name
		} else if f.Name.Name != p.Name {
			return nil, fmt.Errorf("%s: package %s, but the folder's other files are package %s", path, f.Name.Name, p.Name)
		}
		p.Files = append(p.Files, f)
		p.Paths = append(p.Paths, path)
	}
	if p.Name != "main" {
		return nil, fmt.Errorf("%s: package %s: the GC engine builds only package main in job 16.1c (phase-16 README §3 step 16.1)", abs, p.Name)
	}
	return p, nil
}
