package server

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// Auto-import (step 11.6): the completion of the members of a package that
// the document doesn't import yet (strings. without import "strings"), with
// an edit that adds the import. The packages that a document can import
// are those of the embedded standard library and of the document's module;
// their index (name → import paths) is made once, in the background: the
// standard library's once per server, a module's when a document of it is
// opened, again after files change on disk (didChangeWatchedFiles). A
// request waits for it only a little (importIndex.wait); the members come
// from the summaries of the packages (cache.Snapshot.Exports).

// An importable is a package that a document can import.
type importable struct {
	name string // its package name
	path string // its import path
	dir  string // its folder (in Config.StdlibDir for a standard package)
	std  bool
	main bool // a main package: nothing can import it (the workspace features want it, the imports don't)
}

// An importIndex is the index of the importable packages.
type importIndex struct {
	pipeline *cache.Pipeline
	// wait is how long a request waits for the index of a module that is
	// being made; then it goes on with the standard packages only.
	wait time.Duration
	// ctx is done when the server shuts down: the walks stop (each one
	// also stops when a newer generation starts).
	ctx context.Context

	stdOnce  sync.Once
	std      []importable
	stdPaths map[string]bool

	mu   sync.Mutex
	gen  uint64 // one more after each change of files on disk
	mods map[string]*modIndex
}

// A modIndex is the index of the packages of one module.
type modIndex struct {
	gen    uint64
	done   chan struct{}
	cancel context.CancelFunc // stops the walk
	pkgs   []importable       // when done is closed
	prev   []importable       // the index of the generation before, or nil
}

// defaultIndexWait is importIndex.wait of the server.
const defaultIndexWait = 20 * time.Millisecond

// maxIndexDirs is the number of folders that the walk of a module looks
// at, at most: a huge tree (a checkout with node_modules) has an index of
// its first folders only.
const maxIndexDirs = 20000

func newImportIndex(ctx context.Context, p *cache.Pipeline) *importIndex {
	return &importIndex{pipeline: p, wait: defaultIndexWait, ctx: ctx, mods: make(map[string]*modIndex)}
}

// changed takes in files that changed on disk (didChangeWatchedFiles):
// a file or folder that is created or deleted, or a changed gocpp.mod
// (its module path), makes the module indexes old: the next request makes
// them again (in the meantime, it gets the old ones). A saved .gp file
// changes nothing (its package name could, which the index then misses
// until the next such change).
func (ix *importIndex) changed(events []protocol.FileEvent) {
	for _, e := range events {
		if e.Type == protocol.Created || e.Type == protocol.Deleted || filepath.Base(e.URI.Path()) == "gocpp.mod" {
			ix.mu.Lock()
			ix.gen++
			ix.mu.Unlock()
			return
		}
	}
}

// generation returns the number of the changes of files on disk that made the
// module indexes old (changed): it is the same while nothing was created,
// deleted or is a gocpp.mod.
func (ix *importIndex) generation() uint64 {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	return ix.gen
}

// warm starts the index of the module of the folder dir, if it has one.
func (ix *importIndex) warm(dir string) {
	go func() {
		if mod := load.FindModule(dir); mod != nil && mod.Path != "" {
			ix.module(mod, false)
		}
	}()
}

// stdPackages returns the standard packages.
func (ix *importIndex) stdPackages() []importable {
	ix.stdOnce.Do(func() {
		ix.stdPaths = make(map[string]bool)
		for _, p := range ix.pipeline.StdPackages() {
			names, srcs := ix.pipeline.StdFiles(p)
			name := path.Base(p)
			for i, n := range names {
				if strings.HasSuffix(n, "_test.gp") {
					continue
				}
				if pn := packageName(n, srcs[i]); pn != "" {
					name = pn
				}
				break
			}
			ix.std = append(ix.std, importable{name: name, path: p, dir: filepath.Dir(names[0]), std: true})
			ix.stdPaths[p] = true
		}
	})
	return ix.std
}

// isStd reports whether path is the import path of a standard package.
func (ix *importIndex) isStd(path string) bool {
	ix.stdPackages()
	return ix.stdPaths[path]
}

// packageName returns the package name in the package clause of the file,
// or "".
func packageName(name string, src []byte) string {
	f, _ := parser.ParseFile(token.NewFileSet(), name, src, parser.PackageClauseOnly)
	if f == nil || f.Name == nil || f.Name.Name == "_" {
		return ""
	}
	return f.Name.Name
}

// module returns the packages of the module mod that can be imported: its
// index, or, while it is being made, the one before. With wait, it waits for
// the new one for up to ix.wait, if there is no index before.
func (ix *importIndex) module(mod *load.Module, wait bool) []importable {
	all := ix.moduleAll(mod, wait)
	pkgs := make([]importable, 0, len(all))
	for _, p := range all {
		if !p.main {
			pkgs = append(pkgs, p)
		}
	}
	return pkgs
}

// moduleWait returns the packages of the module mod, main packages too, after
// its index is made (it waits for as long as ctx lives): for the requests
// that must see the whole workspace (workspace symbols).
func (ix *importIndex) moduleWait(ctx context.Context, mod *load.Module) ([]importable, error) {
	ix.moduleAll(mod, false) // starts the walk, if the index is old
	ix.mu.Lock()
	m := ix.mods[mod.Dir]
	ix.mu.Unlock()
	if m == nil {
		return nil, nil
	}
	select {
	case <-m.done:
		return m.pkgs, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// moduleAll is module with the main packages too (rename and references
// look in them, since they import packages).
func (ix *importIndex) moduleAll(mod *load.Module, wait bool) []importable {
	ix.mu.Lock()
	m := ix.mods[mod.Dir]
	if m == nil || m.gen != ix.gen {
		ctx, cancel := context.WithCancel(ix.ctx)
		next := &modIndex{gen: ix.gen, done: make(chan struct{}), cancel: cancel}
		if m != nil {
			select {
			case <-m.done:
				next.prev = m.pkgs
			default:
				// The walk of an older generation is useless now.
				m.cancel()
				next.prev = m.prev
			}
		}
		m = next
		ix.mods[mod.Dir] = m
		go func() {
			defer cancel()
			pkgs := ix.walk(ctx, mod)
			if ctx.Err() != nil {
				pkgs = m.prev // stopped: what it found is not all
			}
			m.pkgs = pkgs
			close(m.done)
		}()
	}
	ix.mu.Unlock()
	select {
	case <-m.done:
		return m.pkgs
	default:
	}
	if m.prev != nil || !wait {
		return m.prev
	}
	t := time.NewTimer(ix.wait)
	defer t.Stop()
	select {
	case <-m.done:
		return m.pkgs
	case <-t.C:
		return nil
	}
}

// walk returns the packages of the module mod, from the files on disk:
// each folder with a .gp file that is not a test, but those that the go
// command leaves out too (testdata, and the folders whose names start with
// . or _) and those of nested modules (with a gocpp.mod of their own). The
// main packages are in the list, marked (importable.main).
func (ix *importIndex) walk(ctx context.Context, mod *load.Module) []importable {
	var pkgs []importable
	dirs := 0
	var visit func(dir, rel string)
	visit = func(dir, rel string) {
		if dirs >= maxIndexDirs || ctx.Err() != nil {
			return
		}
		dirs++
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		var sub []string
		name := ""
		for _, e := range entries {
			n := e.Name()
			switch {
			case e.IsDir():
				if !strings.HasPrefix(n, ".") && !strings.HasPrefix(n, "_") && n != "testdata" {
					sub = append(sub, n)
				}
			case rel != "." && n == "gocpp.mod":
				return // another module
			case name == "" && strings.HasSuffix(n, ".gp") && !strings.HasSuffix(n, "_test.gp") &&
				!strings.HasPrefix(n, ".") && !strings.HasPrefix(n, "_"):
				src, err := readHeader(filepath.Join(dir, n))
				if err == nil {
					name = packageName(n, src)
				}
			}
		}
		if name != "" {
			pkgs = append(pkgs, importable{name: name, path: joinPath(mod.Path, rel), dir: dir, main: name == "main"})
		}
		for _, n := range sub {
			visit(filepath.Join(dir, n), path.Join(rel, n))
		}
	}
	visit(mod.Dir, ".")
	return pkgs
}

// readHeader reads the start of a file, which has its package clause
// (unless a long comment comes first: then the whole file).
func readHeader(name string) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, 4096)
	n, _ := f.Read(buf)
	if n < len(buf) || packageName(name, buf[:n]) != "" {
		return buf[:n], nil
	}
	return os.ReadFile(name)
}

// joinPath returns the import path of the folder rel (slash-separated) of
// the module with the path mod.
func joinPath(mod, rel string) string {
	if rel == "." {
		return mod
	}
	return mod + "/" + rel
}

// importables returns the packages that the document of q can import,
// but its own package, those it imports already and the internal packages
// it may not import, sorted by import path. The open documents count too:
// a new folder that has only unsaved files.
func (ix *importIndex) importables(q *query) []importable {
	all := slices.Clone(ix.stdPackages())
	if mod := q.lp.Module; mod != nil && mod.Path != "" && !q.lp.Std {
		all = append(all, ix.module(mod, true)...)
		known := make(map[string]bool)
		for _, p := range all {
			known[p.dir] = true
		}
		for _, o := range q.snap.Overlays() {
			name := o.URI().Path()
			dir := filepath.Dir(name)
			rel, err := filepath.Rel(mod.Dir, dir)
			if known[dir] || err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) ||
				strings.HasSuffix(name, "_test.gp") || !strings.HasSuffix(name, ".gp") {
				continue
			}
			if pn := packageName(name, []byte(o.Text())); pn != "" && pn != "main" {
				known[dir] = true
				all = append(all, importable{name: pn, path: joinPath(mod.Path, filepath.ToSlash(rel)), dir: dir})
			}
		}
	}
	imported := make(map[string]bool)
	for _, spec := range q.file.AST.Imports {
		imported[importPath(spec)] = true
	}
	var list []importable
	for _, p := range all {
		if p.path == q.lp.ImportPath || p.dir == q.lp.Dir || imported[p.path] || !internalOK(q.lp.ImportPath, p.path) {
			continue
		}
		list = append(list, p)
	}
	slices.SortFunc(list, func(a, b importable) int { return strings.Compare(a.path, b.path) })
	return list
}

// internalOK reports whether the package from may import the package path
// by the rule of internal packages (load's internalAllowed): only the
// packages of the tree above the last internal element may.
func internalOK(from, path string) bool {
	var parent string
	switch {
	case strings.HasSuffix(path, "/internal"):
		parent = strings.TrimSuffix(path, "/internal")
	case strings.Contains(path, "/internal/"):
		parent = path[:strings.LastIndex(path, "/internal/")]
	case path == "internal" || strings.HasPrefix(path, "internal/"):
		return false // the standard library's own
	default:
		return true
	}
	return from == parent || strings.HasPrefix(from, parent+"/")
}

// importEdits returns the edits of the document of q that add the import
// of p, in the file as the user wrote it (pf, a fresh parse). The new
// import goes into the group of the gocpp imports that it belongs to (the
// standard packages first, then the others): right after the last import
// of its kind, else after the last gocpp import, else before the header
// and namespace imports; a file without imports gets an import declaration
// after its package clause (and the comments on its line). Then 11.7's
// organizeImports sorts and groups the imports as Organize Imports does,
// and the edits are the difference (computeEdits): so the line ends of the
// file stay ("\r\n"), and the header imports stay after the gocpp ones. A
// file with code on a line of an import declaration (package p; import
// "x") gets no edit, as organizeImports leaves it alone. isStd reports
// whether an import path is of a standard package.
func importEdits(ctx context.Context, q *query, pf *cache.ParsedFile, p importable, isStd func(path string) bool) []protocol.TextEdit {
	f, src := pf.File, pf.Src
	spec := strconv.Quote(p.path)
	if base := path.Base(p.path); p.name != base {
		spec = p.name + " " + spec
	}
	eol := "\n"
	if i := bytes.IndexByte(src, '\n'); i > 0 && src[i-1] == '\r' {
		eol = "\r\n"
	}
	var decls []*ast.GenDecl
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			break // the imports come first
		}
		decls = append(decls, gd)
	}
	at, end := 0, -1 // the edit replaces src[at:end] (nothing when end < 0)
	var text string
	if len(decls) == 0 {
		if f.Name == nil || !f.Name.End().IsValid() {
			return nil
		}
		at, text = lineEndAfter(pf, f.Name.End()), eol+eol+"import "+spec
	} else {
		if codeOnImportLines(decls, pf.Tok, scanFile(pf.Path, src)) {
			return nil
		}
		type imp struct {
			spec  *ast.ImportSpec
			block bool         // in an import ( ... ) declaration
			decl  *ast.GenDecl // its declaration
		}
		var gocpp, sameKind []imp
		var first *imp // the first header or namespace import
		for _, gd := range decls {
			for _, s := range gd.Specs {
				is := s.(*ast.ImportSpec)
				x := imp{is, gd.Lparen.IsValid(), gd}
				switch path := importPath(is); {
				case path == "C" || load.IsHeaderOrNamespace(path):
					if first == nil {
						first = &x
					}
				default:
					gocpp = append(gocpp, x)
					if isStd(path) == p.std {
						sameKind = append(sameKind, x)
					}
				}
			}
		}
		line := func(x imp) string {
			if x.block {
				return "\t" + spec
			}
			return "import " + spec
		}
		var after *imp
		switch {
		case len(sameKind) > 0:
			after = &sameKind[len(sameKind)-1]
		case len(gocpp) > 0:
			after = &gocpp[len(gocpp)-1]
		}
		switch {
		case after != nil && after.block && pf.Tok.Line(after.spec.End()) == pf.Tok.Line(after.decl.Rparen):
			// import ("fmt"; "os"): before the ).
			at, text = pf.Tok.Offset(after.spec.End()), eol+line(*after)
		case after != nil && after.block:
			at, text = lineEndAfter(pf, after.spec.End()), eol+line(*after)
		case after != nil:
			// import "fmt" becomes import ( "fmt"; "strings" ), so
			// that the two are one group; the comment on its line
			// stays with "fmt".
			at, end = pf.Tok.Offset(after.decl.Pos()), lineEndAfter(pf, after.spec.End())
			text = "import (" + eol + "\t" + string(src[pf.Tok.Offset(after.spec.Pos()):end]) + eol + "\t" + spec + eol + ")"
		default:
			// Before the header imports, and before their comment.
			var n ast.Node = first.spec
			if first.spec.Doc != nil {
				n = first.spec.Doc
			}
			for _, gd := range decls {
				if len(gd.Specs) > 0 && gd.Specs[0] == first.spec && !gd.Lparen.IsValid() {
					n = gd
					if gd.Doc != nil {
						n = gd.Doc
					}
				}
			}
			at = pf.Tok.Offset(pf.Tok.LineStart(pf.Tok.Line(n.Pos())))
			text = line(*first) + eol + eol
			if pf.Tok.Line(n.Pos()) == pf.Tok.Line(f.Name.Pos()) {
				return nil // the package clause on its line
			}
		}
	}
	if end < 0 {
		end = at
	}
	out := make([]byte, 0, len(src)+len(text))
	out = append(append(append(out, src[:at]...), text...), src[end:]...)
	fset := token.NewFileSet()
	nf, err := parser.ParseFile(fset, pf.Path, out, parser.ImportsOnly|parser.ParseComments)
	if err == nil {
		if res, err := organizeImports(pf.Path, out, nf, fset.File(nf.FileStart), nil); err == nil {
			out = res
		}
	}
	return computeEdits(ctx, q.mapper, string(out))
}

// lineEndAfter returns the offset of the line end of the line of pos in
// the file pf ("\r\n" or "\n"), after the comments that start on that
// line after pos (a /* … */ comment can end on a later line), or the end of
// the file.
func lineEndAfter(pf *cache.ParsedFile, pos token.Pos) int {
	off, line := pf.Tok.Offset(pos), pf.Tok.Line(pos)
	for _, cg := range pf.File.Comments {
		for _, c := range cg.List {
			if pf.Tok.Line(c.Pos()) == line && pf.Tok.Offset(c.Pos()) >= off {
				off, line = pf.Tok.Offset(c.End()), pf.Tok.Line(c.End())
			}
		}
	}
	src := pf.Src
	i := bytes.IndexByte(src[off:], '\n')
	if i < 0 {
		return len(src)
	}
	end := off + i
	if end > 0 && src[end-1] == '\r' {
		end--
	}
	return end
}

// importDetail is the end of the detail of a candidate that needs the
// import of p, like gopls's.
func importDetail(p *importable) string {
	return fmt.Sprintf("(from %q)", p.path)
}
