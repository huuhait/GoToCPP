package check

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/huuhait/gocpp/internal/load"
)

// The typed cgo helpers (Phase 04 README §2.6 "Typed helpers", step 4.9).
//
// FakeImportC makes every C.X opaque, also cgo's helpers, so s := C.GoString(p) would leave s
// without a type, and s + "!" would not be checked. So, before the first check of a package
// with C in it, Check rewrites each call of a helper that the user wrote, C.GoString(p), into a
// call of a function of the hidden package gocpp/cgoh, gp_cgoh.GoString(p) (the same import
// as the conversions', see Conversion), which has a real declaration:
//
//	package cgoh
//
//	type char int8 // C.char in the messages
//	type int int32 // C.int in the messages
//
//	func CString(s string) *C.char // C.CString; the result is opaque: a C pointer
//	func GoString(p *char) string
//	func GoStringN(p *char, n int) string
//	func GoBytes(p unsafe.Pointer, n int) []byte
//	func Free(p unsafe.Pointer)    // C.free
//	func Cstr(s string) C.cstr     // C.cstr; the result is opaque: the holder, a C++ object
//
// (*C.char and C.cstr stand for the invalid type: a variable declared *C.char has it too, since
// go/types makes a pointer to an invalid type invalid, so CString's result compares with nil and
// passes like one. free and cstr
// get exported names there, Free and Cstr, so that the user's package can call them: the
// rewrite changes the name of the selector too.)
//
// The parameters are cgo's (cmd/cgo/out.go), so a gocpp value of another type is go/types' own
// error, as in cgo (C.GoString(b) with b []byte, C.GoStringN(p, n) with n int, C.CString(n)
// with n int), and an opaque argument (a C/C++ value) passes silently, as everywhere: go/types
// says nothing about an invalid operand. A variable of a C pointer type (var p *C.uchar, of
// the type *invalid) gives "cannot use p ... as *C.char value", a false error that the
// collector drops (falseErrors: incompatibleAssign of a C/C++ value), so any char pointer
// passes, as the runtime's helpers accept (gp::cgo::GoString takes char*, const char*,
// unsigned char*). The results of GoString, GoStringN and GoBytes are gocpp types, so s is a
// string; CString's and cstr's are C/C++ values (the invalid type, which the opaque pass labels
// opaque: objects of another package with an incomplete type come from C/C++).
//
// C.cstr is rewritten too, although README §2.6 keeps it opaque (its holder must stay a C++
// object): only its parameter gets a type; its result stays opaque, the C++ holder.
//
// A helper that is not called (f := C.GoString) is not rewritten: it stays an opaque C.X,
// which codegen reports ("must call C.GoString", as cgo). Messages show the helpers and their
// types as the user wrote them (fixMessage): C.GoString, *C.char, C.int.
//
// Check does the rewrite, not the loader's Phase 01 rewrite that README §2.6 names: the hidden
// package is Check's own (it is made in Go code), and so is the rest of the rewrite machinery
// (the import, the messages).

// cgoHelpers maps the names of the cgo helpers that Check rewrites (C.free) to their names in the
// hidden package (Free).
var cgoHelpers = map[string]string{
	"CString": "CString", "GoString": "GoString", "GoStringN": "GoStringN", "GoBytes": "GoBytes",
	"free": "Free", "cstr": "Cstr",
}

// helperOf maps the names of the helpers in the hidden package to the names that the user writes.
var helperOf = func() map[string]string {
	m := make(map[string]string, len(cgoHelpers))
	for user, hidden := range cgoHelpers {
		m[hidden] = user
	}
	return m
}()

// CgoHelper returns the name of the cgo helper (GoString) that call calls after Check's
// rewrite, gp_cgoh.GoString(p), or "".
func (p *Package) CgoHelper(call *ast.CallExpr) string {
	if p == nil || p.Info == nil {
		return ""
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	return CgoHelperOf(sel, p.Info)
}

// IsCgoHelper reports whether sel is a cgo helper after Check's rewrite: gp_cgoh.GoString.
func (p *Package) IsCgoHelper(sel *ast.SelectorExpr) bool {
	return p != nil && p.Info != nil && CgoHelperOf(sel, p.Info) != ""
}

// HiddenImport returns the name of the import of the hidden package gocpp/cgoh that Check added
// to the file f (gp_cgoh), or "". Every name with it in f is Check's: the X of a rewritten
// conversion (gp_cgoh.As[T](x), at the place of T) or of a cgo helper (gp_cgoh.GoString, at the
// place of the user's C), which covers more text than the user's own name there. Tools that look
// up the name at a place skip them.
func HiddenImport(f *ast.File) string {
	if spec := cgohImport(f); spec != nil {
		return spec.Name.Name
	}
	return ""
}

// IsHelperSelector reports whether sel, whose X is the hidden import of its file (HiddenImport),
// is a rewritten cgo helper, gp_cgoh.GoString, whose X stands for the user's C.
func IsHelperSelector(sel *ast.SelectorExpr) bool {
	return helperOf[sel.Sel.Name] != ""
}

// CgoHelperDocs are the declarations of the cgo helpers as the user writes them, and what they
// do, by name: for the editor's hover.
var CgoHelperDocs = map[string][2]string{
	"CString":   {"func C.CString(s string) *C.char", "C.CString copies the bytes of s and a NUL into C's heap (malloc). Free the copy with C.free."},
	"GoString":  {"func C.GoString(p *C.char) string", "C.GoString copies the C string at p, up to its NUL, into a gocpp string. A nil p gives \"\". p may be any char pointer (const char*, unsigned char*)."},
	"GoStringN": {"func C.GoStringN(p *C.char, n C.int) string", "C.GoStringN copies the n bytes at p into a gocpp string, NUL bytes too."},
	"GoBytes":   {"func C.GoBytes(p unsafe.Pointer, n C.int) []byte", "C.GoBytes copies the n bytes at p into a new []byte."},
	"free":      {"func C.free(p unsafe.Pointer)", "C.free frees memory of C's heap, as C's free (C.CString's copies). A nil p does nothing."},
	"cstr":      {"func C.cstr(s string) C.cstr", "C.cstr gives s to C as a const char*, without a copy when a NUL follows its bytes already. The C string lives as long as the holder that C.cstr returns: to the end of the statement, or as long as the variable it is in (name := C.cstr(s))."},
}

// CgoHelperOf returns the name of the cgo helper that sel names after Check's rewrite
// (gp_cgoh.GoString, with the package's info), as the user wrote it (GoString, free), or "".
func CgoHelperOf(sel *ast.SelectorExpr, info *types.Info) string {
	id, ok := sel.X.(*ast.Ident)
	user := helperOf[sel.Sel.Name]
	if !ok || user == "" {
		return ""
	}
	if pn, ok := info.Uses[id].(*types.PkgName); ok && pn.Imported().Path() == load.CgohPath {
		return user
	}
	return ""
}

// addHelpers declares the cgo helpers in the hidden package pkg (see above).
func addHelpers(pkg *types.Package) {
	named := func(name string, under types.Type) *types.Named {
		tn := types.NewTypeName(token.NoPos, pkg, name, nil)
		t := types.NewNamed(tn, under, nil)
		pkg.Scope().Insert(tn)
		return t
	}
	char := types.NewPointer(named("char", types.Typ[types.Int8]))
	cint := named("int", types.Typ[types.Int32])
	param := func(name string, t types.Type) *types.Var { return types.NewParam(token.NoPos, pkg, name, t) }
	fn := func(name string, params []*types.Var, results ...types.Type) {
		var res []*types.Var
		for _, r := range results {
			res = append(res, param("", r))
		}
		sig := types.NewSignatureType(nil, nil, nil, types.NewTuple(params...), types.NewTuple(res...), false)
		pkg.Scope().Insert(types.NewFunc(token.NoPos, pkg, name, sig))
	}
	str, ptr := types.Typ[types.String], types.Typ[types.UnsafePointer]
	invalid := types.Typ[types.Invalid]
	fn("CString", []*types.Var{param("s", str)}, invalid)
	fn("GoString", []*types.Var{param("p", char)}, str)
	fn("GoStringN", []*types.Var{param("p", char), param("n", cint)}, str)
	fn("GoBytes", []*types.Var{param("p", ptr), param("n", cint)}, types.NewSlice(types.Universe.Lookup("byte").Type()))
	fn("Free", []*types.Var{param("p", ptr)})
	fn("Cstr", []*types.Var{param("s", str)}, invalid)
}

// rewriteHelpers rewrites the calls of the cgo helpers that the user wrote, C.GoString(p), into
// calls of the hidden package, gp_cgoh.GoString(p) (see above), in every file of the package
// with import "C". Only the C of the selector changes: the new name has its place, so messages
// and the editor find the call where the user wrote it. A C.X that the loader made out of a
// header package's use (cstdlib.free) is a C/C++ name, not a helper: it stays.
func (cv *converter) rewriteHelpers() {
	for _, f := range cv.lp.Files {
		if !importsC(f.AST) {
			continue
		}
		made := make(map[*ast.SelectorExpr]bool) // the selectors of the loader's rewrite
		if f.Rewrite != nil {
			for _, u := range f.Rewrite.Uses {
				made[u.Sel] = true
			}
		}
		ast.Inspect(f.AST, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
			if !ok || made[sel] || cgoHelpers[sel.Sel.Name] == "" {
				return true
			}
			// C is the package: the loader lets no name be C (load's reserved names), and a
			// name that the file declares would have an object of the parser (Ident.Obj).
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "C" && id.Obj == nil {
				sel.X = &ast.Ident{NamePos: id.NamePos, Name: cv.importName(f.AST)}
				if hidden := cgoHelpers[sel.Sel.Name]; hidden != sel.Sel.Name {
					sel.Sel = &ast.Ident{NamePos: sel.Sel.NamePos, Name: hidden}
				}
			}
			return true
		})
	}
}

// fixHelpers returns msg with the names of the hidden package that a message of go/types can
// show turned back into what the user wrote: name.GoString and name.Free (name is the file's
// import of gocpp/cgoh) into C.GoString and C.free, and the types cgoh.char and cgoh.int into
// C.char and C.int.
func fixHelpers(msg, name string) string {
	if !strings.Contains(msg, "cgoh") {
		return msg
	}
	for hidden, user := range helperOf {
		msg = replaceWord(msg, name+"."+hidden, "C."+user, func(rest string) bool { return rest == "" || !isNameByte(rest[0]) })
	}
	for _, t := range []string{"char", "int"} {
		msg = replaceWord(msg, "cgoh."+t, "C."+t, func(rest string) bool { return rest == "" || !isNameByte(rest[0]) })
	}
	return msg
}

// replaceWord replaces each old in msg that doesn't follow a name byte, and for which ok
// reports true for the text after it, with new.
func replaceWord(msg, old, new string, ok func(rest string) bool) string {
	var b strings.Builder
	for {
		i := strings.Index(msg, old)
		if i < 0 {
			b.WriteString(msg)
			return b.String()
		}
		rest := msg[i+len(old):]
		if (i == 0 || !isNameByte(msg[i-1])) && ok(rest) {
			b.WriteString(msg[:i])
			b.WriteString(new)
		} else {
			b.WriteString(msg[:i+len(old)])
		}
		msg = rest
	}
}
