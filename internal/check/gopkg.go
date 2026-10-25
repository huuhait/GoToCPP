package check

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"runtime"
	"strings"
)

// The GC engine's type checker: go/types over the stock go/parser's AST (phase-16 README
// §2.3 rule 1 and §2.7, job 16.1c). It is a separate path from Check, which is built around
// gocpp's loader, its macros, its C/C++ interop and the phase gates; none of that applies to a
// .go package. Job 16.7/16.8 replace this with the GC engine's own checker and lowering.
//
// The checker uses Go's sizes (types.SizesFor("gc", GOARCH)), because the GC engine writes
// Go's layout: int is 8 bytes on amd64 and arm64, and the emitter writes int64_t (README §2.2).
// An import is an error for now: the GC engine's world has Go's standard library and Go
// modules (README §2.3 rule 4), and steps 16.9-16.13 build that. A package main without
// imports is what job 16.1c proves.

// goImporter rejects every import with a message that says what is missing.
type goImporter struct{}

func (goImporter) Import(path string) (*types.Package, error) {
	return nil, fmt.Errorf("package %s: imports are not supported by the GC engine yet (job 16.1c: Go source without imports; Go's standard library comes with steps 16.9-16.13)", path)
}

// CheckGo type-checks the files of a Go package with go/types and returns the package and the
// information that the GC emitter reads (Defs, Uses and Types). The files must come from fset.
// All type errors are returned, one per line, each with its file position.
func CheckGo(fset *token.FileSet, files []*ast.File) (*types.Package, *types.Info, error) {
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	var errs []string
	conf := types.Config{
		Importer: goImporter{},
		Sizes:    types.SizesFor("gc", runtime.GOARCH),
		Error: func(err error) {
			errs = append(errs, err.Error())
		},
	}
	pkg, err := conf.Check("main", fset, files, info)
	if err != nil {
		if len(errs) == 0 {
			errs = append(errs, err.Error())
		}
		return nil, nil, errors.New(strings.Join(errs, "\n"))
	}
	return pkg, info, nil
}
