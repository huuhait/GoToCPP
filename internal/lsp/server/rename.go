package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/lsp/safe"
	"github.com/huuhait/gocpp/internal/syntax/gpast"
)

// textDocument/prepareRename and textDocument/rename (step 11.8): a safe
// rewrite of a name and everything that stands for it (refs.go says where
// the search looks and what stands for a name).
//
// Safe means: the edits are made only when the result still type-checks the
// way the user's code does. Rename checks, in this order:
//
//   - the new name is an identifier (not a keyword, not _), and for a
//     package-level name not init;
//   - the name can be renamed at all: not a predeclared name, not one of the
//     standard library or of a package outside the workspace, not a C/C++
//     name, not a method that implements a method of a package outside the
//     workspace (an interface of the standard library), and not an exported
//     name that other packages use when the new name is not exported;
//   - conflicts, from go/types' scopes, by the rules of gopls's rename: the
//     new name is not declared yet where the name is (also not as an import
//     name of a file, or a field or method of the same type, also promoted),
//     none of its uses would be captured by a nearer declaration of the new
//     name, and the new name's own existing uses in its scope would not be
//     captured by the renamed declaration;
//   - the result: the edits are applied to a scratch copy of the workspace
//     (cache.Snapshot.Edited) and every package that the edits touch is
//     checked again; an error that was not there before refuses the rename,
//     with its message. This catches what the scope rules above miss.
//
// A rename is refused with an error message (the client shows it), never
// half done. Files that the build constraints leave out and that mention the
// name get a warning (window/showMessage): their uses can't be told from other
// names (they are not checked), and they are not renamed.
//
// An import name changes its uses and the import: an import without a name
// gets one (import newname "path"). The name of a package (the package
// clause) and an import path are refused: a package rename also moves its
// folder and changes every importer, which is a different tool.

// skipEmbedderCheck is set by a test to see what the check of the result alone
// catches of what checkEmbedders refuses.
var skipEmbedderCheck bool

// skipConflictChecks is set by a test to see what the check of the result
// alone catches.
var skipConflictChecks bool

// refactor holds what the editing requests learned from initialize.
type refactor struct {
	// documentChanges: the client takes versioned document edits in a
	// WorkspaceEdit.
	documentChanges atomic.Bool
}

func (rf *refactor) initialize(client protocol.ClientCapabilities, caps *protocol.ServerCapabilities) {
	rf.documentChanges.Store(client.Workspace.WorkspaceEdit != nil && client.Workspace.WorkspaceEdit.DocumentChanges)
	caps.ReferencesProvider = &protocol.Or_ServerCapabilities_referencesProvider{Value: true}
	caps.DocumentHighlightProvider = &protocol.Or_ServerCapabilities_documentHighlightProvider{Value: true}
	// prepareRename tells the user at once that a name can't be renamed.
	prepare := client.TextDocument.Rename != nil && client.TextDocument.Rename.PrepareSupport
	if prepare {
		caps.RenameProvider = protocol.RenameOptions{PrepareProvider: true}
	} else {
		caps.RenameProvider = true
	}
}

// addRefactor adds the requests of the editing features.
func (s *Server) addRefactor() {
	s.requests["textDocument/references"] = method{run: s.references}
	s.requests["textDocument/documentHighlight"] = method{run: s.documentHighlight}
	s.requests["textDocument/prepareRename"] = method{run: s.prepareRename}
	// A change of any file makes the edits stale (their offsets are the
	// snapshot's), but the client checks versions of the documents it
	// applies the edit to (documentChanges) and the edit is made for the
	// snapshot that was current.
	s.requests["textDocument/rename"] = method{run: s.rename, contentModified: true}
}

// renameRefused is the error of a rename that is refused: the message is for
// the user.
func renameRefused(err error) error {
	var r *refusal
	if errors.As(err, &r) {
		return protocol.Errorf(protocol.RequestFailed, "%s", r.msg)
	}
	return err
}

// prepareRename answers textDocument/prepareRename: the range and name of the
// symbol, or an error that says why it can't be renamed. It makes the checks
// that don't need the search.
func (s *Server) prepareRename(ctx context.Context, e env, params json.RawMessage) (any, error) {
	var p protocol.PrepareRenameParams
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	q, err := newRefQuery(ctx, e.snapshot, p.TextDocument.URI, p.Position)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, protocol.Errorf(protocol.RequestFailed, "cannot rename here: the file is not part of a package that gocpp checks")
	}
	sym, err := q.symbolAt(ctx, true)
	if err != nil {
		return nil, renameRefused(err)
	}
	if sym == nil {
		return nil, protocol.Errorf(protocol.RequestFailed, "%s", q.noNameMessage())
	}
	if err := sym.renameable(); err != nil {
		return nil, renameRefused(err)
	}
	if err := s.refusedBySemantics(sym); err != nil {
		return nil, renameRefused(err)
	}
	return protocol.PrepareRenamePlaceholder{Range: sym.rng, Placeholder: sym.name}, nil
}

// noNameMessage is the answer of a rename where no name is: when the file has
// a syntax error, the code after it is not understood, which is the likelier
// reason.
func (q *query) noNameMessage() string {
	msg := "cannot rename here: there is no name at this position"
	if q.pkg == nil {
		return msg
	}
	line := 0
	for _, l := range []*diag.List{q.pkg.Program.Errors, q.pkg.Check.Errors} {
		for _, d := range l.Diagnostics() {
			if d.Severity == diag.Error && d.Pos.Filename == q.file.Path && d.Pos.Line > 0 && (line == 0 || d.Pos.Line < line) {
				line = d.Pos.Line
			}
		}
	}
	if line > 0 {
		msg += fmt.Sprintf(" (the file has an error at line %d, and the code after it may not be understood)", line)
	}
	return msg
}

// renameable returns a refusal when the symbol can be seen not to be
// renameable before a search.
func (sym *symbol) renameable() error {
	switch {
	case sym.why != "":
		return refuse("cannot rename %s: %s", sym.name, sym.why)
	case sym.std:
		return refuse("cannot rename %s: it is declared in the standard library", sym.name)
	case sym.name == "main" && sym.packageLevel() && sym.q.types != nil && sym.q.types.Name() == "main":
		if _, ok := sym.objs[0].(*types.Func); ok {
			return refuse("cannot rename main: it is the entry point of the program (the build would fail with \"function main is undeclared in the main package\")")
		}
	}
	q := sym.q
	if mod := q.lp.Module; mod != nil && sym.declPath != "" && sym.declPath != q.lp.ImportPath {
		if dep := q.lp.Imports[sym.declPath]; dep != nil && !dep.Std {
			rel, err := filepath.Rel(mod.Dir, dep.Dir)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return refuse("cannot rename %s: it is declared in %s, which is outside the workspace's module", sym.name, sym.declPath)
			}
		}
	}
	return nil
}

// refusedBySemantics returns a refusal for a name whose rename would change
// what the program does with no error from the build: an init function (it
// would no longer run) and the methods that the errors package calls by name.
// The legacy wide family leaves them as it always did.
func (s *Server) refusedBySemantics(sym *symbol) error {
	if len(sym.objs) != 1 {
		return nil
	}
	fn, ok := sym.objs[0].(*types.Func)
	if !ok {
		return nil
	}
	if sig, _ := fn.Type().(*types.Signature); sig != nil && sig.Recv() != nil && sym.q != nil {
		// A method that #Derive writes has the name of the generator in the #Derive line.
		if d, _ := sym.q.declOf(context.Background(), fn); d != nil && d.ident != nil &&
			derivedMethodName(d.pf.File, fn, typeName(sig.Recv().Type())) == d.ident {
			return refuse("cannot rename %s: #Derive writes it; the name is the generator's", sym.name)
		}
	}
	if s.wideFamily {
		return nil
	}
	if sym.name == "init" && sym.packageLevel() {
		return refuse("cannot rename init: the runtime calls it when the package is initialized; under another name it would not run, and the build would not say so")
	}
	if by := errorsProtocol(fn); by != "" {
		return refuse("cannot rename %s: %s, so under another name the program would behave differently with no error from the build", sym.name, by)
	}
	return nil
}

// isMustFunc reports whether the symbol is a function (or method) declared with a `!`: func f!(...).
func (sym *symbol) isMustFunc(ctx context.Context) bool {
	if len(sym.objs) != 1 {
		return false
	}
	fn, ok := sym.objs[0].(*types.Func)
	if !ok {
		return false
	}
	d, err := sym.q.declOf(ctx, fn)
	if err != nil || d == nil {
		return false
	}
	fd, ok := d.decl.(*ast.FuncDecl)
	return ok && fd.Name != nil && gpast.IsMust(fd.Name.Name)
}

// isMacro reports whether the symbol is a macro.
func (sym *symbol) isMacro() bool {
	return len(sym.objs) == 1 && sym.q.macroOf(sym.objs[0]) != nil
}

// validName reports whether name can be the new name of a symbol.
func validName(name string) error {
	switch {
	case name == "_":
		return refuse("cannot rename to _")
	case !token.IsIdentifier(name):
		return refuse("%q is not a valid name: it must be an identifier", name)
	}
	return nil
}

// rename answers textDocument/rename.
func (s *Server) rename(ctx context.Context, e env, params json.RawMessage) (any, error) {
	var p protocol.RenameParams
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	q, err := newRefQuery(ctx, e.snapshot, p.TextDocument.URI, p.Position)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, protocol.Errorf(protocol.RequestFailed, "cannot rename here: the file is not part of a package that gocpp checks")
	}
	sym, err := q.symbolAt(ctx, true)
	if err != nil {
		return nil, renameRefused(err)
	}
	if sym == nil {
		return nil, protocol.Errorf(protocol.RequestFailed, "%s", q.noNameMessage())
	}
	if err := sym.renameable(); err != nil {
		return nil, renameRefused(err)
	}
	if err := s.refusedBySemantics(sym); err != nil {
		return nil, renameRefused(err)
	}
	typed := p.NewName // (for the messages: what the user wrote)
	if name, bang := strings.CutSuffix(p.NewName, "!"); bang {
		// A must function is written f!: the ! is not part of its name, rename never changes it (step 11.13).
		// A client may send the new name with it; for a symbol that is not a must function it is an error.
		if !sym.isMustFunc(ctx) {
			return nil, renameRefused(refuse("%q is not a valid name: it must be an identifier. A ! after a name marks a must function or a must call, and rename leaves it as it is", p.NewName))
		}
		p.NewName = name
	}
	if name, hash := strings.CutPrefix(p.NewName, "#"); hash {
		// The same for a macro, written #Name at its uses.
		if !sym.isMacro() {
			return nil, renameRefused(refuse("%q is not a valid name: it must be an identifier. A # before a name marks a macro use, and rename leaves it as it is", p.NewName))
		}
		p.NewName = name
	}
	if err := validName(p.NewName); err != nil {
		if typed != p.NewName {
			err = refuse("%q is not a valid name: it must be an identifier", typed)
		}
		return nil, renameRefused(err)
	}
	if p.NewName == sym.name {
		return &protocol.WorkspaceEdit{}, nil
	}
	if p.NewName == "init" && sym.packageLevel() {
		return nil, renameRefused(refuse("cannot rename %s to init: init is a reserved function name", sym.name))
	}
	res, err := s.search(ctx, sym)
	if err != nil {
		return nil, err
	}
	rn := &renamer{s: s, e: e, sym: sym, res: res, newName: p.NewName, docs: newDocs(e.snapshot)}
	edits, err := rn.edits(ctx)
	if err != nil {
		return nil, renameRefused(err)
	}
	if err := rn.current(edits); err != nil {
		return nil, err
	}
	rn.warnExcluded(ctx)
	rn.warnNameAsString(ctx)
	rn.otherDeclarations()
	rn.newImplements()
	s.warnModules(e, "rename", q, sym, true)
	return rn.workspaceEdit(edits)
}

// current checks that the files to edit are still what the edits were made
// for: a client that takes plain changes (no document versions) would apply
// them to a changed text. It is a ContentModified error when one of them
// changed since the request's snapshot.
func (rn *renamer) current(files map[string]*fileEdits) error {
	cur, release := rn.s.session.Snapshot()
	defer release()
	for path, fe := range files {
		now, err := documentOfSnapshot(cur, protocol.URIFromPath(path))
		if err != nil || string(now.src) != string(fe.doc.src) {
			return protocol.Errorf(protocol.ContentModified, "%s changed while the rename was made", filepath.Base(path))
		}
	}
	return nil
}

// packageLevel reports whether the symbol is declared at the top level of a
// package.
func (sym *symbol) packageLevel() bool {
	if len(sym.objs) != 1 {
		return false
	}
	obj := sym.objs[0]
	if obj.Pkg() == nil {
		return false
	}
	if fn, ok := obj.(*types.Func); ok {
		if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
			return false
		}
	}
	if v, ok := obj.(*types.Var); ok && v.IsField() {
		return false
	}
	_, isPkgName := obj.(*types.PkgName)
	return !isPkgName && obj.Parent() == obj.Pkg().Scope()
}

// A renamer is one rename.
type renamer struct {
	s       *Server
	e       env
	sym     *symbol
	res     *result
	newName string
	docs    *docs

	// These facts are recorded by the existing conflict/test-declaration guards.
	bindingsChecked bool
	declFilesKnown  bool
	declHasTests    bool
}

// An fileEdits are the edits of one file, for the file's text in the
// snapshot.
type fileEdits struct {
	doc   *document
	edits []protocol.TextEdit
	text  string // the file after the edits
}

// edits checks the rename and returns the edits by file.
func (rn *renamer) edits(ctx context.Context) (map[string]*fileEdits, error) {
	res, sym := rn.res, rn.sym
	switch {
	case res.outside != "":
		return nil, refuse("cannot rename %s: it is related to %s by the interface it implements or is implemented by, which is declared outside the workspace", sym.name, res.outside)
	case len(res.incomplete) > 0:
		return nil, refuse("cannot rename %s: %s; fix the errors first", sym.name, res.incomplete[0])
	case res.testErr != nil:
		return nil, refuse("cannot rename %s: %v", sym.name, res.testErr)
	case len(res.hits) == 0:
		return nil, refuse("cannot rename %s: it has no declaration that gocpp can see", sym.name)
	}
	if err := rn.checkDynamic(); err != nil {
		return nil, err
	}
	if err := rn.checkEmbedders(); err != nil {
		return nil, err
	}
	if !rn.s.wideFamily {
		for _, h := range res.hits {
			if fn, ok := h.obj.(*types.Func); ok && h.def {
				if by := errorsProtocol(fn); by != "" {
					return nil, refuse("cannot rename %s: %s is in the same family, and %s, so under another name the program would behave differently with no error from the build", sym.name, objectName(fn), by)
				}
			}
		}
	}
	if err := rn.checkExported(); err != nil {
		return nil, err
	}
	if !skipConflictChecks {
		if err := rn.checkConflicts(); err != nil {
			return nil, err
		}
		if err := rn.checkTestDecls(ctx); err != nil {
			return nil, err
		}
		rn.bindingsChecked = true
	}
	files := make(map[string]*fileEdits)
	for _, h := range res.hits {
		doc, err := rn.docs.get(h.path)
		if err != nil {
			return nil, err
		}
		fe := files[h.path]
		if fe == nil {
			fe = &fileEdits{doc: doc}
			files[h.path] = fe
		}
		start, end := h.off, h.end
		text := rn.newName
		if h.spec != nil {
			end = start // an import without a name gets one: import name "path"
			text = rn.newName + " "
		}
		fe.edits = append(fe.edits, protocol.TextEdit{
			Range:   protocol.Range{Start: doc.mapper.Position(start), End: doc.mapper.Position(end)},
			NewText: text,
		})
	}
	// The text after the edits, for the check of the result.
	for path, fe := range files {
		offs := make([]offsetEdit, 0, len(fe.edits))
		for _, h := range res.hits {
			if h.path != path {
				continue
			}
			if h.spec != nil {
				offs = append(offs, offsetEdit{h.off, h.off, rn.newName + " "})
			} else {
				offs = append(offs, offsetEdit{h.off, h.end, rn.newName})
			}
		}
		fe.text = applyOffsets(string(fe.doc.src), offs)
	}
	if err := rn.checkResult(ctx, files); err != nil {
		return nil, err
	}
	return files, nil
}

type offsetEdit struct {
	start, end int
	text       string
}

// applyOffsets applies the edits (sorted, not overlapping) to src.
func applyOffsets(src string, edits []offsetEdit) string {
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var b strings.Builder
	last := 0
	for _, e := range edits {
		b.WriteString(src[last:e.start])
		b.WriteString(e.text)
		last = e.end
	}
	b.WriteString(src[last:])
	return b.String()
}

// checkTestDecls refuses a rename to a name that the _test.gp files of the
// package that declares the name declare at the top level (the internal tests:
// their package clause is the package's), or, for a method, as a method of the
// same type: those files are not part of the package's own check, so the
// scopes of the check don't know the name.
func (rn *renamer) checkTestDecls(ctx context.Context) error {
	sym := rn.sym
	if len(sym.objs) != 1 || sym.declDir == "" {
		return nil
	}
	obj := sym.objs[0]
	recv := ""
	switch o := obj.(type) {
	case *types.Func:
		if sig, ok := o.Type().(*types.Signature); ok && sig.Recv() != nil {
			if n, ok := recvBase(o).(*types.Named); ok {
				recv = n.Obj().Name()
			}
		}
	case *types.Var:
		if o.IsField() {
			return nil
		}
	}
	if recv == "" && !sym.packageLevel() {
		return nil
	}
	files, err := rn.e.snapshot.PackageFiles(&load.Package{Dir: sym.declDir})
	if err != nil {
		return nil
	}
	rn.declFilesKnown = true
	for _, name := range files {
		if !strings.HasSuffix(name, "_test.gp") || filepath.Dir(name) != sym.declDir {
			continue
		}
		rn.declHasTests = true
		pf, err := rn.e.snapshot.ParsePath(ctx, name, cache.ParseFull)
		if err != nil || pf.File == nil || pf.File.Name == nil || pf.File.Name.Name != sym.q.types.Name() {
			continue // an external test package has its own scope
		}
		if declaresInFile(pf.File, rn.newName, recv) {
			return refuse("cannot rename %s to %s: %s is declared in the test file %s", sym.name, rn.newName, rn.newName, filepath.Base(name))
		}
	}
	return nil
}

// declaresInFile reports whether f declares name at the top level (recv "")
// or as a method of the type recv.
func declaresInFile(f *ast.File, name, recv string) bool {
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Name.Name == name && recvName(d) == recv {
				return true
			}
		case *ast.GenDecl:
			if recv != "" {
				continue
			}
			for _, spec := range d.Specs {
				switch spec := spec.(type) {
				case *ast.ValueSpec:
					for _, id := range spec.Names {
						if id.Name == name {
							return true
						}
					}
				case *ast.TypeSpec:
					if spec.Name.Name == name {
						return true
					}
				case *ast.ImportSpec:
					if spec.Name != nil && spec.Name.Name == name {
						return true
					}
				}
			}
		}
	}
	return false
}

// dynamicLimit returns the most methods that the dynamic rule may add to a
// rename (a negative value: any number).
func (s *Server) dynamicLimit() int {
	if n := int(s.dynLimitOpt.Load()); n != 0 {
		return n
	}
	return DefaultDynamicLimit
}

// The settings of the server (the initializationOptions of the client, and
// workspace/didChangeConfiguration), as {"dynamicLimit": n} or
// {"gocpp": {"dynamicLimit": n}}: dynamicLimit is Options.DynamicLimit.

// didChangeConfiguration is the notification of new settings.
func (s *Server) didChangeConfiguration(_ context.Context, params json.RawMessage) error {
	var p struct {
		Settings json.RawMessage `json:"settings"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil // not ours to refuse: a client sends what it likes
	}
	s.applySettings(p.Settings)
	return nil
}

// applySettings takes the settings it knows from raw (anything else is ignored).
func (s *Server) applySettings(raw json.RawMessage) {
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		return
	}
	if g, ok := top["gocpp"]; ok {
		var inner map[string]json.RawMessage
		if json.Unmarshal(g, &inner) == nil {
			top = inner
		}
	}
	if v, ok := top["checkCacheMB"]; ok {
		var n float64
		if json.Unmarshal(v, &n) == nil && n >= 0 {
			s.pipeline.SetMaxCheckBytes(int64(n) << 20)
		}
	}
	if v, ok := top["dynamicLimit"]; ok {
		var n float64
		if json.Unmarshal(v, &n) == nil {
			s.dynLimitOpt.Store(int64(n))
		}
	}
}

// checkDynamic refuses a rename whose family grew by more than the limit only
// because a type assertion (a type switch, errors.As) could reach the types:
// it would change that many methods across the module on the strength of a
// guess about what flows where, and the user should say that they mean it.
func (rn *renamer) checkDynamic() error {
	limit := rn.s.dynamicLimit()
	dyn := rn.res.dynamic
	if limit < 0 || len(dyn) <= limit {
		return nil
	}
	var names []string
	for _, d := range dyn[:min(3, len(dyn))] {
		names = append(names, objectName(d.obj))
	}
	site := dyn[0].site
	where := "a type assertion"
	if site.IsValid() {
		where = fmt.Sprintf("%s at %s:%d", dyn[0].iface+" taken out of a value", filepath.Base(site.Filename), site.Line)
	}
	return refuse("cannot rename %s without asking: it would also rename %d methods of other types (%s, ...) because %s could reach them, and renaming only some would change what the program does without a compile-time error. Set gocpp.dynamicLimit to %d (or -1 for any number) in the editor's server settings (initializationOptions or workspace configuration; or start gocpp lsp with -dynamic-limit %d; in VS Code add \"-dynamic-limit\", \"%d\" to gocpp.lsp.args) to rename them all, or rename the interface's method by hand", rn.sym.name, len(dyn), strings.Join(names, ", "), where, len(dyn), len(dyn), len(dyn))
}

// checkExported refuses a rename of an exported name to an unexported one
// when another package uses it.
func (rn *renamer) checkExported() error {
	sym := rn.sym
	if !token.IsExported(sym.name) || token.IsExported(rn.newName) {
		return nil
	}
	for _, h := range rn.res.hits {
		if h.v.types == nil || h.obj == nil || h.obj.Pkg() == nil {
			continue
		}
		if h.v.types.Path() != h.obj.Pkg().Path() {
			return refuse("cannot rename %s to %s: %s uses it, and the new name is not exported", sym.name, rn.newName, filepath.Base(filepath.Dir(h.path)))
		}
	}
	return nil
}

// A conflict is why the new name can't be used at a place.
func (rn *renamer) conflict(h hit, format string, args ...any) error {
	p := h.position()
	return refuse("cannot rename %s to %s: %s (at %s:%d)", rn.sym.name, rn.newName, fmt.Sprintf(format, args...), filepath.Base(p.Filename), p.Line)
}

// checkConflicts looks for the conflicts that a rename of every hit to the new
// name makes, from the scopes of go/types.
func (rn *renamer) checkConflicts() error {
	selectorsOf := (*view).selectors
	owners := make(map[*view]*fieldOwners)
	// The declarations first: their conflicts say more than the uses'.
	hits := slices.Clone(rn.res.hits)
	slices.SortStableFunc(hits, func(a, b hit) int {
		if a.def != b.def {
			if a.def {
				return -1
			}
			return 1
		}
		return 0
	})
	for _, h := range hits {
		if h.obj == nil {
			// The declaring name of a type switch: its objects are the
			// clauses' (the other hits).
			continue
		}
		v := h.v
		if v.info == nil {
			continue
		}
		switch obj := h.obj.(type) {
		case *types.Var:
			if obj.IsField() {
				if err := rn.checkMember(h, owners, selectorsOf(v)); err != nil {
					return err
				}
				continue
			}
		case *types.Func:
			if sig, ok := obj.Type().(*types.Signature); ok && sig.Recv() != nil {
				if err := rn.checkMember(h, owners, selectorsOf(v)); err != nil {
					return err
				}
				continue
			}
		case *types.Label:
			if h.def {
				if err := rn.checkLabel(h); err != nil {
					return err
				}
			}
			continue
		}
		// A name with a scope.
		if h.spec != nil {
			if err := rn.checkDeclaration(h); err != nil {
				return err
			}
			continue
		}
		if h.def {
			if err := rn.checkDeclaration(h); err != nil {
				return err
			}
			if err := rn.checkCapture(h); err != nil {
				return err
			}
			continue
		}
		// A use. A qualified name (pkg.Name) is looked up in its package.
		if sel := selectorsOf(v)[h.id]; sel != nil {
			if x, ok := sel.X.(*ast.Ident); ok {
				if _, ok := v.info.Uses[x].(*types.PkgName); ok {
					continue // the declaring package's own scope was checked at the declaration
				}
			}
		}
		if err := rn.checkUse(h); err != nil {
			return err
		}
	}
	return nil
}

// encloses reports whether outer is inner or one of its parents.
func encloses(outer, inner *types.Scope) bool {
	for s := inner; s != nil; s = s.Parent() {
		if s == outer {
			return true
		}
	}
	return false
}

// checkDeclaration looks for another declaration of the new name where h
// declares the old one.
func (rn *renamer) checkDeclaration(h hit) error {
	v, obj := h.v, h.obj
	name := rn.newName
	scope := obj.Parent()
	if h.spec != nil || isPkgName(obj) {
		// An import name: the file's scope and the package's.
		if scope != nil {
			if other := scope.Lookup(name); other != nil && other != obj {
				return rn.conflictAt(h, other, "%s is already declared in this file", name)
			}
			if p := scope.Parent(); p != nil {
				if other := p.Lookup(name); other != nil {
					return rn.conflictAt(h, other, "%s is already declared in the package", name)
				}
			}
		}
		return rn.checkCapture(h)
	}
	if scope == nil {
		return nil
	}
	if other := scope.Lookup(name); other != nil && other != obj {
		return rn.conflictAt(h, other, "%s is already declared in this scope", name)
	}
	if v.types != nil && scope == v.types.Scope() {
		// A package-level name conflicts with the import names of the files.
		for _, f := range v.files {
			if fs := v.info.Scopes[f]; fs != nil {
				if other := fs.Lookup(name); other != nil {
					return rn.conflictAt(h, other, "%s is imported in %s", name, filepath.Base(v.fset.Position(f.Pos()).Filename))
				}
			}
		}
		if name == "main" && v.types.Name() == "main" {
			if _, isFunc := obj.(*types.Func); !isFunc {
				return rn.conflict(h, "main is the entry point of the program")
			}
		}
	}
	return nil
}

func isPkgName(obj types.Object) bool {
	_, ok := obj.(*types.PkgName)
	return ok
}

// conflictAt is conflict with the other object that the new name is.
func (rn *renamer) conflictAt(h hit, other types.Object, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	if other != nil && other.Pos().IsValid() {
		if p := h.v.fset.Position(other.Pos()); p.Filename != "" {
			msg += fmt.Sprintf(" (%s:%d)", filepath.Base(p.Filename), p.Line)
		}
	}
	return rn.conflict(h, "%s", msg)
}

// checkUse looks for a declaration of the new name that would capture the use
// h of the renamed name: one that lies between the use and the declaration.
func (rn *renamer) checkUse(h hit) error {
	v, obj := h.v, h.obj
	if v.types == nil || obj.Parent() == nil {
		return nil // an object without a scope (a label's use, a universe name)
	}
	inner := v.types.Scope().Innermost(h.id.Pos())
	if inner == nil {
		return nil
	}
	_, found := inner.LookupParent(rn.newName, h.id.Pos())
	if found == nil || found == obj {
		return nil
	}
	if found.Pkg() != nil && obj.Pkg() != nil && found.Pkg() != obj.Pkg() && found.Parent() != nil && found.Parent() == found.Pkg().Scope() {
		// A name of another package (a dot import): not ours to change.
		return rn.conflictAt(h, found, "%s, from a dot import, would be hidden", rn.newName)
	}
	if encloses(obj.Parent(), found.Parent()) {
		return rn.conflictAt(h, found, "the use of %s here would refer to another declaration of %s", rn.sym.name, rn.newName)
	}
	// found is declared outside the scope of obj: after the rename, obj is
	// the nearer declaration, so the use still refers to obj (checkCapture
	// looks at the uses of found).
	return nil
}

// checkCapture looks for the uses of the new name's existing declarations
// that the renamed declaration h would capture: a use in the scope of h's
// object that refers to a declaration outside of it.
func (rn *renamer) checkCapture(h hit) error {
	v, obj := h.v, h.obj
	if v.info == nil {
		return nil
	}
	scope := obj.Parent()
	if scope == nil {
		return nil
	}
	pkgLevel := v.types != nil && scope == v.types.Scope()
	for id, other := range v.info.Uses {
		if id.Name != rn.newName || other == nil || other == obj || v.synthetic[id] {
			continue
		}
		if !id.Pos().IsValid() || !pkgLevel && (id.Pos() < scope.Pos() || id.Pos() >= scope.End()) {
			continue
		}
		p := v.fset.Position(id.Pos())
		if _, ok := v.files[p.Filename]; !ok {
			continue
		}
		// An import name is seen in its file only.
		if isPkgName(obj) && !(id.Pos() >= scope.Pos() && id.Pos() < scope.End()) {
			continue
		}
		if other.Parent() != nil && encloses(other.Parent(), scope) && other.Parent() != scope {
			return rn.conflictAt(h, other, "it would hide the declaration of %s that is used at %s:%d", rn.newName, filepath.Base(p.Filename), p.Line)
		}
	}
	return nil
}

// fieldOwners finds the struct and the named type that declare a field.
type fieldOwners struct {
	structOf map[*types.Var]*types.Struct
	namedOf  map[*types.Struct]*types.Named
}

func (v *view) fieldOwners() *fieldOwners {
	if v.pkg == nil || v.test {
		return v.makeFieldOwners()
	}
	return v.pkg.Memo(fieldOwnersKey{}, func() any { return v.makeFieldOwners() }).(*fieldOwners)
}

type fieldOwnersKey struct{}

// selectors returns the selector expressions of the view's files by their
// selected name (the walk is done once for each checked package: every
// rename of a name that other packages use asks for it in each of them).
func (v *view) selectors() map[*ast.Ident]*ast.SelectorExpr {
	build := func() any {
		m := make(map[*ast.Ident]*ast.SelectorExpr)
		for _, f := range v.files {
			ast.Inspect(f, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok {
					m[sel.Sel] = sel
				}
				return true
			})
		}
		return m
	}
	if v.pkg == nil || v.test {
		return build().(map[*ast.Ident]*ast.SelectorExpr)
	}
	return v.pkg.Memo(selectorsKey{}, build).(map[*ast.Ident]*ast.SelectorExpr)
}

type selectorsKey struct{}

func (v *view) makeFieldOwners() *fieldOwners {
	o := &fieldOwners{structOf: make(map[*types.Var]*types.Struct), namedOf: make(map[*types.Struct]*types.Named)}
	for _, f := range v.files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.StructType:
				if st, ok := v.info.TypeOf(n).(*types.Struct); ok {
					for i := range st.NumFields() {
						o.structOf[st.Field(i)] = st
					}
				}
			case *ast.TypeSpec:
				if tn, ok := v.info.Defs[n.Name].(*types.TypeName); ok {
					if named, ok := tn.Type().(*types.Named); ok {
						if st, ok := named.Underlying().(*types.Struct); ok {
							o.namedOf[st] = named
						}
					}
				}
			}
			return true
		})
	}
	return o
}

// checkMember looks for a conflict of the new name with the fields and methods
// of the type of a field or method hit (declaration or selection).
func (rn *renamer) checkMember(h hit, owners map[*view]*fieldOwners, sels map[*ast.Ident]*ast.SelectorExpr) error {
	v, obj := h.v, h.obj
	name := rn.newName
	pkg := obj.Pkg()
	lookup := func(t types.Type) (types.Object, bool) {
		found, _, _ := types.LookupFieldOrMethod(t, true, pkg, name)
		return found, found != nil
	}
	if h.def {
		switch o := obj.(type) {
		case *types.Func:
			rt := recvBase(o)
			if rt == nil {
				return nil
			}
			if found, ok := lookup(rt); ok {
				return rn.conflictAt(h, found, "%s already has a field or method %s", types.TypeString(rt, types.RelativeTo(pkg)), name)
			}
		case *types.Var:
			ow := owners[v]
			if ow == nil {
				ow = v.fieldOwners()
				owners[v] = ow
			}
			st := ow.structOf[o]
			if st == nil {
				return nil
			}
			for i := range st.NumFields() {
				if f := st.Field(i); f != o && f.Name() == name {
					return rn.conflictAt(h, f, "the struct already has a field %s", name)
				}
			}
			if named := ow.namedOf[st]; named != nil {
				if found, ok := lookup(named); ok && found != types.Object(o) {
					return rn.conflictAt(h, found, "%s already has a field or method %s", named.Obj().Name(), name)
				}
			}
		}
		return nil
	}
	// A selection x.f: the new name must find nothing else in x's type.
	sel := sels[h.id]
	if sel == nil {
		return nil // a field of a composite literal: its declaration was checked
	}
	s := v.info.Selections[sel]
	if s == nil {
		return nil
	}
	if found, ok := lookup(s.Recv()); ok && origin(found) != origin(obj) {
		return rn.conflictAt(h, found, "%s.%s would refer to another %s", types.ExprString(sel.X), name, kindName(found))
	}
	return nil
}

func kindName(obj types.Object) string {
	switch o := obj.(type) {
	case *types.Func:
		return "method"
	case *types.Var:
		if o.IsField() {
			return "field"
		}
	}
	return "name"
}

// checkLabel looks for another label with the new name in the same function.
func (rn *renamer) checkLabel(h hit) error {
	v := h.v
	body := innermostFunc(v, h.id.Pos())
	for id, obj := range v.info.Defs {
		l, ok := obj.(*types.Label)
		if !ok || l.Name() != rn.newName {
			continue
		}
		if innermostFunc(v, id.Pos()) == body {
			return rn.conflictAt(h, l, "the function already has a label %s", rn.newName)
		}
	}
	return nil
}

// innermostFunc returns the function (declaration or literal) with the
// smallest body that contains pos, or nil.
func innermostFunc(v *view, pos token.Pos) ast.Node {
	var best ast.Node
	for _, f := range v.files {
		if pos < f.Pos() || pos >= f.End() {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if n == nil || pos < n.Pos() || pos >= n.End() {
				return false
			}
			switch n.(type) {
			case *ast.FuncDecl, *ast.FuncLit:
				best = n
			}
			return true
		})
	}
	return best
}

// checkResult applies the edits to a scratch copy of the workspace and
// compares the errors of the packages the edits touch, and of those that
// the search looked in, with what they had before: a new error refuses the
// rename.
func (rn *renamer) checkResult(ctx context.Context, files map[string]*fileEdits) error {
	texts := make(map[protocol.DocumentURI]string, len(files))
	dirs := make(map[string]bool)
	for path, fe := range files {
		texts[protocol.URIFromPath(path)] = fe.text
		if !strings.HasSuffix(path, "_test.gp") {
			dirs[filepath.Dir(path)] = true
		}
	}
	edited := maps.Clone(dirs)
	viewOf := make(map[string]*view, len(rn.res.views))
	for _, v := range rn.res.views {
		if !v.test && v.dir != "" {
			viewOf[v.dir] = v
		}
	}
	// The packages that the search looked at, and the importers of the edited
	// folders, whether or not the search found a use there: a method that is
	// renamed changes the method sets of its types, and an importer that passes a
	// value of one to an interface of the package (a tie that the search did not
	// see) would no longer compile. For a method the packages that cannot be
	// affected are left out (mayBreak): checking each of 1,500 packages of a module
	// again, when a rename changes 10 methods, was the cost of the rename.
	narrow := rn.canNarrowCheck()
	consider := func(dir string) {
		if v := viewOf[dir]; v == nil || edited[dir] || !narrow || rn.mayBreak(v) {
			dirs[dir] = true
		}
	}
	for dir := range viewOf {
		consider(dir)
	}
	if rn.res.importersOf != nil && (rn.sym.scope == scopeFamily || rn.sym.scope == scopeModule) {
		for _, dir := range slices.Sorted(maps.Keys(edited)) {
			for _, imp := range rn.res.importersOf(dir) {
				consider(imp)
			}
		}
	}
	if rn.localProvesResult(ctx, dirs) || rn.fieldProvesResult(ctx) ||
		rn.methodProvesResult(ctx, dirs, edited, viewOf) {
		return ctx.Err()
	}
	unchecked := rn.qualifiedOnly() // (renamefast.go)
	if rn.declaringScopeProvesResult(ctx, dirs, unchecked) {
		return ctx.Err()
	}
	after, release, err := rn.e.snapshot.Edited(texts)
	if err != nil {
		return err
	}
	defer release()
	var sorted []string
	for d := range dirs {
		if !unchecked[d] {
			sorted = append(sorted, d)
		}
	}
	slices.Sort(sorted)
	// The packages are checked together (an importer waits for the edited
	// package it imports, the others don't wait for anything), and compared in
	// order, so the refusal names the same package every time.
	type checked struct {
		pkg *cache.Package
		err error
	}
	results := make([]checked, len(sorted))
	var wg sync.WaitGroup
	var pc safe.Catcher
	sem := make(chan struct{}, 8)
	for i, dir := range sorted {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			pc.Run(func() { results[i].pkg, results[i].err = after.Package(ctx, dir) })
		}()
	}
	wg.Wait()
	pc.Rethrow()
	if err := ctx.Err(); err != nil {
		return err
	}
	for i, dir := range sorted {
		before, err := rn.e.snapshot.Package(ctx, dir)
		if err != nil {
			continue
		}
		now, err := results[i].pkg, results[i].err
		if err != nil {
			return refuse("cannot rename %s to %s: the package %s can't be checked afterwards: %v", rn.sym.name, rn.newName, dir, err)
		}
		if msg := newError(packageErrors(before), packageErrors(now)); msg != "" {
			return refuse("cannot rename %s to %s: it would make an error: %s", rn.sym.name, rn.newName, msg)
		}
	}
	// Every check with test files that the search loaded, whether or not a
	// test file is edited: a rename that leaves a type of a test file without
	// the method of its interface breaks no file that it touches.
	testDirs := make([]string, 0, len(rn.res.tests))
	for dir := range rn.res.tests {
		testDirs = append(testDirs, dir)
	}
	slices.Sort(testDirs)
	for _, dir := range testDirs {
		before := rn.res.tests[dir]
		now, err := after.CheckTests(ctx, dir)
		if err != nil {
			return refuse("cannot rename %s to %s: the test files of %s can't be checked afterwards: %v", rn.sym.name, rn.newName, dir, err)
		}
		if msg := newError(testErrors(before), testErrors(now)); msg != "" {
			return refuse("cannot rename %s to %s: it would make an error in the tests of %s: %s", rn.sym.name, rn.newName, filepath.Base(dir), msg)
		}
	}
	return nil
}

// canNarrowCheck reports whether the check of the result may leave out the
// packages that cannot be affected: for a rename of a method (the family's
// narrow rule), whose effect on a package is through its uses of the name and
// its ties to the interfaces that have it.
func (rn *renamer) canNarrowCheck() bool {
	if rn.s.wideFamily || len(rn.sym.objs) != 1 {
		return false
	}
	fn, ok := rn.sym.objs[0].(*types.Func)
	if !ok || recvBase(fn) == nil {
		return false
	}
	// The methods of an interface literal belong to every identical literal, in
	// other packages too (b.Show(i) takes the interface{ M() int } that a.Ifc()
	// returns): a package that never names the method can break.
	for _, h := range rn.res.hits {
		if f, ok := h.obj.(*types.Func); ok && h.def {
			if rt := recvBase(f); rt != nil {
				if _, named := rt.(*types.Named); !named {
					return false
				}
			}
		}
	}
	return true
}

// mayBreak reports whether the rename of a method can make an error in the
// package of the view v, which the search looked at and the edits do not
// touch. The search closes the family over every tie that the packages it
// looked at make, so such a package has no value of a type of the family
// passed to an interface that is not renamed with it, and no use of a member
// (it would be edited). It can still break:
//
//   - by using the new name, which can now be found where it was not;
//   - by a tie to an interface that has the new name or the old one (the
//     method set of the value's type changes by promotion: a struct that embeds
//     the type and another type with a method of the new name has an
//     ambiguous one, and stops implementing the interface that wants it: it
//     may be declared in another package, which passes its value on);
//
// (A type that only embeds a type of the family breaks nothing by itself: its
// promoted method set matters where a method is selected, which names the new
// name, or where it is passed to an interface, which is a tie.)
//
// A package that the search did not look at is checked, as the ties it may have
// are not known.
func (rn *renamer) mayBreak(v *view) bool {
	if v.info == nil {
		return true
	}
	if v.identIndex().byName[rn.newName] != nil {
		return true
	}
	for _, t := range v.ties() {
		if t.names[rn.sym.name] || t.names[rn.newName] {
			return true
		}
	}
	return false
}

// checkEmbedders refuses the rename of a method to a name that a type embedding the
// method's type already has, at any depth: the promoted method set of the
// embedder is made by the shallowest name, so the new name would hide the
// renamed method (a field or method of the embedder itself), make it ambiguous
// (another one at its depth), or hide the other one (a deeper one), and the
// embedder would lose a method or call another, with no error where the types
// still fit: W embeds a.A, whose M becomes N, and X, which embeds b.B, whose N is
// a level deeper; or W embeds a.A and b.B, whose N is at the same depth; or W has
// a field N.
func (rn *renamer) checkEmbedders() error {
	if skipEmbedderCheck || !rn.canNarrowCheck() {
		return nil
	}
	famKeys := make(map[objKey]bool, len(rn.res.keys))
	for _, k := range rn.res.keys {
		famKeys[k] = true
	}
	pkg := rn.sym.objs[0].Pkg()
	byName := func(p *types.Package) string { return p.Name() }
	for _, v := range rn.res.views {
		for _, rec := range v.embeds() {
			obj, _, _ := types.LookupFieldOrMethod(rec.outer, true, pkg, rn.sym.name)
			fn, ok := obj.(*types.Func)
			if !ok || !famKeys[keyOf(origin(fn))] {
				continue
			}
			nobj, nidx, _ := types.LookupFieldOrMethod(rec.outer, true, pkg, rn.newName)
			if nobj == nil && nidx == nil {
				continue
			}
			what := "an ambiguity"
			if nobj != nil {
				what = types.ObjectString(nobj, byName)
			}
			return refuse("cannot rename %s to %s: %s embeds %s, whose %s would become %s, and %s is already there (hidden by it, hiding it, or ambiguous with it): the type's method set would change with no error from the build", rn.sym.name, rn.newName,
				types.TypeString(rec.outer, byName), types.TypeString(recvBase(fn), byName), rn.sym.name, rn.newName, what)
		}
	}
	return nil
}

// packageErrors returns the error diagnostics of a checked package.
func packageErrors(pkg *cache.Package) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, l := range []*diag.List{pkg.Program.Errors, pkg.Check.Errors} {
		for _, d := range l.Diagnostics() {
			if d.Severity == diag.Error {
				out = append(out, d)
			}
		}
	}
	return out
}

// testErrors returns the error diagnostics of a check with tests.
func testErrors(tc *cache.TestCheck) []diag.Diagnostic {
	var out []diag.Diagnostic
	lists := []*diag.List{tc.Program.Errors}
	for _, root := range tc.Program.Roots {
		if cp := tc.Result.Package(root); cp != nil {
			lists = append(lists, cp.Errors)
		}
	}
	for _, l := range lists {
		for _, d := range l.Diagnostics() {
			if d.Severity == diag.Error {
				out = append(out, d)
			}
		}
	}
	return out
}

// newError returns the first error of after, with its place, whose message
// before does not have as often ("" if none): the messages are compared
// without their places, since an edit moves them.
func newError(before, after []diag.Diagnostic) string {
	count := make(map[string]int)
	for _, d := range before {
		count[d.Msg]++
	}
	for _, d := range after {
		if count[d.Msg] > 0 {
			count[d.Msg]--
			continue
		}
		return fmt.Sprintf("%s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Msg)
	}
	return ""
}

// warnNameAsString warns, for a method or a field, that a string literal of the
// module is its name: reflect finds a method (MethodByName) or a field
// (FieldByName), and the encoders a field, by a string, which a rename does not
// change, and the program then does something else with no error.
func (rn *renamer) warnNameAsString(ctx context.Context) {
	if rn.e.notify == nil || len(rn.sym.objs) != 1 || rn.s.wideFamily {
		return
	}
	switch o := rn.sym.objs[0].(type) {
	case *types.Func:
		if recvBase(o) == nil {
			return
		}
	case *types.Var:
		if !o.IsField() {
			return
		}
	default:
		return
	}
	where := rn.s.nameAsString(ctx, rn.e.snapshot, rn.res.moduleDirs, rn.sym.name)
	if where == "" {
		return
	}
	msg := fmt.Sprintf("gocpp rename: the string %q is at %s: reflect and the encoders find a method or a field by a name in a string, which a rename does not change. Look at it if it means %s.", rn.sym.name, where, rn.sym.name)
	if err := rn.e.notify("window/showMessage", &protocol.ShowMessageParams{Type: protocol.Warning, Message: msg}); err != nil {
		rn.e.log.Printf("window/showMessage: %v", err)
	}
}

// warnExcluded tells the user about the files of the packages that were
// searched that the build leaves out (build constraints) and that mention
// the name: they are not checked, so they were not renamed. It tells it as a
// window/showMessage warning, after the edit is made (the rename is not
// refused for it: gopls too renames for the build it checks).
func (rn *renamer) warnExcluded(ctx context.Context) {
	var warn []string
	seen := make(map[string]bool)
	for _, v := range rn.res.views {
		if v.test || seen[v.dir] {
			continue
		}
		seen[v.dir] = true
		files, err := rn.e.snapshot.PackageFiles(&load.Package{Dir: v.dir})
		if err != nil {
			continue
		}
		for _, name := range files {
			if strings.HasSuffix(name, "_test.gp") || filepath.Dir(name) != v.dir {
				continue
			}
			if _, ok := v.files[name]; ok {
				continue
			}
			src, err := rn.e.snapshot.ReadFile(name)
			if err == nil && bytes.Contains(src, []byte(rn.sym.name)) {
				warn = append(warn, filepath.Base(name))
			}
		}
	}
	if len(warn) == 0 || rn.e.notify == nil {
		return
	}
	slices.Sort(warn)
	msg := fmt.Sprintf("gocpp rename: %s is not renamed in %s: the build leaves the file out (build constraints), so gocpp doesn't check it. Look for the name there.", rn.sym.name, strings.Join(warn, ", "))
	if err := rn.e.notify("window/showMessage", &protocol.ShowMessageParams{Type: protocol.Warning, Message: msg}); err != nil {
		rn.e.log.Printf("window/showMessage: %v", err)
	}
}

// workspaceEdit returns the edits as a WorkspaceEdit in the form the client
// takes: versioned document edits, or plain changes.
func (rn *renamer) workspaceEdit(files map[string]*fileEdits) (*protocol.WorkspaceEdit, error) {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	we := &protocol.WorkspaceEdit{}
	if rn.s.refactor.documentChanges.Load() {
		for _, p := range paths {
			fe := files[p]
			te := &protocol.TextDocumentEdit{
				TextDocument: protocol.OptionalVersionedTextDocumentIdentifier{
					Version:                fe.doc.version,
					TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: locationURI(rn.e.snapshot, p)},
				},
			}
			for _, ed := range fe.edits {
				te.Edits = append(te.Edits, protocol.Or_TextDocumentEdit_edits_Elem{Value: ed})
			}
			we.DocumentChanges = append(we.DocumentChanges, protocol.DocumentChange{TextDocumentEdit: te})
		}
		return we, nil
	}
	we.Changes = make(map[protocol.DocumentURI][]protocol.TextEdit, len(paths))
	for _, p := range paths {
		we.Changes[locationURI(rn.e.snapshot, p)] = files[p].edits
	}
	return we, nil
}
