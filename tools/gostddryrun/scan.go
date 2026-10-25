package main

// The scan of one package's selected Go files: bodyless functions, //go:linkname
// directives of every kind, and the compiler intrinsics of Phase 16 README 2.7.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
)

// A Bodyless is a function declared without a body in the selected files: its
// implementation is assembly, another package (a linkname pull), or the
// compiler (an intrinsic).
type Bodyless struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	Name      string `json:"name"`
	Signature string `json:"signature"`
}

// A Linkname is one //go:linkname directive.
type Linkname struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Local  string `json:"local"`
	Target string `json:"target"` // "" for the one-argument form
	Kind   string `json:"kind"`   // pull, push, one-argument, variable, linker, unknown
	Decl   string `json:"decl"`   // func, var, or "" when the local name is not declared
}

// An Intrinsic is one of the compiler intrinsics that the GC engine must
// provide as natives.
type Intrinsic struct {
	Package string `json:"package"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Name    string `json:"name"`
}

// intrinsicNames are the compiler intrinsics Phase 16 README 2.7 names. They
// are bodyless functions whose implementation is the compiler, not a linkname.
var intrinsicNames = map[string]bool{
	"internal/abi.FuncPCABI0":            true,
	"internal/abi.FuncPCABIInternal":     true,
	"internal/runtime/sys.GetCallerPC":   true,
	"internal/runtime/sys.GetCallerSP":   true,
	"internal/runtime/sys.GetClosurePtr": true,
}

// PackageScan is the result of scanning one package. The import closure parses
// the imports on its own (select.go); the scan does not need them.
type PackageScan struct {
	Name       string            `json:"name"`
	Bodyless   []Bodyless        `json:"bodyless"`
	Linknames  []Linkname        `json:"linknames"`
	Intrinsics []Intrinsic       `json:"intrinsics"`
	Symbols    map[string]string `json:"-"` // package-scope name -> func-bodyless, func-body, var
}

// scanPackage parses the selected files of the package at dir.
func scanPackage(pkg, dir string, files []string) (*PackageScan, error) {
	out := &PackageScan{Name: "", Symbols: map[string]string{}}
	fset := token.NewFileSet()
	parsed := make([]*ast.File, 0, len(files))
	for _, name := range files {
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, f)
		if out.Name == "" {
			out.Name = f.Name.Name
		} else if out.Name != f.Name.Name {
			return nil, fmt.Errorf("%s: package %s and %s in one directory", dir, out.Name, f.Name.Name)
		}
	}

	// The package scope decides whether a linkname names a func or a var, as
	// the compiler's package-scope lookup does (cmd/compile noder.go).
	for _, f := range parsed {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				kind := "func-body"
				if d.Body == nil {
					kind = "func-bodyless"
				}
				// The compiler looks a linkname's local name up in the package
				// scope, which has no methods.
				if d.Recv == nil {
					out.Symbols[d.Name.Name] = kind
				}
			case *ast.GenDecl:
				if d.Tok != token.VAR {
					continue
				}
				for _, s := range d.Specs {
					vs, ok := s.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, n := range vs.Names {
						out.Symbols[n.Name] = "var"
					}
				}
			}
		}
	}

	for _, f := range parsed {
		base := fset.Position(f.Pos()).Filename
		base = filepath.Base(base)
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body != nil {
				continue
			}
			pos := fset.Position(fd.Pos())
			sig, err := signature(fset, fd)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %v", base, pos.Line, err)
			}
			out.Bodyless = append(out.Bodyless, Bodyless{
				File:      base,
				Line:      pos.Line,
				Name:      fd.Name.Name,
				Signature: sig,
			})
			if intrinsicNames[pkg+"."+fd.Name.Name] {
				out.Intrinsics = append(out.Intrinsics, Intrinsic{
					Package: pkg,
					File:    base,
					Line:    pos.Line,
					Name:    fd.Name.Name,
				})
			}
		}
		// A //go:linkname directive counts wherever it is in the file, not only
		// as a doc comment: the compiler collects every pragma of the file
		// (cmd/compile/internal/noder/noder.go, pragma), and runtime/linkname.go
		// keeps its directives in comment groups of their own.
		for _, cg := range f.Comments {
			for _, c := range cg.List {
				if !strings.HasPrefix(c.Text, "//go:linkname ") {
					continue
				}
				fields := strings.Fields(strings.TrimPrefix(c.Text, "//"))
				if len(fields) < 2 {
					continue
				}
				ln := Linkname{
					File:  base,
					Line:  fset.Position(c.Pos()).Line,
					Local: fields[1],
				}
				if len(fields) >= 3 {
					ln.Target = fields[2]
				}
				ln.Decl = out.Symbols[ln.Local]
				if ln.Decl == "" {
					ln.Decl = "unknown"
				}
				ln.Kind = classifyLinkname(ln.Target, out.Symbols[ln.Local])
				out.Linknames = append(out.Linknames, ln)
			}
		}
	}

	sort.Slice(out.Bodyless, func(i, j int) bool {
		if out.Bodyless[i].File != out.Bodyless[j].File {
			return out.Bodyless[i].File < out.Bodyless[j].File
		}
		return out.Bodyless[i].Line < out.Bodyless[j].Line
	})
	sort.Slice(out.Linknames, func(i, j int) bool {
		a, b := out.Linknames[i], out.Linknames[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Local < b.Local
	})
	return out, nil
}

// classifyLinkname classifies one //go:linkname directive by Phase 16 README
// 2.7: one-argument (exports the local symbol only), linker symbol
// (go:fipsinfo and the like: a target with no package), variable (a var, not
// a func), pull (a bodyless function that names another package's symbol),
// push (a function with a body that names another package's symbol).
func classifyLinkname(target, symbol string) string {
	switch {
	case target == "":
		return "one-argument"
	case !strings.Contains(target, "."):
		// go:fipsinfo and the _cgo_* entry points: a linker symbol, even when
		// the local declaration is a var.
		return "linker"
	case symbol == "var":
		return "variable"
	case symbol == "func-body":
		return "push"
	case symbol == "func-bodyless":
		return "pull"
	}
	return "unknown"
}

// signature prints a function declaration without its body, as go/format
// writes it: the receiver, name, parameters and results.
func signature(fset *token.FileSet, fd *ast.FuncDecl) (string, error) {
	c := *fd
	c.Doc = nil
	c.Body = nil
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, &c); err != nil {
		return "", err
	}
	return buf.String(), nil
}
