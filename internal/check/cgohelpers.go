package check

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
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
//	func Wstr(s string) C.wstr     // C.wstr: the same for wide characters (UTF-16 on Windows)
//	func WString(s string) *C.wchar_t        // C.WString; the result is opaque: a C pointer
//	func GoStringW(p C.wide) string          // C.GoStringW: p is any pointer to 2- or 4-byte integers
//	func GoStringWN(p C.wide, n int) string  // C.GoStringWN
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
	// Phase 14 (step 14.4): the wide-string helpers of Windows (ADR-082).
	"wstr": "Wstr", "WString": "WString", "GoStringW": "GoStringW", "GoStringWN": "GoStringWN",
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
	if helperOf[sel.Sel.Name] != "" || sel.Sel.Name == unsafeSliceName || sel.Sel.Name == unsafeStringName || sel.Sel.Name == unsafeSizeName {
		return true
	}
	for _, form := range sliceForms {
		if sel.Sel.Name == form {
			return true
		}
	}
	return false
}

// CgoHelperDocs are the declarations of the cgo helpers as the user writes them, and what they
// do, by name: for the editor's hover.
var CgoHelperDocs = map[string][2]string{
	"CString":    {"func C.CString(s string) *C.char", "C.CString copies the bytes of s and a NUL into C's heap (malloc). Free the copy with C.free."},
	"GoString":   {"func C.GoString(p *C.char) string", "C.GoString copies the C string at p, up to its NUL, into a gocpp string. A nil p gives \"\". p may be any char pointer (const char*, unsigned char*)."},
	"GoStringN":  {"func C.GoStringN(p *C.char, n C.int) string", "C.GoStringN copies the n bytes at p into a gocpp string, NUL bytes too."},
	"GoBytes":    {"func C.GoBytes(p unsafe.Pointer, n C.int) []byte", "C.GoBytes copies the n bytes at p into a new []byte."},
	"free":       {"func C.free(p unsafe.Pointer)", "C.free frees memory of C's heap, as C's free (C.CString's copies). A nil p does nothing."},
	"wstr":       {"func C.wstr(s string) C.wstr", "C.wstr gives s to a W function as a const wchar_t* (LPCWSTR): UTF-16 where wchar_t has 16 bits (Windows), UTF-32 where it has 32. Invalid UTF-8 bytes become U+FFFD, and a surrogate written as three bytes (WTF-8) becomes a lone surrogate unit, as in Go's syscall.UTF16FromString. The text lives as long as the holder that C.wstr returns: to the end of the statement, or as long as the variable it is in (name := C.wstr(s))."},
	"GoStringW":  {"func C.GoStringW(p *C.wchar_t) string", "C.GoStringW copies the wide C string at p, up to its NUL, into a gocpp string (UTF-8; a lone surrogate becomes its three WTF-8 bytes, as Go's syscall.UTF16ToString). A nil p gives \"\". p is any pointer to 2-byte or 4-byte integers (LPCWSTR, LPWSTR, a WCHAR buffer)."},
	"GoStringWN": {"func C.GoStringWN(p *C.wchar_t, n C.int) string", "C.GoStringWN copies the n wide characters at p into a gocpp string, NUL characters too."},
	"cstr":       {"func C.cstr(s string) C.cstr", "C.cstr gives s to C as a const char*, without a copy when a NUL follows its bytes already. The C string lives as long as the holder that C.cstr returns: to the end of the statement, or as long as the variable it is in (name := C.cstr(s))."},
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
	fn("Wstr", []*types.Var{param("s", str)}, invalid)
	fn("WString", []*types.Var{param("s", str)}, invalid)
	// A wide pointer is opaque (the invalid type): LPCWSTR of a header, a *uint16 of a gocpp slice, ...
	fn("GoStringW", []*types.Var{param("p", invalid)}, str)
	fn("GoStringWN", []*types.Var{param("p", invalid), param("n", cint)}, str)
	addUnsafeHelpers(pkg) // unsafe.Slice and unsafe.String of C values (unsafecalls.go)
	addSliceHelper(pkg)   // a[lo:hi] of a C array (slicecalls.go)
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

// checkWideArgs reports the calls of C.GoStringW and C.GoStringWN whose first argument cannot be a wide C string: the
// parameter of the hidden function is the invalid type, so that any C pointer passes (an LPCWSTR of a header, a
// wchar_t* of any size), and go/types would let a string or an int through to a C++ error. The pointer must be a C
// value (opaque), nil, or a gocpp pointer to an integer of 2 or 4 bytes (*uint16, *int32): the units of UTF-16 and
// UTF-32.
func (cv *converter) checkWideArgs(p *Package, fset *token.FileSet) {
	if p == nil || p.Info == nil {
		return
	}
	for _, f := range cv.lp.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
			if !ok {
				return true
			}
			name := CgoHelperOf(sel, p.Info)
			if name != "GoStringW" && name != "GoStringWN" {
				return true
			}
			arg := call.Args[0]
			t := p.Info.TypeOf(arg)
			if t == nil || wideArgOK(t) {
				return true
			}
			p.Errors.Add(diag.Diagnostic{
				Pos: fset.Position(arg.Pos()),
				End: fset.Position(arg.End()),
				Msg: "cannot use " + types.ExprString(arg) + " (value of type " + types.TypeString(t, nil) + ") as a wide C string in argument to C." + name +
					": want a C pointer to wide characters (LPCWSTR, wchar_t*), a *uint16 or nil",
			})
			return true
		})
	}
}

// wideArgOK reports whether a value of type t can be the pointer of C.GoStringW: a C value (invalid type), nil, or a
// pointer to a 2-byte or 4-byte integer.
func wideArgOK(t types.Type) bool {
	if b, ok := t.(*types.Basic); ok {
		return b.Kind() == types.Invalid || b.Kind() == types.UntypedNil
	}
	ptr, ok := t.Underlying().(*types.Pointer)
	if !ok {
		return false
	}
	b, ok := ptr.Elem().Underlying().(*types.Basic)
	if !ok {
		return false
	}
	switch b.Kind() {
	case types.Int16, types.Uint16, types.Int32, types.Uint32, types.Invalid:
		return true
	}
	return false
}
