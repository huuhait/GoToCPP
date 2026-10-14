package load

import (
	"go/ast"
	"go/token"

	"github.com/huuhait/gocpp/internal/diag"
)

// reservedC is the error for a declaration named C. It has the form of
// go/types' "cannot declare init - must be func".
const reservedC = "cannot declare C - C is reserved for C's built-in types"

// CheckReservedC adds an error to errs for each declaration in the file f
// that is named C. In gocpp, C holds C's built-in types and the boundary
// helpers (C.int, C.size_t, C.CString) in every file, without an import,
// so no other name C may hide it.
//
// These declarations can't be named C: package-level funcs, vars, consts
// and types, local vars, consts and types, the names on the left of :=
// (also in range clauses, type switch guards and select cases), the
// receivers, parameters and results of funcs, methods and closures, and
// type parameters (also those of a generic receiver, func (l List[C]) m()).
// Names that are not in a scope with C are allowed: struct fields,
// methods, interface methods, labels, and the parameter names of a func
// type (func(C int)), which nothing can use. ClassifyImports checks the
// import names.
//
// The package clause can't name the package C either ("invalid package
// name C", like go/types' "invalid package name _"): a file that imports
// the package would get a second C.
func CheckReservedC(fset *token.FileSet, f *ast.File, errs *diag.List) {
	if f.Name != nil && f.Name.Name == "C" {
		errs.Add(diag.Diagnostic{
			Pos: fset.Position(f.Name.Pos()),
			End: fset.Position(f.Name.End()),
			Msg: "invalid package name C - C is reserved for C's built-in types",
		})
	}
	check := func(id *ast.Ident) {
		if id != nil && id.Name == "C" {
			errs.Add(diag.Diagnostic{Pos: fset.Position(id.Pos()), End: fset.Position(id.End()), Msg: reservedC})
		}
	}
	checkExpr := func(e ast.Expr) {
		if id, ok := e.(*ast.Ident); ok {
			check(id)
		}
	}
	checkFields := func(fields *ast.FieldList) {
		if fields == nil {
			return
		}
		for _, field := range fields.List {
			for _, id := range field.Names {
				check(id)
			}
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.GenDecl:
			return n.Tok != token.IMPORT
		case *ast.ValueSpec:
			for _, id := range n.Names {
				check(id)
			}
		case *ast.TypeSpec:
			check(n.Name)
			checkFields(n.TypeParams)
		case *ast.FuncDecl:
			if n.Recv == nil {
				check(n.Name) // a method name is allowed
			} else {
				checkFields(n.Recv)
				for _, field := range n.Recv.List {
					for _, e := range receiverTypeParams(field.Type) {
						checkExpr(e)
					}
				}
			}
			if n.Type != nil { // nil only in an AST that the parser didn't make
				checkFields(n.Type.TypeParams)
				checkFields(n.Type.Params)
				checkFields(n.Type.Results)
			}
		case *ast.FuncLit:
			if n.Type != nil {
				checkFields(n.Type.Params)
				checkFields(n.Type.Results)
			}
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE {
				for _, e := range n.Lhs {
					checkExpr(e)
				}
			}
		case *ast.RangeStmt:
			if n.Tok == token.DEFINE {
				checkExpr(n.Key)
				checkExpr(n.Value)
			}
		}
		return true
	})
}

// receiverTypeParams returns the type parameters that the receiver type t
// of a method declares, such as C in func (l *List[C]) m(): the indexes of
// the generic type, which are identifiers in valid code.
func receiverTypeParams(t ast.Expr) []ast.Expr {
	for {
		switch e := t.(type) {
		case *ast.ParenExpr:
			t = e.X
		case *ast.StarExpr:
			t = e.X
		case *ast.IndexExpr:
			return []ast.Expr{e.Index}
		case *ast.IndexListExpr:
			return e.Indices
		default:
			return nil
		}
	}
}
