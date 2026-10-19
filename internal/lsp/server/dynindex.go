package server

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/huuhait/gocpp/internal/lsp/cache"
)

// The syntax index of the dynamic family (dynamic.go).
//
// A rename of a method has to know whether any package of the module takes a
// value out as an interface that has the method (a type assertion, a type
// switch, errors.As), also in a package that the search would not look at (core
// asserts x.(Closer) on what main passes it, and does not import the package
// of the type). Checking every package of the module for that costs a type
// check of each, so a table of the files by their content tells first, from
// the parse alone (which the workspace symbols keep warm), which packages
// declare an interface with a method of the name and which have an assertion
// at all; only the packages that can reach such an interface are checked. A
// file is looked at again only when its content changes.

// A fileSyn is what the dynamic family needs to know of a file.
type fileSyn struct {
	hash cache.Hash
	// ifaceMethods are the names of the methods declared in the interface types
	// of the file (a named interface, an interface literal, a constraint).
	ifaceMethods map[string]bool
	// asserts: the file has a type assertion, a type switch or a call of a
	// function named As (errors.As).
	asserts bool
	// strs are the string literals of the file that look like a name (reflect
	// and the encoders find a method or a field by a string), with the line of
	// the first.
	strs map[string]int
}

// synTable holds the fileSyn of each file, for the content that they were made from.
type synTable struct {
	mu sync.Mutex
	m  map[string]*fileSyn
}

// maxSynFiles is the number of files whose index is kept before the table starts over.
const maxSynFiles = 100000

// fileSyntax returns the index of the file path in the snapshot.
func (s *Server) fileSyntax(ctx context.Context, snap *cache.Snapshot, path string) *fileSyn {
	h, err := snap.FileHash(path)
	if err != nil {
		return nil
	}
	s.syn.mu.Lock()
	fs := s.syn.m[path]
	s.syn.mu.Unlock()
	if fs != nil && fs.hash == h {
		return fs
	}
	pf, err := snap.ParsePath(ctx, path, cache.ParseFull)
	if err != nil || pf.File == nil {
		return nil
	}
	fs = &fileSyn{hash: h}
	ast.Inspect(pf.File, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.InterfaceType:
			if n.Methods != nil {
				for _, f := range n.Methods.List {
					if _, ok := f.Type.(*ast.FuncType); ok {
						for _, id := range f.Names {
							if fs.ifaceMethods == nil {
								fs.ifaceMethods = make(map[string]bool)
							}
							fs.ifaceMethods[id.Name] = true
						}
					}
				}
			}
		case *ast.BasicLit:
			if n.Kind == token.STRING && len(n.Value) > 2 && len(n.Value) <= 42 {
				if v, err := strconv.Unquote(n.Value); err == nil && token.IsIdentifier(v) {
					if fs.strs == nil {
						fs.strs = make(map[string]int)
					}
					if _, ok := fs.strs[v]; !ok {
						fs.strs[v] = pf.Tok.Line(n.Pos())
					}
				}
			}
		case *ast.TypeAssertExpr:
			fs.asserts = fs.asserts || n.Type != nil
		case *ast.TypeSwitchStmt:
			fs.asserts = true
		case *ast.CallExpr:
			switch f := n.Fun.(type) {
			case *ast.SelectorExpr:
				fs.asserts = fs.asserts || f.Sel.Name == "As"
			case *ast.Ident:
				fs.asserts = fs.asserts || f.Name == "As"
			}
		}
		return true
	})
	s.syn.mu.Lock()
	if s.syn.m == nil || len(s.syn.m) >= maxSynFiles {
		s.syn.m = make(map[string]*fileSyn)
	}
	s.syn.m[path] = fs
	s.syn.mu.Unlock()
	return fs
}

// discoverDynamic finds the dynamic targets of the module on an interface that
// has a method of the symbol's name, for a rename of a method, into r.dyn. The
// packages that it checks are those that declare such an interface (or have
// an interface literal of the kind), those that import them, directly or not,
// and have an assertion: no other package can take a value out as the
// interface. It needs the imports of the module (loadImports), and does
// nothing without them.
func (r *searcher) discoverDynamic(ctx context.Context) error {
	sym := r.sym
	if !sym.rename || r.s.wideFamily || r.local || r.imports == nil || len(sym.objs) != 1 {
		return nil
	}
	fn, ok := sym.objs[0].(*types.Func)
	if !ok || recvBase(fn) == nil {
		return nil
	}
	name := sym.name
	declaring := make(map[string]bool)
	asserting := make(map[string]bool)
	testAsserting := make(map[string]bool)
	for _, p := range r.pkgs {
		files, err := r.snap.SourceFiles(p.dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			fs := r.s.fileSyntax(ctx, r.snap, f)
			if fs == nil {
				continue
			}
			if fs.ifaceMethods[name] {
				declaring[p.dir] = true
			}
			if fs.asserts {
				if strings.HasSuffix(f, "_test.gp") {
					testAsserting[p.dir] = true // (the test files are checked with the package)
				} else {
					asserting[p.dir] = true
				}
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if len(declaring) == 0 {
		return nil
	}
	// The packages that can see an interface with the method.
	reach := make(map[string]bool, len(declaring))
	for d := range declaring {
		reach[d] = true
		for _, p := range r.pkgs {
			if p.dir == d {
				for _, imp := range r.importers(p.path, true) {
					reach[imp] = true
				}
			}
		}
	}
	var cand []string
	for d := range reach {
		if asserting[d] {
			cand = append(cand, d)
		}
		if testAsserting[d] {
			if r.dynTest == nil {
				r.dynTest = make(map[string]bool)
			}
			r.dynTest[d] = true
		}
	}
	if len(cand) == 0 {
		return nil
	}
	slices.Sort(cand)
	views, _, err := r.views(ctx, cand)
	if err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, v := range views {
		for _, t := range v.assertions() {
			if !t.names[name] {
				continue
			}
			if dt := newDynTarget(t, v); dt != nil && !seen[dt.id()] {
				seen[dt.id()] = true
				r.dyn = append(r.dyn, dt)
			}
		}
	}
	return nil
}

// nameAsString finds a string literal in the files of dirs that is the name,
// and returns its place (file:line), or "".
func (s *Server) nameAsString(ctx context.Context, snap *cache.Snapshot, dirs []string, name string) string {
	for _, dir := range dirs {
		files, err := snap.SourceFiles(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if fs := s.fileSyntax(ctx, snap, f); fs != nil {
				if line, ok := fs.strs[name]; ok {
					return fmt.Sprintf("%s:%d", filepath.Base(f), line)
				}
			}
		}
	}
	return ""
}
