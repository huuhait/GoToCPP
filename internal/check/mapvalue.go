package check

import (
	"errors"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// The value of an element of a map literal whose key is a C/C++ value (skipMapValue, skipped.go) is
// skipped by go/types: it can't assign the key, so it goes on with the next element. checkSkipped
// gives the value what go/types would have: it asks go/types itself, with a one-element map literal
// that holds the value and has the element type of the map,
//
//	map[int]ELEM{0: value}
//
// so that the value is checked against ELEM with Go's own words (cannot use "x" (untyped string
// constant) as int value in map literal), an elided literal ({1, 2}, {}, a nested one) has the type
// that ELEM gives it, and the types of the value are recorded as go/types records them. The
// literals inside the value that have a C/C++ key themselves are handled the same way.
//
// ELEM is not written as a type expression (the names of the type would be looked up again where the
// value is: an aliased import, a type of another file or package, an unexported or shadowed name, an
// anonymous struct with a tag, an interface with methods, each is a way to find another type or none).
// The element type itself is given a name that no program can write (elemName) in a scope of its own
// round the value, and the literal says that name: the type is the same type, and go/types prints it
// as it prints any type, so the messages are Go's. The scope is a child of the one the value is in,
// with the range of the value (CheckExpr finds its scope by the position). Nothing but the value is in
// its range, and the language server does not offer the name (completer.go).

// elidedCTypeMsg starts the error of an elided literal of a C/C++ type (flush drops it when go/types
// has an error of its own: an undefined type is invalid too, and no C/C++ type).
const elidedCTypeMsg = "a composite literal without its type ("

// elemName is the name of the element type in the scope of a checked value, and keyName that
// of the key type: names no program can write.
const (
	elemName = "gp·elem"
	keyName  = "gp·key"
)

// mapElem returns the element type of the map literal lit, or nil when go/types has none (the type
// of the literal is not recorded, or is not a map).
func mapElem(info *types.Info, lit ast.Expr) types.Type {
	tv, ok := info.Types[lit]
	if !ok || tv.Type == nil {
		return nil
	}
	if m, ok := tv.Type.Underlying().(*types.Map); ok {
		return m.Elem()
	}
	return nil
}

// isElidedCError reports whether msg is the error of an elided literal of a C/C++ type.
func isElidedCError(msg string) bool {
	return strings.HasPrefix(msg, elidedCTypeMsg)
}

// elidedCMessage is the error of the elided literal lit in the map literal mapLit, whose element type
// elem has a C/C++ part: the C/C++ type; or none ("") when the written element type has an instance of
// a generic type with a C/C++ type argument: that instance is the limit, which checkGenericCArgs
// reports at its place in the type (genericc.go).
func (m *mapValues) elidedCMessage(lit *ast.CompositeLit, mapLit ast.Expr) string {
	if ml, ok := mapLit.(*ast.CompositeLit); ok {
		if mt, ok := ml.Type.(*ast.MapType); ok {
			if inst := m.genericWithC(mt.Value); inst != nil {
				return ""
			}
		}
	}
	return elidedCTypeMsg + sourceExprString(lit) + ") for a C/C++ type next to a C/C++ map key is not supported yet: write the type"
}

// genericWithC returns the first instance of a generic type in the type expression e that has a C/C++
// type argument (go/types leaves such an instance invalid, so the types of the arguments tell), or nil.
func (m *mapValues) genericWithC(e ast.Expr) ast.Expr {
	var found ast.Expr
	ast.Inspect(e, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		var args []ast.Expr
		switch x := n.(type) {
		case *ast.IndexExpr:
			args = []ast.Expr{x.Index}
		case *ast.IndexListExpr:
			args = x.Indices
		default:
			return true
		}
		for _, a := range args {
			if tv, ok := m.info.Types[a]; ok && tv.IsType() && incomplete(tv.Type) {
				found = n.(ast.Expr)
				return false
			}
		}
		return true
	})
	return found
}

// A mapValues checks the values of the map literals that go/types skipped.
type mapValues struct {
	fset   *token.FileSet
	pkg    *types.Package
	info   *types.Info
	op     *opaquePass
	report func(token.Pos, error)

	errs   []pendingError     // the errors of the values checked since the last flush
	inside map[token.Pos]bool // the places of the variables that the nested values use
}

// A pendingError is an error of a value, kept until its root and the values in it are all checked.
type pendingError struct {
	pos token.Pos
	err error
}

// check checks root, the value of an element of the map literal mapLit, and reports whether it did.
func (m *mapValues) check(root, mapLit ast.Expr) bool {
	elem := mapElem(m.info, mapLit)
	if elem == nil {
		return false
	}
	pos := root.Pos()
	if incomplete(elem) {
		// The element type has a C/C++ part that go/types left invalid (a C/C++ struct, a slice of C/C++
		// numbers, an instance of a generic type with a C/C++ type argument): go/types can't tell what a
		// value is for it, so the value stays opaque, as before. An elided literal has no type to be written
		// with (the lowering can't write one), so it is refused here, in check and in the editor, as in the
		// build. (Not asked of a Go type that has a C/C++ field, a Go struct with one, or a slice, map or
		// pointer of it: go/types gives the literal that type, only the field is invalid, and the value is
		// checked below like any other.)
		if lit, ok := root.(*ast.CompositeLit); ok && lit.Type == nil {
			if msg := m.elidedCMessage(lit, mapLit); msg != "" {
				m.report(pos, types.Error{Fset: m.fset, Pos: pos, Msg: msg})
			}
			return true
		}
		return false
	}
	parent := innermost(m.pkg, pos)
	if parent == nil {
		parent = m.pkg.Scope()
	}
	scope := types.NewScope(parent, pos, root.End(), "map value")
	scope.Insert(types.NewTypeName(token.NoPos, m.pkg, elemName, elem))
	scope.Insert(types.NewTypeName(token.NoPos, m.pkg, keyName, types.Typ[types.Int]))
	lit := &ast.CompositeLit{
		Type: &ast.MapType{
			Map:   pos,
			Key:   &ast.Ident{NamePos: pos, Name: keyName},
			Value: &ast.Ident{NamePos: pos, Name: elemName},
		},
		Lbrace: pos,
		Elts: []ast.Expr{&ast.KeyValueExpr{
			Key:   &ast.BasicLit{ValuePos: pos, Kind: token.INT, Value: "0"},
			Colon: pos,
			Value: root,
		}},
		Rbrace: root.End(),
	}
	m.info.Scopes[lit] = scope // (the scope tree and Info.Scopes agree: checkScopeTree)
	if err := types.CheckExpr(m.fset, m.pkg, pos, lit, m.info); err != nil {
		m.errs = append(m.errs, pendingError{pos, err})
	}
	return true
}

// flush reports the errors of the values checked since the last flush. go/types ends a function
// literal that is in the root before the values in it that it skipped are checked, and says "declared
// and not used" for a variable that only such a value uses: the error is false once that value is checked.
func (m *mapValues) flush() {
	for _, e := range m.errs {
		var terr types.Error
		if errors.As(e.err, &terr) && strings.HasPrefix(terr.Msg, "declared and not used: ") && m.inside[terr.Pos] {
			continue
		}
		m.report(e.pos, e.err)
	}
	m.errs, m.inside = nil, nil
}

// nested checks the values of the map literals inside root that have a C/C++ key as well: go/types
// skipped those too, and they were not in the list of the roots, because the literal that holds them
// had no type before check gave it one. One walk does all the levels (a literal is visited after the
// check of the value it is in, which gave it its type), each value once.
func (m *mapValues) nested(root ast.Expr) {
	ast.Inspect(root, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if mapElem(m.info, lit) == nil {
			return true
		}
		for _, elt := range lit.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok && m.op.opaqueExpr(kv.Key) {
				if m.check(kv.Value, lit) {
					m.noteUses(kv.Value)
				}
			}
		}
		return true
	})
}

// noteUses notes the places of the variables that the value uses (that go/types didn't see used).
func (m *mapValues) noteUses(value ast.Expr) {
	ast.Inspect(value, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if obj := m.info.Uses[id]; obj != nil {
				if m.inside == nil {
					m.inside = make(map[token.Pos]bool)
				}
				m.inside[obj.Pos()] = true
			}
		}
		return true
	})
}
