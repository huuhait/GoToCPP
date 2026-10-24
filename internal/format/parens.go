package format

import (
	"bytes"
	"go/ast"
	"go/token"

	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// In the header of an if, for or switch statement, the parser reads a "{"
// after a type name as the start of the block, so a composite literal there
// needs parentheses: "for range (T{1, 2}) {". go/printer removes the
// parentheses around the expression of the header, but not when it finds a
// composite literal in it whose type is a name (stripParens and isTypeName
// in go/printer/nodes.go). It knows only plain names, though: T and pkg.T.
// For a generic type, T[int] or std.vector[int], it removes the
// parentheses, and the result doesn't parse ("expected 1 expression").
// gofmt has the same bug with Go generics.
//
// keepParens works around it without a copy of go/printer: in each such
// header, it gives the literal a type that go/printer recognizes, an
// *ast.Ident whose name is the printed type, "std.vector[int]". The
// printer prints the name as it is, so the text is the same, and it keeps
// the parentheses. The type is printed without line information, so it
// comes out on one line even if it was written on several:
// T[int,\n string] becomes T[int, string]. For a Go file without such a
// literal, keepParens changes nothing, so the result is still exactly what
// gofmt prints.
//
// One such literal in a header is enough. Two kinds of type can't go in
// an *ast.Ident, and if no literal of the header has another type,
// keepParens reports an error instead of printing code that doesn't parse:
//   - a type that still prints on more than one line: a struct or
//     interface type with two or more fields or methods in a type
//     argument, or a raw string with a newline in a struct tag;
//   - a type with a comment in it. The comment would move after the type,
//     and often the next gocpp fmt would move it again: a // comment goes
//     into the braces of the literal, and so does a /* */ comment that
//     ends a line. Only a /* */ comment in the middle of a line would stay
//     after the type, so all comments get the error: the rule is simpler.
//
// An error is simpler than keeping these types as they are written, and it
// is stable: the file stays as it is until it is changed by hand. Giving
// the type argument a name avoids the error.

// keepParens changes the headers of the if, for and switch statements of
// file in which go/printer would remove the parentheses that a composite
// literal needs (see above). It reports an error for a literal whose type
// can't be printed on one line, or has a comment in it.
func keepParens(fset *token.FileSet, file *ast.File) error {
	var headers [][]*ast.CompositeLit
	ast.Inspect(file, func(n ast.Node) bool {
		var x ast.Expr
		switch n := n.(type) {
		case *ast.IfStmt:
			x = n.Cond
		case *ast.SwitchStmt:
			x = n.Tag
		case *ast.ForStmt:
			x = n.Cond
		case *ast.RangeStmt:
			x = n.X
		}
		if lits := litsInParens(x); lits != nil {
			headers = append(headers, lits)
		}
		return true
	})

	var errs scanner.ErrorList
Headers:
	for _, lits := range headers {
		// One literal with a type name is enough for go/printer: the
		// first that can have one gets it.
		var firstErr string
		for _, lit := range lits {
			name, err := oneLine(fset, file, lit.Type)
			if err != "" {
				if firstErr == "" {
					firstErr = err
				}
				continue
			}
			lit.Type = &ast.Ident{NamePos: lit.Type.Pos(), Name: name}
			continue Headers
		}
		errs.Add(fset.Position(lits[0].Type.Pos()), "cannot keep the parentheses around this composite literal: "+firstErr)
	}
	return errs.Err()
}

// oneLine returns the type x of a composite literal printed on one line,
// or the reason why it can't be printed so.
func oneLine(fset *token.FileSet, file *ast.File, x ast.Expr) (name, err string) {
	if commentIn(file, x) {
		return "", "its type has a comment in it; move the comment out of the type"
	}
	// An empty FileSet has no line information: the printer puts the type
	// on one line where it can.
	var b bytes.Buffer
	if err := config.Fprint(&b, token.NewFileSet(), x); err != nil {
		// Fprint fails only if it can't write to b, or for a node it
		// doesn't know, and the parser makes neither.
		return "", err.Error()
	}
	if bytes.ContainsRune(b.Bytes(), '\n') {
		return "", "its type can't be written on one line; give the type a name"
	}
	return b.String(), ""
}

// commentIn reports whether file has a comment inside x.
func commentIn(file *ast.File, x ast.Expr) bool {
	for _, g := range file.Comments {
		if g.End() <= x.Pos() {
			continue
		}
		if g.Pos() >= x.End() {
			return false // file.Comments is sorted
		}
		return true // no token comes between the comments of a group, so g is in x
	}
	return false
}

// litsInParens returns the composite literals of x that need the
// parentheses around x, if go/printer would remove them, and nil if it
// would keep them or if they aren't needed. x is the expression of the
// header of an if, for or switch statement.
//
// It looks at x like stripParens in go/printer: through each pair of
// parentheses around x, for the composite literals that aren't inside more
// parentheses or inside another composite literal.
func litsInParens(x ast.Expr) []*ast.CompositeLit {
	for {
		px, ok := x.(*ast.ParenExpr)
		if !ok {
			return nil
		}
		var named bool
		var lits []*ast.CompositeLit
		ast.Inspect(px.X, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.ParenExpr:
				return false // the parentheses protect the literals in them
			case *ast.CompositeLit:
				if isTypeName(n.Type) {
					named = true
				} else if endsHeader(n.Type) {
					lits = append(lits, n)
				}
				return false
			}
			return true
		})
		if named {
			return nil // go/printer keeps these parentheses
		}
		if lits != nil {
			return lits
		}
		x = px.X // go/printer removes these parentheses, and looks at the next ones
	}
}

// isTypeName is the function of go/printer: whether stripParens keeps the
// parentheses around a composite literal of type x.
func isTypeName(x ast.Expr) bool {
	switch t := x.(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return isTypeName(t.X)
	}
	return false
}

// endsHeader reports whether, in the header of an if, for or switch
// statement, the parser reads a "{" after the type x as the start of the
// block. These are the cases of parsePrimaryExpr in
// internal/syntax/parser (and go/parser) that return at a "{" when
// exprLev < 0. A literal type ([]T{}, map[K]V{}, struct{}{}) is not one
// of them: the "{" after it starts the literal.
func endsHeader(x ast.Expr) bool {
	switch x.(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		return true
	}
	return false
}
