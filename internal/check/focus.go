package check

import (
	"go/ast"

	"github.com/huuhait/gocpp/internal/load"
)

// Focused checks (Config.Focus). The language server answers a keystroke in the body of one
// function from the check of the file that was typed in: the function bodies of the other files are
// left out, so go/types, the gates and the passes after it do their work for the declarations of
// the package and for the bodies of one file only. What the caller keeps of such a check is what
// belongs to that file (its diagnostics, its types.Info); the diagnostics of the bodies of the
// other files are missing, and so are "imported and not used" for the imports that only those
// bodies use, which are false errors there. The caller (internal/lsp/cache, focus.go) proves that
// the edit cannot change anything else before it asks for such a check.

// stripOtherBodies takes the bodies off the functions of the files of lp other than the file named
// focus, and reports whether it did: whether lp has the file. A func init and a generic function
// keep their bodies, because go/types reports them without one ("func init must have a body",
// "generic function is missing function body"), which the functions are not meant to have in a
// focused check. (Such an error would also come before the bodies of the focused file are checked,
// and go/types leaves out some errors that follow a first error: those with "invalid operand" or
// "invalid type" after the start of the message.) A method of a generic type has no type
// parameters of its own and loses its body like any other.
func stripOtherBodies(lp *load.Package, focus string) bool {
	found := false
	for _, f := range lp.Files {
		if f.AST != nil && f.Path == focus {
			found = true
		}
	}
	if !found {
		return false
	}
	for _, f := range lp.Files {
		if f.AST == nil || f.Path == focus {
			continue
		}
		for _, decl := range f.AST.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Body != nil && fn.Type.TypeParams == nil && !(fn.Recv == nil && fn.Name.Name == "init") {
				fn.Body = nil
			}
		}
	}
	return true
}
