package cache

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/memoize"
	"github.com/huuhait/gocpp/internal/syntax/gpast"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// Focused checks. A keystroke in the body of a function leaves the declarations of its package as
// they were, and the diagnostics of the other files with them. The language server answers such a
// keystroke from a check that leaves the function bodies of the other files out
// (check.Config.Focus): the diagnostics of the edited file come from that check, and the full
// check of the package follows when the typing pauses. The shortcut is taken only when everything
// that could make a diagnostic of the other files differ is proven the same; anything uncertain
// declines, and the caller does the full check.
//
// What is proven, against the last full check of the package (the base):
//
//   - the configuration, the module, the imports with their API hashes, the metadata of the imports
//     and the list of the files are the same, and exactly one file differs (focusBase.changed);
//   - the base is a plain package: nothing from C or C++ in it, no macros, `!` forms, #Derive
//     lines, directives or generics, no line mappings in any file, and no error of loading
//     (Package.focusEligible);
//   - the change lies inside the body of one function, between its braces, and the braces of the
//     new text are where the change puts them (proveBodyEdit); the new text has no syntax error and
//     its edit brought no C, namespace, macro or `!` form (focusRequest.accept);
//   - the initializer of no package-level variable uses a function or a variable of the package,
//     so that no initialization cycle goes through a function body (initializersUseLocals).
//
// A plain package has no C or C++ in it and no directive, so its check asks for no probe (the
// compiler, a header, pkg-config) and replaces no constant; the body of a function cannot change
// that.
//
// With those, go/types sees the same package-level declarations, and the same bodies in every file
// but the edited one: what it reports for the other files has not changed, and what it reports for
// the edited file does not depend on their bodies.

// errFocusDeclined is what a focused check returns when it finds that the shortcut does not apply.
var errFocusDeclined = errors.New("cache: no focused check")

// A focusBase is what a Package keeps for the focused checks that follow it.
type focusBase struct {
	env   Hash // the key of the check without the content of the files (focusEnv)
	files []focusFile
}

// A focusFile is a file of the package of a check: its name and the hash of its content.
type focusFile struct {
	path string
	hash Hash
}

// focusEnv returns the part of the key of a check that does not depend on the content of the files:
// what must be the same in two snapshots for the declarations of the package to be the only thing
// that its files can change.
func (p *Pipeline) focusEnv(h *handle, k *keyed) Hash {
	x := newHasher("focus")
	x.hash(p.cfgKey)
	x.str(h.id.dir)
	x.bool(h.id.std)
	x.str(h.importPath)
	if h.mod != nil {
		x.str(h.mod.path)
		x.hash(h.mod.hash)
	}
	x.int(int64(len(h.files)))
	for _, f := range h.files {
		x.str(f.path)
	}
	x.hash(h.meta.digest)
	x.int(int64(len(h.deps)))
	for i, dep := range h.deps {
		x.str(dep.id.dir)
		x.bool(dep.id.std)
		if s := k.deps[i]; s != nil {
			x.str(s.importPath)
			x.hash(s.apiHash)
		}
	}
	return x.sum()
}

// newFocusBase returns the focusBase of the check of h with the key k, or nil when the package is
// not one that focused checks apply to.
func (p *Pipeline) newFocusBase(h *handle, k *keyed) *focusBase {
	if h.id.std || h.cyclic || h.variant || k.variant {
		return nil
	}
	b := &focusBase{env: p.focusEnv(h, k), files: make([]focusFile, len(h.files))}
	for i, f := range h.files {
		b.files[i] = focusFile{f.path, f.hash}
	}
	return b
}

// changed reports which file of the package differs from the base, when the environment is the same
// and exactly one file differs.
func (b *focusBase) changed(env Hash, files []*fileHandle) (path string, ok bool) {
	if b.env != env || len(files) != len(b.files) {
		return "", false
	}
	for i, f := range files {
		if f.path != b.files[i].path || f.err != nil {
			return "", false
		}
		if f.hash == b.files[i].hash {
			continue
		}
		if path != "" {
			return "", false
		}
		path = f.path
	}
	return path, path != ""
}

// Focused returns the focused check of the package of dir in the snapshot, when the edit since
// base, the last full check of the package, can be proven to have changed one function body only
// (see above). The result is for the diagnostics of that file (Package.Focus says which); it is not
// cached, and the bodies of the other files are missing from it. It returns a nil package and no
// error when the shortcut does not apply: the caller asks for the package (Snapshot.Package).
func (s *Snapshot) Focused(ctx context.Context, dir string, base *Package) (*Package, error) {
	st, ok := s.state.(*state)
	if !ok || base == nil || base.focus == nil || !base.focusEligible() {
		return nil, nil
	}
	h, err := st.handle(ctx, pkgID{dir: filepath.Clean(dir)}, "", nil)
	if err != nil || h.cyclic || h.id.std {
		return nil, err
	}
	k, err := h.keyed.Get(ctx)
	if err != nil {
		return nil, err
	}
	path, ok := base.focus.changed(st.p.focusEnv(h, k), h.files)
	if !ok {
		return nil, nil
	}
	var newSrc []byte
	for _, f := range h.files {
		if f.path == path {
			newSrc = f.src
		}
	}
	edit, ok := findBodyEdit(base, path, newSrc)
	if !ok {
		return nil, nil
	}
	// A syntax error in the body is the common state while typing, and the load would find it after
	// half the work of a full check: the body alone is parsed first, which is nothing next to it.
	// (A body that parses alone but not in the file, or the other way, is left to the load.)
	if !bodyParses(edit.newBody(newSrc)) {
		return nil, nil
	}
	// One check for the requests of the snapshot that ask at the same time (the diagnostics and a
	// completion after the same keystroke).
	h.focusMu.Lock()
	if h.focusProm == nil || h.focusFor != base {
		h.focusFor = base
		h.focusProm = memoize.NewPromise(func(ctx context.Context) (*Package, error) {
			return st.p.check(ctx, h, k, &focusRequest{base: base, file: path, edit: edit})
		})
	}
	prom := h.focusProm
	h.focusMu.Unlock()
	pkg, err := prom.Get(ctx)
	if errors.Is(err, errFocusDeclined) {
		return nil, nil
	}
	return pkg, err
}

// Focus returns the name of the file that a focused package (Snapshot.Focused) is for, or "" for a
// package that is a full check.
func (pkg *Package) Focus() string { return pkg.focused }

// A focusRequest is a focused check that the pipeline runs.
type focusRequest struct {
	base *Package
	file string
	edit bodyEdit
}

// accept proves the edit of the file against the base, for the freshly loaded package. It reports
// false when the shortcut does not apply.
func (fr *focusRequest) accept(prog *load.Program, root *load.Package) bool {
	if len(prog.Packages) != 1 || !plainPackage(root) {
		return false
	}
	var newFile *load.File
	for _, f := range root.Files {
		if f.Path == fr.file {
			newFile = f
		}
	}
	if newFile == nil {
		return false
	}
	if len(gpast.Leftovers(newFile.AST)) != 0 {
		return false
	}
	return fr.edit.confirm(newFile, prog.Fset)
}

// plainPackage reports whether the loaded package is plain: loaded without an error, nothing from
// C or C++, no directive, no macro, no #Derive line. (A file that the loader rewrote for C or a
// namespace is not plain.) Whether a function body can change what the package reports then is
// left to the other proofs.
func plainPackage(root *load.Package) bool {
	if root.Incomplete || root.Std || root.Errors.HasErrors() {
		return false
	}
	foreign := len(root.CFiles) + len(root.CXXFiles)
	forms := len(root.Macros) + len(root.MacroUses) + len(root.Derives)
	if foreign+forms != 0 {
		return false
	}
	for _, f := range root.Files {
		if f.AST == nil || f.Info == nil || len(f.Directives) != 0 {
			return false
		}
		if len(f.Info.Includes) != 0 || len(f.Info.NamespaceRenames) != 0 {
			return false
		}
		if r := f.Rewrite; r != nil {
			if len(r.Uses) != 0 || len(r.Namespaces) != 0 || len(r.Renames) != 0 {
				return false
			}
			if r.ImportC != nil {
				return false
			}
		}
	}
	return true
}

// focusEligible reports whether focused checks may follow the check pkg (see above): it is a plain
// Go package without the forms that tie the function bodies to the declarations.
func (pkg *Package) focusEligible() bool {
	pkg.focusOnce.Do(func() { pkg.focusOK = pkg.computeFocusEligible() })
	return pkg.focusOK
}

func (pkg *Package) computeFocusEligible() bool {
	cp := pkg.Check
	if pkg.variant || pkg.Program == nil {
		return false
	}
	if len(pkg.Program.Roots) != 1 || len(pkg.Program.Packages) != 1 {
		return false
	}
	if cp == nil || cp.Types == nil || cp.Info == nil || cp.Focused {
		return false
	}
	if cp.Must != nil || len(cp.Consts) != 0 {
		return false
	}
	if pkg.summary == nil || pkg.summary.hasC || len(pkg.summary.records) != 0 {
		return false
	}
	root := pkg.Program.Roots[0]
	if cp.Load != root || !plainPackage(root) {
		return false
	}
	for _, f := range root.Files {
		if hasGenerics(f.AST) || len(gpast.Leftovers(f.AST)) != 0 {
			return false
		}
		if hasLineDirective(pkg.Program.Fset, f.AST) {
			return false
		}
	}
	return true
}

// hasLineDirective recognizes directives in parsed comments, not text in string literals. As in
// the scanner, a //line comment must start at the physical beginning of a line, whereas /*line
// can appear anywhere. Conservatively decline even a malformed directive or an identity mapping:
// the quick publish filters logical filenames, not the physical files that produced diagnostics.
// This runs once for each full base, through focusEligible's sync.Once, over its own files only.
func hasLineDirective(fset *token.FileSet, file *ast.File) bool {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "/*line ") {
				return true
			}
			if strings.HasPrefix(comment.Text, "//line ") &&
				fset.PositionFor(comment.Slash, false).Column == 1 {
				return true
			}
		}
	}
	return false
}

// hasGenerics reports whether the file declares a type parameter (go/types checks the
// instantiations of the whole package together, through its bodies).
func hasGenerics(f *ast.File) bool {
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Type.TypeParams != nil {
				return true
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok && ts.TypeParams != nil {
					return true
				}
			}
		}
	}
	return false
}

// initializersUseLocals reports whether the initializer of a package-level variable uses a
// function, a method or a variable of the package (also inside a function literal). Only such a use
// gives the initialization order an edge into the bodies of the package, where a cycle through a
// function of another file would be reported at a variable of the edited file.
func initializersUseLocals(root *load.Package, cp *check.Package) bool {
	pkg := cp.Types
	info := cp.Info
	local := func(obj types.Object) bool {
		if obj == nil || obj.Pkg() != pkg {
			return false
		}
		switch obj := obj.(type) {
		case *types.Func:
			return true
		case *types.Var:
			return obj.Parent() == pkg.Scope()
		}
		return false
	}
	found := false
	for _, f := range root.Files {
		if f.AST == nil {
			continue
		}
		for _, decl := range f.AST.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				for _, value := range spec.(*ast.ValueSpec).Values {
					ast.Inspect(value, func(n ast.Node) bool {
						switch n := n.(type) {
						case *ast.Ident:
							found = found || local(info.Uses[n])
						case *ast.SelectorExpr:
							if sel := info.Selections[n]; sel != nil {
								found = found || local(sel.Obj())
							}
						}
						return !found
					})
					if found {
						return true
					}
				}
			}
		}
	}
	return false
}

// focusDeclinedText are the words that make a change decline: text that changes how the positions
// of a file are read.
var focusDeclinedText = [][]byte{[]byte("//line"), []byte("/*line")}

// A bodyEdit is a change of the text of a file that lies inside the body of one function of the
// base: the declaration it is in, the region of the old text that the change replaces, and how much
// longer the new text is.
type bodyEdit struct {
	decl           int // the index of the function in the declarations of the file
	start, oldEnd  int // the replaced region of the old text, in bytes
	lbrace, rbrace int // the offsets of the braces of the body in the old text
	delta          int
	oldFile        *load.File
	oldTok         *token.File
	newLen, oldLen int
}

// findBodyEdit returns the change from the text of the file in the base to newSrc, when it lies
// between the braces of the body of one function of the base. It looks at the base only, so it
// is cheap and runs before the new text is loaded.
func findBodyEdit(base *Package, path string, newSrc []byte) (bodyEdit, bool) {
	oldSrc, ok := base.src[path]
	if !ok {
		return bodyEdit{}, false
	}
	var oldFile *load.File
	for _, f := range base.Program.Roots[0].Files {
		if f.Path == path {
			oldFile = f
		}
	}
	if oldFile == nil || oldFile.AST == nil {
		return bodyEdit{}, false
	}

	// The changed region: what is left when the common start and the common end of the texts go.
	start := 0
	for start < len(oldSrc) && start < len(newSrc) && oldSrc[start] == newSrc[start] {
		start++
	}
	oldEnd := len(oldSrc)
	newEnd := len(newSrc)
	for oldEnd > start && newEnd > start && oldSrc[oldEnd-1] == newSrc[newEnd-1] {
		oldEnd--
		newEnd--
	}
	if start == oldEnd && start == newEnd {
		return bodyEdit{}, false // the same text: the hashes would not differ
	}

	// A line directive in the change, or made by it with the text around it, moves the positions
	// of what follows.
	const margin = 8
	window := func(src []byte, to int) []byte {
		return src[max(start-margin, 0):min(to+margin, len(src))]
	}
	oldWindow := window(oldSrc, oldEnd)
	newWindow := window(newSrc, newEnd)
	for _, word := range focusDeclinedText {
		if bytes.Contains(oldWindow, word) || bytes.Contains(newWindow, word) {
			return bodyEdit{}, false
		}
	}

	oldTok := base.Program.Fset.File(oldFile.AST.Pos())
	if oldTok == nil || oldTok.Size() != len(oldSrc) {
		return bodyEdit{}, false
	}
	// The function whose body holds the whole change in the old text.
	for i, decl := range oldFile.AST.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		lbrace := oldTok.Offset(fn.Body.Lbrace)
		rbrace := oldTok.Offset(fn.Body.Rbrace)
		if lbrace < start && oldEnd <= rbrace {
			return bodyEdit{
				decl: i, start: start, oldEnd: oldEnd, lbrace: lbrace, rbrace: rbrace,
				delta: newEnd - oldEnd, oldFile: oldFile, oldTok: oldTok,
				oldLen: len(oldSrc), newLen: len(newSrc),
			}, true
		}
	}
	return bodyEdit{}, false
}

// newBody returns the text of the edited function's body in the new text, from its { to the } that
// the change puts where the old one was (nil when that is not inside the text).
func (e bodyEdit) newBody(newSrc []byte) []byte {
	end := e.rbrace + e.delta + 1
	if e.lbrace < 0 || end > len(newSrc) || end <= e.lbrace {
		return nil
	}
	return newSrc[e.lbrace:end]
}

// bodyParses reports whether body, the text of a function body, is a body without syntax errors.
func bodyParses(body []byte) bool {
	if body == nil {
		return false
	}
	src := append([]byte("package p\nfunc _() "), body...)
	mode := parser.Phase10Syntax | parser.SkipObjectResolution
	_, err := parser.ParseFile(token.NewFileSet(), "", src, mode)
	return err == nil
}

// confirm reports whether newFile, the loaded file of the new text (with fset its FileSet), has the
// same declarations as the old one, with the body of the edited function ending where the change
// puts its end: then everything outside the body is the same text, parsed the same way.
func (e bodyEdit) confirm(newFile *load.File, fset *token.FileSet) bool {
	if newFile.AST == nil {
		return false
	}
	newTok := fset.File(newFile.AST.Pos())
	if newTok == nil || newTok.Size() != e.newLen {
		return false
	}
	if len(e.oldFile.AST.Decls) != len(newFile.AST.Decls) {
		return false
	}
	fn := e.oldFile.AST.Decls[e.decl].(*ast.FuncDecl)
	newFn, ok := newFile.AST.Decls[e.decl].(*ast.FuncDecl)
	if !ok || newFn.Body == nil {
		return false
	}
	return newTok.Offset(newFn.Body.Lbrace) == e.oldTok.Offset(fn.Body.Lbrace) &&
		newTok.Offset(newFn.Body.Rbrace) == e.oldTok.Offset(fn.Body.Rbrace)+e.delta
}

// focusedPackage makes the Package of a focused check, or declines. The package has no summary of
// value (nothing imports it, and nothing is written for it), and it is not kept by any cache.
func (p *Pipeline) focusedPackage(
	h *handle, k *keyed, fr *focusRequest, prog *load.Program, root *load.Package, cp *check.Package,
	imported map[string]*types.Package,
) (*Package, error) {
	if cp == nil || cp.Load != root || !cp.Focused || cp.Types == nil || cp.Info == nil {
		return nil, errFocusDeclined
	}
	if initializersUseLocals(root, cp) {
		return nil, errFocusDeclined
	}
	src := make(map[string][]byte, len(h.files))
	for _, f := range h.files {
		src[f.path] = f.src
	}
	sum := &summary{importPath: root.ImportPath, load: root.Summary()}
	return &Package{
		Program: prog, Check: cp, summary: sum, src: src, imported: imported, focused: fr.file,
	}, nil
}
