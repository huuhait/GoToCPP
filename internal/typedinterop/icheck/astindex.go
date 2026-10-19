package icheck

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"github.com/huuhait/gocpp/internal/check"
)

// An astInfo is what the checker needs to know about the node of a use of a
// reference, beyond what the collector's Site says.
type astInfo struct {
	node ast.Expr

	// inDeferGo says that the use is inside a defer or go statement: the
	// generated code calls the function from a lambda with the arguments stored
	// in the frame, so a probe is not an exact mirror there.
	inDeferGo bool

	// inGeneric says that the use is inside a generic function (or a method of
	// a generic type): it is lowered once for each instance that is used, so
	// code that is never instantiated is never compiled.
	inGeneric bool

	// lhs says that the expression is assigned to (the left side of an
	// assignment, the operand of ++ or --).
	lhs bool

	// callee is the text of the function of the call that the expression is an
	// argument of ("" otherwise), argIndex its position.
	callee   string
	argIndex int
}

type posKey struct {
	file      string
	line, col int
}

// An astIndex finds the AST nodes of the uses that the collector listed.
type astIndex struct {
	byPos map[posKey][]*astInfo
	dir   string
}

// buildIndex indexes the expressions of the files of p.
func buildIndex(p *check.Package, fset *token.FileSet) *astIndex {
	ix := &astIndex{byPos: make(map[posKey][]*astInfo), dir: p.Load.Dir}
	for _, f := range p.Load.Files {
		if f.AST == nil {
			continue
		}
		ix.addFile(f.AST, fset)
	}
	return ix
}

func (ix *astIndex) addFile(file *ast.File, fset *token.FileSet) {
	var stack []ast.Node
	deferGo, generic := 0, 0
	// lhsNodes are the expressions assigned to, found when their statement is
	// entered.
	lhs := map[ast.Node]bool{}
	args := map[ast.Node]*astInfo{}
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			switch top := top.(type) {
			case *ast.DeferStmt, *ast.GoStmt:
				deferGo--
			case *ast.FuncDecl:
				if isGenericFunc(top) {
					generic--
				}
			}
			return true
		}
		stack = append(stack, n)
		switch n := n.(type) {
		case *ast.DeferStmt, *ast.GoStmt:
			deferGo++
		case *ast.FuncDecl:
			if isGenericFunc(n) {
				generic++
			}
		case *ast.AssignStmt:
			if n.Tok != token.DEFINE && len(n.Lhs) == 1 && len(n.Rhs) == 1 {
				for _, l := range n.Lhs {
					lhs[l] = true
				}
			}
		case *ast.IncDecStmt:
			lhs[n.X] = true
		case *ast.CallExpr:
			callee := types.ExprString(n.Fun)
			for i, a := range n.Args {
				args[a] = &astInfo{callee: callee, argIndex: i}
			}
		}
		e, ok := n.(ast.Expr)
		if !ok {
			return true
		}
		info := &astInfo{node: e, inDeferGo: deferGo > 0, inGeneric: generic > 0, lhs: lhs[e]}
		if a := args[e]; a != nil {
			info.callee, info.argIndex = a.callee, a.argIndex
		}
		pos := fset.Position(e.Pos())
		k := posKey{filepath.Clean(pos.Filename), pos.Line, pos.Column}
		ix.byPos[k] = append(ix.byPos[k], info)
		return true
	})
}

// isGenericFunc reports whether the function has type parameters of its own or
// is a method of a generic type.
func isGenericFunc(fd *ast.FuncDecl) bool {
	if fd.Type.TypeParams != nil && len(fd.Type.TypeParams.List) > 0 {
		return true
	}
	if fd.Recv != nil && len(fd.Recv.List) > 0 {
		t := fd.Recv.List[0].Type
		if s, ok := t.(*ast.StarExpr); ok {
			t = s.X
		}
		switch t.(type) {
		case *ast.IndexExpr, *ast.IndexListExpr:
			return true
		}
	}
	return false
}

// lookup returns the node of the use at the place, the one whose text is text
// when several start there (a call, its function and the first name of it), else
// the outermost one. nil when the place is not known.
func (ix *astIndex) lookup(file string, line, col int, text string) *astInfo {
	if !filepath.IsAbs(file) {
		file = filepath.Join(ix.dir, file)
	}
	infos := ix.byPos[posKey{filepath.Clean(file), line, col}]
	var best *astInfo
	for _, in := range infos {
		if best == nil || in.node.End() > best.node.End() {
			best = in
		}
		if exprText(in.node) == text {
			return in
		}
	}
	return best
}

// exprText is the text of an expression as the collector writes the text of a
// site (Site.Text): Go's, with check's own names put back.
func exprText(e ast.Expr) string {
	return strings.NewReplacer("gp_cgoh.Cstr", "C.cstr", "gp_cgoh.Free", "C.free", "gp_cgoh.", "C.").Replace(types.ExprString(e))
}
