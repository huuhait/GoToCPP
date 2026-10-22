package server

import (
	"fmt"
	"go/ast"
	"go/types"
	"regexp"
	"slices"
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

// withSourceDefinitions replaces the declarations of a function or variable that a header only
// declares by their definitions in the C or C++ sources of the package (step 12.13e), when a
// completed fact has them. The cache keeps only definitions that the compiler took for another
// declaration of the entity that the header declares (not a static function, not a function
// without a previous declaration). A function declaration is replaced by the definition with the
// same parameter types, which tell the overloads apart whatever typedef names the declarations
// spell (`size_t` and `unsigned long`); a variable by its definition (a name has one, and its
// type may be spelled another way: `extern int a[]`, `int a[3]`). A declaration without a
// definition stays.
func (q *query) withSourceDefinitions(u *load.CppUse, names []string, decls []probe.Declaration,
) []probe.Declaration {
	if !q.trustedView() || len(decls) == 0 {
		return decls
	}
	scope, name, ok := foreignScope(u, names)
	if !ok {
		return decls
	}
	defs, ready := q.snap.ForeignDefinitions(q.pkg, q.file, scope, name)
	if !ready {
		return decls
	}
	return pairDefinitions(decls, defs)
}

// pairDefinitions is the pairing of withSourceDefinitions.
func pairDefinitions(decls, defs []probe.Declaration) []probe.Declaration {
	out := make([]probe.Declaration, len(decls))
	copy(out, decls)
	for i, d := range decls {
		if d.Defined || d.Type == "" {
			continue
		}
		same := func(def probe.Declaration) bool {
			if def.Kind != d.Kind {
				return false
			}
			return d.Kind == "variable" || def.Params == d.Params
		}
		if at := slices.IndexFunc(defs, same); at >= 0 {
			out[i] = defs[at]
		}
	}
	return out
}

// sourcesNote says that some sources of the package were not read for definitions, for the hover
// of a function or a variable that no source was found to define: the definition may be in one of
// them.
func (q *query) sourcesNote(u *load.CppUse, names []string, decls []probe.Declaration) string {
	if !q.trustedView() || !slices.ContainsFunc(decls, func(d probe.Declaration) bool {
		return d.Kind == "function" || d.Kind == "variable"
	}) {
		return ""
	}
	left, limit := q.snap.ForeignSourcesLeftOut(q.pkg, q.file)
	if left == 0 {
		return ""
	}
	if scope, name, ok := foreignScope(u, names); ok {
		defs, ready := q.snap.ForeignDefinitions(q.pkg, q.file, scope, name)
		if ready && len(defs) > 0 {
			return ""
		}
	}
	return fmt.Sprintf("Go to definition may show this header: %d C/C++ source files of the "+
		"package were not read for definitions (the limit is %d files of up to 512 KiB).",
		left, limit)
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
	h.text = joinParas(declarationNote(name, decls), hresultNote(decls))
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

// hresultWord is the typedef that typed interop makes Go's error (ADR-137), as a word of the text of
// a declaration.
// It is the result of the head of a signature: only qualifiers before it, a name after it (not a
// pointer or a reference to one: HRESULT* f()).
var hresultWord = regexp.MustCompile(`^\s*(?:(?:static|virtual|inline|extern|constexpr|friend)\s+)*HRESULT\s+[A-Za-z_~]`)

// hresultNote says that a call of the function is an error in the program when a declaration
// writes HRESULT for the result: the words before the first parenthesis of a signature are the
// result, the qualifiers and the name, and a parameter of that type is after it.
func hresultNote(decls []probe.Declaration) string {
	for _, d := range decls {
		if d.Kind != "function" && d.Kind != "method" {
			continue
		}
		head, _, _ := strings.Cut(d.Signature, "(")
		if hresultWord.MatchString(head) {
			return "In typed mode a call returns an `error`: `nil` when the HRESULT is not negative, " +
				"`int32(call)` is the code itself."
		}
	}
	return ""
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
