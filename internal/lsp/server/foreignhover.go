package server

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// Hover and go-to-definition for named foreign API symbols (step 12.13c2): raylib.InitWindow,
// std.string, sqlite3.Open, and the members of a proven receiver (win.size()). The answer is
// the compiler's declaration in the header, from facts that a background worker completed for
// this snapshot (cache.Snapshot.ForeignDeclarations, ForeignMemberDeclarations). A request
// never probes and never waits: a name without a completed fact, a repaired or stale view and
// a set of overloads that is not complete are answered as before.

// trustedView reports whether the query is on the checked text of the request itself: a
// repaired or moved view must not take the facts of the original bindings and names.
func (q *query) trustedView() bool {
	return q.pkg != nil && !q.pkg.IsVariant() && q.blanked == nil && q.shift == nil && !q.isFixed
}

// foreignScope returns the namespace and name that the use and the names after it stand for:
// raylib.InitWindow is ("", InitWindow), std.chrono.seconds is ("std::chrono", seconds).
func foreignScope(u *load.CppUse, names []string) (scope, name string, ok bool) {
	if len(names) == 0 {
		return "", "", false
	}
	switch u.Kind {
	case load.HeaderUse:
	case load.NamespaceUse:
		scope = strings.TrimPrefix(u.Namespace, "::")
	default:
		return "", "", false
	}
	if len(names) > 1 {
		parts := names[:len(names)-1]
		if scope != "" {
			parts = append([]string{scope}, parts...)
		}
		scope = strings.Join(parts, "::")
	}
	return scope, names[len(names)-1], true
}

// foreignNameDecls returns the completed declarations of the C/C++ name at the place of q.
func (q *query) foreignNameDecls(u *load.CppUse, names []string) []probe.Declaration {
	if !q.trustedView() {
		return nil
	}
	scope, name, ok := foreignScope(u, names)
	if !ok {
		return nil
	}
	decls, ready := q.snap.ForeignDeclarations(q.pkg, q.file, scope, name)
	if !ready {
		return nil
	}
	return decls
}

// foreignMemberDecls returns the completed declarations of the member that sel selects on an
// original variable binding, or nil.
func (q *query) foreignMemberDecls(sel *ast.SelectorExpr) []probe.Declaration {
	if !q.trustedView() {
		return nil
	}
	id, ok := ast.Unparen(sel.X).(*ast.Ident)
	if !ok {
		return nil
	}
	variable, ok := q.info.Uses[id].(*types.Var)
	if !ok || variable.IsField() {
		return nil
	}
	decls, ready := q.snap.ForeignMemberDeclarations(q.pkg, variable, sel.Sel.Name)
	if !ready {
		return nil
	}
	return decls
}

// memberSelectorAt returns the selector whose member name is at the place of q, on a value that
// the checker took as opaque, or nil.
func (q *query) memberSelectorAt(id *ast.Ident, path []ast.Node) *ast.SelectorExpr {
	if len(path) == 0 {
		return nil
	}
	sel, ok := path[len(path)-1].(*ast.SelectorExpr)
	if !ok || sel.Sel != id || !q.opaque.IsOpaque(sel.X) {
		return nil
	}
	return sel
}

// applyDeclarations makes h the hover of the declarations: the C++ declarations as the
// signature, where the header declares them, and the comment of the header.
func applyDeclarations(h *hoverResult, name string, decls []probe.Declaration) {
	signatures := make([]string, 0, len(decls))
	for _, d := range decls {
		signatures = append(signatures, d.Signature)
	}
	h.signature = strings.Join(signatures, "\n")
	h.signatureLanguage = "cpp"
	h.text = declarationNote(name, decls)
	h.doc = ""
	if doc := declarationDocs(decls); doc != "" {
		h.text = joinParas(h.text, doc)
	}
}

// declarationNote says what the declarations are and where they are.
func declarationNote(name string, decls []probe.Declaration) string {
	places := make([]string, 0, len(decls))
	for _, d := range decls {
		places = append(places, fmt.Sprintf("`%s:%d`", d.File, d.Line+1))
	}
	if len(decls) == 1 {
		return fmt.Sprintf("C/C++ %s `%s`, declared at %s.", decls[0].Kind, name, places[0])
	}
	// Only functions of one kind are overloads: a struct and a function of one name (stat) are not.
	what := "declarations"
	if overloaded(decls) {
		what = "declarations (overloads)"
	}
	return fmt.Sprintf("C/C++ `%s` has %d %s, at %s.", name, len(decls), what, strings.Join(places, ", "))
}

// overloaded reports whether the declarations are all functions (or function templates).
func overloaded(decls []probe.Declaration) bool {
	for _, d := range decls {
		switch d.Kind {
		case "function", "method", "constructor", "template":
		default:
			return false
		}
	}
	return true
}

// declarationDocs joins the comments of the header. One comment is the comment of every
// declaration that has it; different comments are told apart by the first line of their
// declaration.
func declarationDocs(decls []probe.Declaration) string {
	var docs []string
	same := true
	for _, d := range decls {
		if d.Doc == "" {
			continue
		}
		if len(docs) > 0 && d.Doc != docs[0] {
			same = false
		}
		docs = append(docs, d.Doc)
	}
	if len(docs) == 0 {
		return ""
	}
	if same && len(docs) == len(decls) {
		return docs[0]
	}
	var parts []string
	for _, d := range decls {
		if d.Doc == "" {
			continue
		}
		first, _, _ := strings.Cut(d.Signature, "\n")
		parts = append(parts, "`"+first+"`\n"+d.Doc)
	}
	return strings.Join(parts, "\n\n")
}

// declarationLocations returns the places of the declarations in their headers.
func (q *query) declarationLocations(decls []probe.Declaration) []protocol.Location {
	locs := make([]protocol.Location, 0, len(decls))
	for _, d := range decls {
		locs = append(locs, protocol.Location{
			URI: locationURI(q.snap, d.File),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(d.Line), Character: uint32(d.Col)},
				End:   protocol.Position{Line: uint32(d.Line), Character: uint32(d.Col + d.Length)},
			},
		})
	}
	return locs
}
