package check

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/load"
)

// unsafe.Slice and unsafe.String of C/C++ values (Phase 04 README §2.7, step 4.10).
//
// go/types gives a call of an unsafe builtin no type when one argument has none, and a C pointer
// has none (a pointer to the invalid type is invalid), nor has a C number: so with names
// *C.char and size C.int,
//
//	cols := unsafe.Slice(names, int(n))    // names: **C.char
//	b := unsafe.Slice(&buf[0], size)       // size: a C.size_t
//	s := unsafe.String(&buf[0], size)
//
// would leave cols, b and s without a type, and len(cols), range b and s + "!" couldn't be
// checked or lowered as the gocpp values that they are. So, like a conversion of an opaque value
// (convert.go), the calls get a rewrite and the package is checked again:
//
//   - a typed pointer and an opaque length: the length becomes gp_cgoh.As[int](size), so the
//     call is a normal, typed Go call (a conversion to int, as cgo's rules ask of the user);
//   - an opaque pointer: the call goes into a call of gp_cgoh.UnsafeSlice(unsafe.Slice(p, n))
//     or gp_cgoh.UnsafeString(unsafe.String(p, n)), whose results have the types that
//     unsafe.Slice and unsafe.String give: []C.T (a slice of the invalid type, like var xs
//     []C.int) and string. The parameter takes any value, so go/types checks nothing about the
//     call (the C++ compiler does: gp::unsafe_slice takes a raw pointer or a gocpp pointer), and
//     the call stays where it was, so the import of unsafe stays used.
//
// unsafe.Sizeof, Alignof and Offsetof of a C/C++ value or of a type with a C/C++ part are
// wrapped in gp_cgoh.UnsafeSize(unsafe.Sizeof(x)), a call of type uintptr: go/types would fold
// the call into a constant with a guess for the C part, or give it no type, and gocpp knows the
// real number only from the C++ compiler (sizeof, alignof, offsetof). So the result is a value,
// not a constant: a constant context ([unsafe.Sizeof(x)]byte, a const declaration) is Go's error
// "... is not constant", and everywhere else the lowering writes sizeof(x).
//
// unsafe.Add, SliceData and StringData need no rewrite: their results are unsafe.Pointer, a
// pointer to the elements of a slice whose type go/types has (var xs []C.int gives *C.int, a
// pointer to the invalid type, which stays opaque), and *byte.

// The names of the unsafe helpers in gocpp/cgoh, by the unsafe function they stand for.
const (
	unsafeSliceName  = "UnsafeSlice"
	unsafeStringName = "UnsafeString"
	unsafeSizeName   = "UnsafeSize"
)

// unsafeSizes are the unsafe builtins that give a size (they have one argument).
var unsafeSizes = map[string]bool{"Sizeof": true, "Alignof": true, "Offsetof": true}

// unsafeHelpers maps the unsafe function to its helper in gocpp/cgoh.
var unsafeHelpers = map[string]string{"Slice": unsafeSliceName, "String": unsafeStringName}

// An unsafeCandidate is a call of unsafe.Slice or unsafe.String that has no type, and the field
// that holds it.
type unsafeCandidate struct {
	call *ast.CallExpr
	file *ast.File
	slot *ast.Expr
	name string // Slice or String
}

// unsafeCall returns the name (Slice, String, Sizeof, Alignof, Offsetof) of the unsafe builtin
// that call calls, or "".
func unsafeCall(call *ast.CallExpr, info *types.Info) string {
	var id *ast.Ident
	switch f := ast.Unparen(call.Fun).(type) {
	case *ast.SelectorExpr:
		id = f.Sel
	case *ast.Ident:
		id = f
	default:
		return ""
	}
	if b, ok := info.Uses[id].(*types.Builtin); ok && b.Pkg() == types.Unsafe && (unsafeHelpers[b.Name()] != "" || unsafeSizes[b.Name()]) {
		return b.Name()
	}
	return ""
}

// opaqueUnsafeCalls returns the calls of unsafe.Slice and unsafe.String in f, in source order,
// that have no type, and the calls of unsafe.Sizeof, Alignof and Offsetof that have no type or
// whose constant value depends on a C/C++ type.
func opaqueUnsafeCalls(f *ast.File, info *types.Info) []unsafeCandidate {
	var cands []unsafeCandidate
	ast.Inspect(f, func(n ast.Node) bool {
		if w, ok := n.(*ast.CallExpr); ok && len(w.Args) == 1 {
			if sel, ok := w.Fun.(*ast.SelectorExpr); ok && unsafeHelperOf(sel, info) != "" {
				return true // its call is wrapped already
			}
		}
		for _, slot := range exprSlots(n) {
			call, ok := (*slot).(*ast.CallExpr)
			if !ok || call.Ellipsis.IsValid() {
				continue
			}
			name := unsafeCall(call, info)
			if name == "" || len(call.Args) != 2 && !unsafeSizes[name] || len(call.Args) != 1 && unsafeSizes[name] {
				continue
			}
			if tv, ok := info.Types[call]; ok && isValidType(tv.Type) && !hasInvalid(tv.Type) {
				if _, isC := sizeCallCPart(call, name, info); !unsafeSizes[name] || !isC {
					continue
				}
			}
			cands = append(cands, unsafeCandidate{call: call, file: f, slot: slot, name: name})
		}
		return true
	})
	return cands
}

// rewriteUnsafe rewrites the call of u, if its arguments are opaque (see above), and reports
// whether it did.
func (cv *converter) rewriteUnsafe(a *analysis, u unsafeCandidate) bool {
	if unsafeSizes[u.name] {
		return cv.rewriteUnsafeSize(a, u)
	}
	p, n := u.call.Args[0], u.call.Args[1]
	if a.errorIn(u.call) {
		return false
	}
	sp, sn := a.state(p), a.state(n)
	if sp == waiting || sn == waiting || max(sp, sn) != opaque {
		return false // waits for the next round, or isn't C's doing
	}
	if sp == typed {
		// A typed pointer and an opaque length: gp_cgoh.As[int](n).
		conv := &ast.CallExpr{
			Fun:    &ast.Ident{NamePos: n.Pos(), Name: "int"},
			Lparen: n.Pos(),
			Args:   []ast.Expr{n},
			Rparen: n.End() - 1,
		}
		cv.rewriteCall(candidate{call: conv, file: u.file})
		u.call.Args[1] = conv
		return true
	}
	// An opaque pointer: the call goes into the helper, which gives it its type, and stays
	// as it is (so the import of unsafe stays used).
	pos := u.call.Pos()
	*u.slot = &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{NamePos: pos, Name: cv.importName(u.file)},
			Sel: &ast.Ident{NamePos: pos, Name: unsafeHelpers[u.name]},
		},
		Lparen: pos,
		Args:   []ast.Expr{u.call},
		Rparen: u.call.End() - 1,
	}
	return true
}

// rewriteUnsafeSize wraps the call of unsafe.Sizeof, Alignof or Offsetof of u in
// gp_cgoh.UnsafeSize(call), if its argument is a C/C++ value or has a C/C++ part.
func (cv *converter) rewriteUnsafeSize(a *analysis, u unsafeCandidate) bool {
	arg := u.call.Args[0]
	if a.errorIn(u.call) {
		return false
	}
	switch a.state(arg) {
	case waiting:
		return false
	case opaque:
	default:
		if _, isC := sizeCallCPart(u.call, u.name, a.info); !isC {
			return false
		}
	}
	pos := u.call.Pos()
	*u.slot = &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{NamePos: pos, Name: cv.importName(u.file)},
			Sel: &ast.Ident{NamePos: pos, Name: unsafeSizeName},
		},
		Lparen: pos,
		Args:   []ast.Expr{u.call},
		Rparen: u.call.End() - 1,
	}
	return true
}

// UnsafeWrapper returns the call of unsafe.Slice or unsafe.String that check wrapped into a call
// of its helper in gocpp/cgoh, gp_cgoh.UnsafeSlice(unsafe.Slice(p, n)), or nil if call is not
// such a wrapper.
func (p *Package) UnsafeWrapper(call *ast.CallExpr) *ast.CallExpr {
	if p == nil || p.Info == nil || len(call.Args) != 1 {
		return nil
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || unsafeHelperOf(sel, p.Info) == "" {
		return nil
	}
	inner, _ := ast.Unparen(call.Args[0]).(*ast.CallExpr)
	return inner
}

// unsafeHelperOf returns Slice or String if sel names UnsafeSlice or UnsafeString of
// gocpp/cgoh, else "".
func unsafeHelperOf(sel *ast.SelectorExpr, info *types.Info) string {
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	if pn, ok := info.Uses[id].(*types.PkgName); !ok || pn.Imported().Path() != load.CgohPath {
		return ""
	}
	switch sel.Sel.Name {
	case unsafeSliceName:
		return "Slice"
	case unsafeStringName:
		return "String"
	case unsafeSizeName:
		return "Size"
	}
	return ""
}

// addUnsafeHelpers declares the helpers of unsafe.Slice and unsafe.String in the hidden package
// pkg: func UnsafeSlice(call any) []C.T, with the invalid type as the element,
// func UnsafeString(call any) string and func UnsafeSize(call any) uintptr: each takes the call of
// the unsafe function, and gives it its type.
func addUnsafeHelpers(pkg *types.Package) {
	anyType := types.Universe.Lookup("any").Type()
	fn := func(name string, result types.Type) {
		sig := types.NewSignatureType(nil, nil, nil, types.NewTuple(types.NewParam(token.NoPos, pkg, "call", anyType)),
			types.NewTuple(types.NewParam(token.NoPos, pkg, "", result)), false)
		pkg.Scope().Insert(types.NewFunc(token.NoPos, pkg, name, sig))
	}
	fn(unsafeSliceName, types.NewSlice(types.Typ[types.Invalid]))
	fn(unsafeStringName, types.Typ[types.String])
	fn(unsafeSizeName, types.Typ[types.Uintptr])
}

// The text of messages. go/types and the lowerer print the expressions of the rewritten AST, so
// gp_cgoh.UnsafeSlice(unsafe.Slice(p, n)) and gp_cgoh.SliceCLoHi(a, 1, 2) would show: unwrapHelpers
// turns them back into what the user wrote, unsafe.Slice(p, n) and a[1:2].

// sliceText returns the slice expression that the call of the helper form with the arguments
// args (SliceCLoHi(a, lo, hi)) stands for, a[lo:hi], or "".
func sliceText(form string, args []string) string {
	for bounds, name := range sliceForms {
		if name != form || len(args) == 0 {
			continue
		}
		var b strings.Builder
		b.WriteString(args[0])
		b.WriteByte('[')
		rest := args[1:]
		next := func(has bool) string {
			if !has || len(rest) == 0 {
				return ""
			}
			x := rest[0]
			rest = rest[1:]
			return x
		}
		b.WriteString(next(bounds[0]))
		b.WriteByte(':')
		b.WriteString(next(bounds[1]))
		if bounds[2] {
			b.WriteByte(':')
			b.WriteString(next(true))
		}
		b.WriteByte(']')
		return b.String()
	}
	return ""
}

// splitArgs splits the arguments of a call at the commas outside brackets and quotes.
func splitArgs(s string) []string {
	var args []string
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '[', '(', '{':
			if end := closing(s, i+1); end > 0 {
				i = end - 1
			}
		case '"', '\'', '`':
			c := s[i]
			for i++; i < len(s) && s[i] != c; i++ {
				if s[i] == '\\' && c != '`' {
					i++
				}
			}
		case ',':
			args = append(args, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	return append(args, strings.TrimSpace(s[start:]))
}

// unwrapHelpers turns each name.UnsafeSlice(call), name.UnsafeString(call) and
// name.UnsafeSize(call) in msg into call, and each name.SliceC...(a, ...) into the slice
// expression, a[lo:hi]. name is the import of gocpp/cgoh.
func unwrapHelpers(msg, name string) string {
	if !strings.Contains(msg, name+".") {
		return msg
	}
	size := strings.Contains(msg, name+"."+unsafeSizeName+"(")
	wrappers := []string{unsafeSliceName, unsafeStringName, unsafeSizeName}
	for _, form := range sliceForms {
		wrappers = append(wrappers, form)
	}
	// The longest names first: SliceCLo is a prefix of SliceCLoHi.
	slices.SortFunc(wrappers, func(a, b string) int { return len(b) - len(a) })
	for _, w := range wrappers {
		prefix := name + "." + w + "("
		from := 0
		for {
			i := strings.Index(msg[from:], prefix)
			if i < 0 {
				break
			}
			i += from
			from = i + 1
			if i > 0 && isNameByte(msg[i-1]) {
				continue
			}
			j := i + len(prefix)
			end := closing(msg, j)
			if end < 0 {
				continue
			}
			args := msg[j : end-1]
			repl := args
			if strings.HasPrefix(w, "SliceC") {
				if repl = sliceText(w, splitArgs(args)); repl == "" {
					continue
				}
			}
			msg = msg[:i] + repl + msg[end:]
			from = i
		}
	}
	if size && (strings.HasSuffix(msg, "is not constant") || strings.HasSuffix(msg, "must be constant")) {
		msg += ": the size of a C/C++ type is known only to the C++ compiler, so it is a value, not a Go constant"
	}
	return msg
}

// UserText returns text, which prints expressions of the package's ASTs (the lowerer's messages),
// with the names of gocpp/cgoh that the rewrites added turned back into what the user wrote.
func (p *Package) UserText(text string) string {
	for name := range p.hidden {
		text = unwrapHelpers(text, name)
	}
	return text
}
