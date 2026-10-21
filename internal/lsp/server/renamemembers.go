package server

import (
	"context"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// The proofs of the result of the rename of a field and of a method of a
// named type. Like the proofs of renamefast.go they apply after the conflict
// guards (checkConflicts: the members of the type, the selections) and the
// closure of the search succeeded, and they leave the scratch check for every
// form they do not know.

// sameExportedness reports whether the new name is exported exactly when the
// old one is: the packages that see the member are the same.
func (rn *renamer) sameExportedness() bool {
	return token.IsExported(rn.sym.name) == token.IsExported(rn.newName)
}

// searchedViewsAreOrdinary reports whether every view of the search is of a
// checked, error-free package without extensions (normalBindingFacts), and
// there is no test check: the facts of the proofs describe those only.
func (rn *renamer) searchedViewsAreOrdinary() bool {
	if len(rn.res.tests) != 0 || len(rn.res.views) == 0 {
		return false
	}
	for _, view := range rn.res.views {
		if !view.normalBindingFacts() {
			return false
		}
	}
	return true
}

// fieldProvesResult proves the result of the rename of a field of a named,
// nongeneric struct type that is declared in a package, to a name of the same
// exportedness. After the guards (the new name is not another field or method of
// the type, at any depth; every selection finds the field under the new name, and
// no other member) the rename can make an error only where the field's name is
// part of something other than a lookup:
//
//   - a type that embeds the struct's type (the promoted name could hide or be
//     hidden by a member of the embedder that the guards did not look at, and
//     change the embedder's method set): none of the packages that the search
//     looked at embeds it, by any name (an alias too);
//   - the identity of struct types, which the field names are part of. Two
//     named types are never assignable, so only a conversion between them (or
//     between pointers to them) depends on it: it stops to compile when only
//     one of the types has the new name. A struct type that is written without
//     a name (a literal, the target of an alias, a constraint) is assignable
//     to and from a named type of the same underlying type, and two of them
//     are the same type, a duplicate case of a switch. So no package converts
//     a value of a struct type of the names of the struct or of the names it
//     will have, and none writes an unnamed struct type of either.
//
// The struct must have a field that is not exported: its type is then identical to no
// struct type that another package writes, so the packages that the search looked at (the
// declaring package, and its importers, directly or not, for an exported field) have every
// type that could be identical to it, and every embedder. A struct of exported fields only
// keeps the scratch check: a package that neither imports the declaring package nor is
// searched can write a type that is identical to it, and that an importer is handed.
func (rn *renamer) fieldProvesResult(ctx context.Context) bool {
	sym := rn.sym
	if ctx.Err() != nil || recheckAllImporters || skipConflictChecks || !rn.bindingsChecked ||
		len(sym.objs) != 1 || len(sym.extra) != 0 || !rn.sameExportedness() ||
		!rn.searchedViewsAreOrdinary() {
		return false
	}
	if sym.scope != scopeModule && sym.scope != scopeLocal {
		return false
	}
	var declaration hit
	found := false
	for _, h := range rn.res.hits {
		if h.def {
			if found {
				return false
			}
			declaration = h
			found = true
		}
	}
	field, ok := declaration.obj.(*types.Var)
	if !found || !ok || !field.IsField() || field.Embedded() || declaration.id == nil ||
		declaration.v == nil || declaration.spec != nil || declaration.id.Name != sym.name {
		return false
	}
	v := declaration.v
	structure := v.fieldOwners().structOf[field]
	if structure == nil {
		return false
	}
	owners := namedWithStruct(v, structure)
	if len(owners) == 0 {
		return false
	}
	for _, named := range owners {
		if named.TypeParams().Len() != 0 || named.Obj().Pkg() != v.types ||
			named.Obj().Parent() != v.types.Scope() || rn.embeddedAnywhere(named) {
			return false
		}
	}
	// A struct type is identical to another one only with the same field names, and a name
	// that is not exported is the name of its package: a struct with one has no identical
	// type but the ones that its own package writes. Without one, any package that the
	// importers can see (the module's others, the packages that those import) can have an
	// unnamed struct type of the same fields, and a value of the named type is assignable
	// to it with no conversion and no struct type written in the importer: the scratch
	// check stays.
	hasUnexported := false
	for i := range structure.NumFields() {
		hasUnexported = hasUnexported || !structure.Field(i).Exported()
	}
	if !hasUnexported {
		return false
	}
	oldShape, newShape := shapeOf(structure, "", ""), shapeOf(structure, sym.name, rn.newName)
	for _, view := range rn.res.views {
		shapes := view.structShapes()
		if shapes.unknown != 0 || shapes.unnamed[oldShape] != 0 || shapes.unnamed[newShape] != 0 ||
			shapes.converted[oldShape] || shapes.converted[newShape] {
			return false
		}
	}
	// Every occurrence of the field in its package is an edit.
	seen := make(map[*ast.Ident]bool, len(rn.res.hits))
	for _, h := range rn.res.hits {
		if h.id == nil || h.spec != nil || h.id.Name != sym.name {
			return false
		}
		if h.v == v {
			seen[h.id] = true
		}
	}
	if list := v.identIndex().byName[sym.name]; list != nil {
		for _, entries := range [][]identObj{list.defs, list.uses} {
			for _, entry := range entries {
				if entry.obj == field && !seen[entry.id] {
					return false
				}
			}
		}
	}
	return ctx.Err() == nil
}

// embeddedAnywhere reports whether a struct or interface of the packages of the
// search embeds the named type (T, *T or an alias of one of them).
func (rn *renamer) embeddedAnywhere(named *types.Named) bool {
	for _, view := range rn.res.views {
		for _, record := range view.embeds() {
			for _, t := range record.embedded {
				t = types.Unalias(t)
				if pointer, ok := t.(*types.Pointer); ok {
					t = types.Unalias(pointer.Elem())
				}
				other, ok := t.(*types.Named)
				if !ok {
					continue
				}
				if other.Obj().Name() == named.Obj().Name() &&
					other.Obj().Pkg() != nil && named.Obj().Pkg() != nil &&
					other.Obj().Pkg().Path() == named.Obj().Pkg().Path() {
					return true
				}
			}
		}
	}
	return false
}

// shapeOf returns the names of the fields of the struct in order, with a mark on
// the embedded ones. The field called from is called to instead, when from is not
// empty.
func shapeOf(structure *types.Struct, from, to string) string {
	var shape strings.Builder
	for i := range structure.NumFields() {
		field := structure.Field(i)
		name := field.Name()
		if name == from && from != "" {
			name = to
		}
		shape.WriteString(name)
		if field.Embedded() {
			shape.WriteByte('\x01')
		}
		shape.WriteByte('\x00')
	}
	return shape.String()
}

type structShapesKey struct{}

// A shapes is what a package has of struct types, by field names (shapeOf): the
// struct types that are written without a name (not the type of a type
// declaration), the shapes of the conversions between two types that are not the
// same (a pointer to a struct counts as the struct; the conversion of nil or of
// a constant has no struct to convert), and the number of struct types and
// conversions that have no type.
type shapes struct {
	unnamed   map[string]int
	converted map[string]bool
	unknown   int
}

// structShapes returns the shapes of the view's files, once for each checked
// package.
func (v *view) structShapes() *shapes {
	build := func() any {
		found := &shapes{unnamed: make(map[string]int), converted: make(map[string]bool)}
		structOf := func(t types.Type) {
			if t == nil {
				found.unknown++
				return
			}
			t = types.Unalias(t)
			if pointer, ok := t.Underlying().(*types.Pointer); ok {
				t = pointer.Elem()
			}
			if structure, ok := t.Underlying().(*types.Struct); ok {
				found.converted[shapeOf(structure, "", "")] = true
			}
		}
		for _, file := range v.files {
			named := make(map[ast.Expr]bool)
			ast.Inspect(file, func(node ast.Node) bool {
				switch node := node.(type) {
				case *ast.TypeSpec:
					if !node.Assign.IsValid() {
						named[node.Type] = true
					}
				case *ast.StructType:
					if named[node] {
						break
					}
					if structure, ok := v.info.TypeOf(node).(*types.Struct); ok {
						found.unnamed[shapeOf(structure, "", "")]++
					} else {
						found.unknown++
					}
				case *ast.CallExpr:
					if len(node.Args) == 1 && v.info.Types[node.Fun].IsType() {
						target, source := v.info.TypeOf(node.Fun), v.info.TypeOf(node.Args[0])
						if target == nil || source == nil {
							found.unknown++
						} else if !untyped(source) && !types.Identical(target, source) {
							structOf(target)
							structOf(source)
						}
					}
				}
				return true
			})
		}
		return found
	}
	if v.pkg == nil || v.test {
		return build().(*shapes)
	}
	return v.pkg.Memo(structShapesKey{}, build).(*shapes)
}

// untyped reports whether t is the type of an untyped constant or nil: a value
// that has no struct type to be converted from.
func untyped(t types.Type) bool {
	basic, ok := t.(*types.Basic)
	return ok && basic.Info()&types.IsUntyped != 0
}

// namedWithStruct returns the named types of the view's package whose
// underlying type is the struct: the type that declares it, and the types
// declared as that type (type Copy Original), whose fields are the same.
func namedWithStruct(v *view, structure *types.Struct) []*types.Named {
	var out []*types.Named
	for _, obj := range v.info.Defs {
		name, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		if named, ok := name.Type().(*types.Named); ok && named.Underlying() == types.Type(structure) {
			out = append(out, named)
		}
	}
	return out
}

// methodProvesResult proves the result of the rename of a method of a named
// type, to a name of the same exportedness, when the narrow rule leaves nothing
// to check: no package that the search looked at, edited or not, can make an
// error (mayBreak: it has no name of the new name, and no use that needs an
// interface that has the old name or the new one), and every package that may
// have a tie that the search did not see was looked at. The closure of the
// search made the methods of the interfaces that the family is tied to part of
// it, and the guards looked at the members of the embedding types, so what is
// left is the binding at the edits (the guards), and that no occurrence of a
// method of the family is missed: each is an edit.
func (rn *renamer) methodProvesResult(
	ctx context.Context, dirs, edited map[string]bool, viewOf map[string]*view,
) bool {
	sym := rn.sym
	if ctx.Err() != nil || recheckAllImporters || skipConflictChecks || !rn.bindingsChecked ||
		!rn.declFilesKnown || rn.declHasTests || len(sym.objs) != 1 || len(sym.extra) != 0 ||
		len(rn.res.dynamic) != 0 || !rn.sameExportedness() || !rn.canNarrowCheck() ||
		!rn.searchedViewsAreOrdinary() || len(dirs) != len(edited) {
		return false
	}
	for dir := range edited {
		view := viewOf[dir]
		if view == nil || rn.mayBreak(view) {
			return false
		}
	}
	famKeys := make(map[objKey]bool, len(rn.res.keys))
	for _, key := range rn.res.keys {
		famKeys[key] = true
	}
	seen := make(map[*ast.Ident]bool, len(rn.res.hits))
	for _, h := range rn.res.hits {
		if h.id == nil || h.spec != nil || h.id.Name != sym.name {
			return false
		}
		seen[h.id] = true
	}
	for _, view := range rn.res.views {
		list := view.identIndex().byName[sym.name]
		if list == nil {
			continue
		}
		for _, entries := range [][]identObj{list.defs, list.uses} {
			for _, entry := range entries {
				fn, ok := entry.obj.(*types.Func)
				if !ok || recvBase(fn) == nil || !famKeys[keyOf(fn)] || seen[entry.id] {
					continue
				}
				return false
			}
		}
	}
	return ctx.Err() == nil
}
