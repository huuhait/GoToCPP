package check

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/huuhait/gocpp/internal/load"
)

// The hints for a header without a name that code uses by its file name.
//
// A header's default name is its file name, unless the name can't be used: <sys/select.h> would be
// select (a keyword: the loader asks for the name at the syntax error, load/keywordhint.go),
// "./main.h" next to func main would be main (declared by the package), and "./3d.h" would be 3d
// (no Go name). The last two reach go/types as ordinary names, which says something true and no
// help:
//
//	./main.gp:6:15: main.mn undefined (type func() has no field or method mn)
//	./main.gp:6:10: undefined: d3
//
// The names are the ones go/types could not resolve for the user's own reason, so the hint is
// added to the message, not put in its place (the user may well have meant the name that Go
// declares):
//
//	./main.gp:6:15: main.mn undefined (type func() has no field or method mn); "./main.h" has no
//	package name, because main is declared by the package: give it a name: import cmain "./main.h"
//	./main.gp:6:10: undefined: d3; "./3d.h" has no package name, because 3d is not a valid Go name:
//	give it a name: import d3 "./3d.h"
//
// The first is the error of a selector X.sel on a package-level name X that a header without a name
// would have (UsedByPackage). The second is the error of an undefined name X in X.sel, in a file with
// a header whose name is no Go name (NotAName): the user can't write 3d, so a name that Go does not
// know is a candidate, and the example uses it. It is not a candidate when it looks like a typo of a
// name that the file can see (nearName): strngs.Join with "strings" imported is the typo, not a
// header. A header that is imported with _ is not a candidate (load.Include.Hint is only set for the
// headers that have no name at all).

// A headerHinter gives the hint to the errors of one package.
type headerHinter struct {
	info *types.Info
	pkg  *types.Package

	// files are the files that have a header with a hint, by the file name.
	files map[string]*hintFile
}

// A hintFile is a file that has a header without a name.
type hintFile struct {
	ast *ast.File

	// packageLevel are the headers whose name the package declares (UsedByPackage), by that name;
	// notAName are the headers whose name is no Go name.
	packageLevel map[string][]*load.Include
	notAName     []*load.Include

	// bases maps the position of the selector X.sel's name sel and of X to the selector, filled
	// on the first error that needs it.
	bases map[token.Pos]*ast.SelectorExpr
}

// newHeaderHinter returns the hinter of the package lp, or nil if no header of it needs one.
func newHeaderHinter(fset *token.FileSet, lp *load.Package, info *types.Info, pkg *types.Package) *headerHinter {
	var h *headerHinter
	for _, f := range lp.Files {
		if f.Info == nil || f.AST == nil {
			continue
		}
		var hf *hintFile
		for _, inc := range f.Info.Includes {
			if inc.Hint == "" || inc.Name != "" {
				continue
			}
			if hf == nil {
				hf = &hintFile{ast: f.AST, packageLevel: make(map[string][]*load.Include)}
			}
			switch inc.Why {
			case load.UsedByPackage:
				hf.packageLevel[inc.WouldBe] = append(hf.packageLevel[inc.WouldBe], inc)
			case load.NotAName:
				hf.notAName = append(hf.notAName, inc)
			}
		}
		if hf == nil || len(hf.packageLevel) == 0 && len(hf.notAName) == 0 {
			continue
		}
		if h == nil {
			h = &headerHinter{info: info, pkg: pkg, files: make(map[string]*hintFile)}
		}
		h.files[fset.Position(f.AST.Package).Filename] = hf
	}
	return h
}

// hint returns the message of the error err of go/types with the hint, or false if the error
// has none. err.Msg is the first line of an error (the lines that continue it are not asked).
func (h *headerHinter) hint(fset *token.FileSet, err types.Error) (string, bool) {
	if h == nil {
		return "", false
	}
	hf := h.files[fset.Position(err.Pos).Filename]
	if hf == nil {
		return "", false
	}
	if hf.bases == nil {
		hf.bases = make(map[token.Pos]*ast.SelectorExpr)
		ast.Inspect(hf.ast, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if _, ok := sel.X.(*ast.Ident); ok {
					hf.bases[sel.X.Pos()] = sel
					hf.bases[sel.Sel.Pos()] = sel
				}
			}
			return true
		})
	}
	sel := hf.bases[err.Pos]
	if sel == nil {
		return "", false
	}
	id := sel.X.(*ast.Ident)
	switch {
	case err.Pos == sel.Sel.Pos() && strings.HasPrefix(err.Msg, id.Name+"."+sel.Sel.Name+" undefined"):
		incs := hf.packageLevel[id.Name]
		obj := h.info.Uses[id]
		if len(incs) == 0 || obj == nil || obj.Parent() != h.pkg.Scope() {
			return "", false
		}
		return err.Msg + "; " + incs[0].Hint, true
	case err.Pos == id.Pos() && err.Msg == "undefined: "+id.Name:
		if len(hf.notAName) == 0 || h.info.Uses[id] != nil || h.nearName(hf, id.Name) {
			return "", false
		}
		hints := make([]string, len(hf.notAName))
		for i, inc := range hf.notAName {
			hints[i] = inc.NoNameHint(id.Name)
		}
		return err.Msg + "; " + strings.Join(hints, "; "), true
	}
	return "", false
}

// nearName reports whether the undefined name is a typo of a name that the file can see: one that
// the file imports, the package declares or the language has: one letter away for a name of four
// or five letters, two for six or more. A name of up to three letters is never taken for a typo
// (d3 is one letter away from d, and it is the name to give 3d.h).
func (h *headerHinter) nearName(hf *hintFile, name string) bool {
	limit := 0
	switch {
	case len(name) >= 6:
		limit = 2
	case len(name) >= 4:
		limit = 1
	}
	scopes := []*types.Scope{types.Universe, h.pkg.Scope()}
	if fileScope := h.info.Scopes[hf.ast]; fileScope != nil {
		scopes = append(scopes, fileScope)
	}
	for _, scope := range scopes {
		for _, other := range scope.Names() {
			if limit > 0 && other != name && editDistance(name, other, limit) <= limit {
				return true
			}
		}
	}
	return false
}

// editDistance returns the number of single-letter changes (insert, delete, replace) from a to b,
// or more than limit when it is more.
func editDistance(a, b string, limit int) int {
	if d := len(a) - len(b); d > limit || -d > limit {
		return limit + 1
	}
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		row := make([]int, len(b)+1)
		row[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			row[j] = min(prev[j]+1, row[j-1]+1, prev[j-1]+cost)
		}
		prev = row
	}
	return prev[len(b)]
}
