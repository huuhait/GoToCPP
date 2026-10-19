package vet

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/huuhait/gocpp/internal/check"
)

// The analyzers about the C/C++ boundary (Phase 13 README §2.3). The loader turns every header
// name into a name of the package C (stat.stat is C.stat, std.string_view is C.std.string_view),
// and go/types gives those the invalid type, so these analyzers look at the shape of the syntax:
// what is rooted at C, and what Check's rewrite made of the cgo helpers (gp_cgoh.Cstr).

// A cinfo resolves what names the package C stands for, also through the type names that the
// package declares for them: type St = stat.struct_stat, type Count C.int.
type cinfo struct {
	info  *types.Info
	specs map[types.Object]ast.Expr // the declared type of each type name of the package
}

func newCinfo(pass *analysis.Pass) *cinfo {
	ci := &cinfo{info: pass.TypesInfo, specs: make(map[types.Object]ast.Expr)}
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok && ts.TypeParams == nil {
				if obj := pass.TypesInfo.Defs[ts.Name]; obj != nil {
					ci.specs[obj] = ts.Type
				}
			}
			return true
		})
	}
	return ci
}

// declared returns the type expression of the type name that id uses, or nil.
func (ci *cinfo) declared(id *ast.Ident) ast.Expr {
	if tn, ok := ci.info.Uses[id].(*types.TypeName); ok {
		return ci.specs[tn]
	}
	return nil
}

// name returns the name that e has in the package C, without the C: "stat" for C.stat,
// "std.string_view" for C.std.string_view; ok is false for anything else. A type name that the
// package declares for one is resolved (a few levels deep: type A = B, type B = C.int).
func (ci *cinfo) name(e ast.Expr) (name string, ok bool) {
	return ci.nameDepth(e, 0)
}

func (ci *cinfo) nameDepth(e ast.Expr, depth int) (name string, ok bool) {
	var parts []string
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.SelectorExpr:
			parts = append([]string{x.Sel.Name}, parts...)
			e = x.X
			continue
		case *ast.Ident:
			if pn, isPkg := ci.info.Uses[x].(*types.PkgName); isPkg && pn.Imported().Path() == "C" && len(parts) > 0 {
				return strings.Join(parts, "."), true
			}
			if len(parts) == 0 && depth < 4 {
				if te := ci.declared(x); te != nil {
					return ci.nameDepth(te, depth+1)
				}
			}
		}
		return "", false
	}
}

// lastName is the last element of a dotted name: string_view for std.string_view.
func lastName(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[i+1:]
	}
	return name
}

// valueType reports whether the type expression te, as a variable's declared type, is a C/C++
// value: C.T, std.vector[int], an array of those, or a type name of the package for one; not a
// pointer, a slice or a map.
func (ci *cinfo) valueType(te ast.Expr) bool { return ci.valueTypeDepth(te, 0) }

func (ci *cinfo) valueTypeDepth(te ast.Expr, depth int) bool {
	switch t := ast.Unparen(te).(type) {
	case *ast.Ident:
		if d := ci.declared(t); d != nil && depth < 4 {
			return ci.valueTypeDepth(d, depth+1)
		}
	case *ast.SelectorExpr:
		_, ok := ci.name(t)
		return ok
	case *ast.IndexExpr:
		return ci.valueTypeDepth(t.X, depth)
	case *ast.IndexListExpr:
		return ci.valueTypeDepth(t.X, depth)
	case *ast.ArrayType:
		return t.Len != nil && ci.valueTypeDepth(t.Elt, depth)
	}
	return false
}

// walkStack calls f for every node under root with the nodes above it (root first); f returns false
// to skip the subtree.
func walkStack(root ast.Node, f func(n ast.Node, stack []ast.Node) bool) {
	var stack []ast.Node
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if !f(n, stack) {
			return false
		}
		stack = append(stack, n)
		return true
	})
}

// --- cpplocal ---

// cpplocalAnalyzer finds the address of a C/C++ local variable that is kept beyond the variable.
//
// &x of a C/C++ value is a raw C++ address (docs/spec/interop.md, "&x of a C/C++ value"): the
// variable ends with its block, not when nothing points to it any more as in Go. `return &x` is an
// error of the build; the other ways to keep the address compile and dangle:
//
//	g = &x          h.p = &x       (g a package variable, h a pointer parameter)
//	out[i] = &x     list = append(list, &x)   (the target is declared outside the block of x)
//	ch <- &x        p := &x; return p        return T{p: &x}
//
// The analyzer reports each &x (or &x.f, &x[i] of an array) of a local variable that is declared
// with a C/C++ type (var x C.T, x := C.T{...}, a parameter x C.T) when its value is stored in one
// of those places. It does not follow the address through calls, fields of local structs or
// goroutines, so a function that stores the pointer it was given is not found.
var cpplocalAnalyzer = &analysis.Analyzer{
	Name:             "cpplocal",
	Doc:              "report the address of a C/C++ local variable that is stored where it outlives the variable (a package variable, a pointer parameter, an outer variable, a channel, a result)",
	RunDespiteErrors: true,
	Run:              runCppLocal,
}

func runCppLocal(pass *analysis.Pass) (any, error) {
	ci := newCinfo(pass)
	for _, f := range pass.Files {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Body != nil {
					cppLocalFunc(pass, ci, d.Type, d.Recv, d.Body)
				}
			case *ast.GenDecl:
				ast.Inspect(d, func(n ast.Node) bool {
					if lit, ok := n.(*ast.FuncLit); ok {
						cppLocalFunc(pass, ci, lit.Type, nil, lit.Body)
						return false
					}
					return true
				})
			}
		}
	}
	return nil, nil
}

// cppLocalFunc looks at one function; the func literals in it are separate functions.
func cppLocalFunc(pass *analysis.Pass, ci *cinfo, typ *ast.FuncType, recv *ast.FieldList, body *ast.BlockStmt) {
	info := pass.TypesInfo
	locals := make(map[types.Object]bool) // C/C++ values declared in this function
	params := make(map[types.Object]bool)
	results := make(map[types.Object]bool) // named results: what is stored in them is returned
	addParams := func(fl *ast.FieldList, value bool) {
		if fl == nil {
			return
		}
		for _, field := range fl.List {
			for _, id := range field.Names {
				if obj := info.Defs[id]; obj != nil {
					params[obj] = true
					if value && ci.valueType(field.Type) {
						locals[obj] = true
					}
				}
			}
		}
	}
	addParams(recv, false)
	addParams(typ.Params, true)
	addParams(typ.Results, false)
	if typ.Results != nil {
		for _, field := range typ.Results.List {
			for _, id := range field.Names {
				if obj := info.Defs[id]; obj != nil {
					results[obj] = true
				}
			}
		}
	}

	var lits []*ast.FuncLit
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			lits = append(lits, n)
			return false
		case *ast.ValueSpec:
			for i, id := range n.Names {
				if obj := info.Defs[id]; obj != nil {
					switch {
					case n.Type != nil && ci.valueType(n.Type):
						locals[obj] = true
					case n.Type == nil && i < len(n.Values) && ci.compositeValue(n.Values[i]):
						locals[obj] = true
					}
				}
			}
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE && len(n.Lhs) == len(n.Rhs) {
				for i, l := range n.Lhs {
					if id, ok := l.(*ast.Ident); ok && ci.compositeValue(n.Rhs[i]) {
						if obj := info.Defs[id]; obj != nil {
							locals[obj] = true
						}
					}
				}
			}
		}
		return true
	})
	for _, lit := range lits {
		cppLocalFunc(pass, ci, lit.Type, nil, lit.Body)
	}
	if len(locals) == 0 {
		return
	}

	cf := &cppFunc{info: info, params: params, results: results, assigned: make(map[types.Object]int), returned: make(map[types.Object]bool)}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.DeferStmt:
			cf.hasDefer = true
		case *ast.AssignStmt:
			if n.Tok != token.DEFINE {
				for _, l := range n.Lhs {
					if id, ok := ast.Unparen(l).(*ast.Ident); ok {
						cf.assigned[info.Uses[id]]++
					}
				}
			}
		case *ast.ReturnStmt:
			for _, r := range n.Results {
				if id, ok := ast.Unparen(r).(*ast.Ident); ok {
					cf.returned[info.Uses[id]] = true
				}
			}
		}
		return true
	})

	walkStack(body, func(n ast.Node, stack []ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		un, ok := n.(*ast.UnaryExpr)
		if !ok || un.Op != token.AND {
			return true
		}
		x := cLocalRoot(info, un.X, locals)
		if x == nil {
			return true
		}
		if why := cf.keptWhere(x, un, stack); why != "" {
			pass.Report(analysis.Diagnostic{
				Pos: un.Pos(), End: un.End(), Category: "cpplocal",
				Message: fmt.Sprintf("%s takes the address of the C/C++ variable %s and %s: a C/C++ local ends with its block (not when nothing points to it, as in Go), so the pointer dangles; make the object with &T{...} (a gocpp pointer that owns it) or declare %s where it lives long enough",
					types.ExprString(un), x.Name(), why, x.Name()),
			})
		}
		return true
	})
}

// cCompositeValue reports whether e is a composite literal of a C/C++ type: C.T{...}.
func (ci *cinfo) compositeValue(e ast.Expr) bool {
	lit, ok := ast.Unparen(e).(*ast.CompositeLit)
	return ok && lit.Type != nil && ci.valueType(lit.Type)
}

// cLocalRoot returns the local C/C++ variable that the operand of & is part of: x, x.f, x[i] (of
// an array), or nil.
func cLocalRoot(info *types.Info, e ast.Expr, locals map[types.Object]bool) types.Object {
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.Ident:
			if obj := info.Uses[x]; obj != nil && locals[obj] {
				return obj
			}
			return nil
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		default:
			return nil
		}
	}
}

// A cppFunc is what the rules need of the function that holds an &x.
type cppFunc struct {
	info     *types.Info
	params   map[types.Object]bool // parameters, receiver and named results
	results  map[types.Object]bool // the named results
	assigned map[types.Object]int  // how often a local variable is assigned after its declaration
	returned map[types.Object]bool // local variables that a return statement returns by name
	// hasDefer: the function has a defer statement of its own. gocpp then holds a C/C++ local whose
	// address is stored in a variable declared outside its block until the function ends
	// (docs/spec/interop.md, "C/C++ objects in closures and defer"), so that store is safe.
	hasDefer bool
}

// keptWhere says where the value that contains un (an &x) is kept beyond x, or "" when it is
// not (or the analysis can't tell).
func (c *cppFunc) keptWhere(x types.Object, un *ast.UnaryExpr, stack []ast.Node) string {
	info := c.info
	// Climb through the expressions that carry the address along.
	var child ast.Node = un
	i := len(stack) - 1
climb:
	for ; i >= 0; i-- {
		switch p := stack[i].(type) {
		case *ast.ParenExpr:
		case *ast.KeyValueExpr:
			if p.Value != child {
				return ""
			}
		case *ast.CompositeLit:
			// an element of the literal
		case *ast.UnaryExpr:
			if p.Op != token.AND {
				return ""
			}
		case *ast.CallExpr:
			id, ok := ast.Unparen(p.Fun).(*ast.Ident)
			if !ok || id.Name != "append" {
				return ""
			}
			if _, ok := info.Uses[id].(*types.Builtin); !ok {
				return ""
			}
		default:
			break climb
		}
		child = stack[i]
	}
	if i < 0 {
		return ""
	}
	switch p := stack[i].(type) {
	case *ast.ReturnStmt:
		return "returns it"
	case *ast.SendStmt:
		if p.Value == child {
			if v := rootVar(info, p.Chan); v != nil {
				switch {
				case isPackageVar(v):
					return "sends it on the channel " + v.Name() + ", a package variable"
				case c.params[v]:
					return "sends it on the channel " + v.Name() + ", which the caller keeps"
				case outlives(v, x) && !c.hasDefer:
					return "sends it on " + v.Name() + ", which is declared outside the block of " + x.Name()
				}
			}
		}
	case *ast.AssignStmt:
		if len(p.Lhs) == len(p.Rhs) {
			for k, r := range p.Rhs {
				if r == child {
					return c.keptByTarget(x, p.Lhs[k], p.Tok == token.DEFINE)
				}
			}
		}
	case *ast.ValueSpec:
		if len(p.Names) == len(p.Values) {
			for k, v := range p.Values {
				if v == child {
					return c.keptByTarget(x, p.Names[k], true)
				}
			}
		}
	}
	return ""
}

// rootVar returns the variable at the root of e (x, x.f, x[i], *x), or nil.
func rootVar(info *types.Info, e ast.Expr) types.Object {
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.Ident:
			return info.Uses[x]
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		default:
			return nil
		}
	}
}

// keptByTarget classifies the target that the address of x is stored in.
func (c *cppFunc) keptByTarget(x types.Object, target ast.Expr, define bool) string {
	info := c.info
	target = ast.Unparen(target)
	if id, ok := target.(*ast.Ident); ok {
		var v types.Object
		if define {
			v = info.Defs[id]
		}
		if v == nil {
			v = info.Uses[id]
		}
		if v == nil || id.Name == "_" {
			return ""
		}
		switch {
		case isPackageVar(v):
			return "stores it in the package variable " + v.Name()
		case !define && c.results[v] && c.assigned[v] == 1: // assigned again, it may not hold the address at the return
			return "stores it in the result " + v.Name() + ", which the function returns"
		case !define && outlives(v, x) && !c.hasDefer:
			return "stores it in " + v.Name() + ", which is declared outside the block of " + x.Name()
		case define && c.returned[v] && c.assigned[v] == 0:
			return "returns it through " + v.Name()
		}
		return ""
	}
	// h.p, out[i], *p: the target is part of the variable at its root.
	deref := false
	for root := target; ; {
		switch t := ast.Unparen(root).(type) {
		case *ast.SelectorExpr:
			if tv, ok := info.Types[t.X]; ok && tv.Type != nil {
				if _, isPtr := tv.Type.Underlying().(*types.Pointer); isPtr {
					deref = true
				}
			}
			root = t.X
			continue
		case *ast.IndexExpr:
			if tv, ok := info.Types[t.X]; ok && tv.Type != nil {
				switch tv.Type.Underlying().(type) {
				case *types.Slice, *types.Map, *types.Pointer:
					deref = true
				}
			}
			root = t.X
			continue
		case *ast.StarExpr:
			deref = true
			root = t.X
			continue
		}
		break
	}
	v := rootVar(info, target)
	switch {
	case v == nil:
		return ""
	case isPackageVar(v):
		return "stores it in " + types.ExprString(target) + " of the package variable " + v.Name()
	case c.results[v]:
		return "stores it in " + types.ExprString(target) + " of the result " + v.Name() + ", which the function returns"
	case outlives(v, x) && !c.hasDefer:
		return "stores it in " + types.ExprString(target) + ", which is declared outside the block of " + x.Name()
	case c.params[v] && deref:
		return "stores it through the parameter " + v.Name() + " (" + types.ExprString(target) + "), which the caller keeps"
	}
	return ""
}

func isPackageVar(v types.Object) bool {
	return v.Pkg() != nil && v.Parent() == v.Pkg().Scope()
}

// outlives reports whether the variable v is declared in a block that contains the block of x.
func outlives(v, x types.Object) bool {
	if v.Parent() == nil || x.Parent() == nil || v.Parent() == x.Parent() {
		return false
	}
	for s := x.Parent().Parent(); s != nil; s = s.Parent() {
		if s == v.Parent() {
			return true
		}
	}
	return false
}

// --- cstr ---

// cstrAnalyzer finds uses of C.cstr and C.CString whose C string does not last as long as C needs
// it.
//
// C.cstr(s) gives a const char* that lives to the end of the statement (docs/spec/interop.md,
// "Strings at the boundary"); the compiler refuses to store it, but it can't see what C does with
// it. Reported:
//
//   - C.cstr(s) given to a function that keeps its pointer (the functions of the "keeps the
//     pointer" list of typed interop, internal/lower/keeps.go: openlog, getopt, InitWindow,
//     sqlite3_bind_text with a destructor other than SQLITE_TRANSIENT, ...): use C.CString, and
//     C.free it when C is done;
//   - C.cstr(s) inside a view that is kept (v := std.string_view(C.cstr(s)): the view
//     dangles at the next statement): write std.string_view(s), or std.string(s) for a copy;
//   - a string constant with a NUL byte in C.cstr or C.CString: the C string ends there.
var cstrAnalyzer = &analysis.Analyzer{
	Name:             "cstr",
	Doc:              "report C.cstr strings that do not last as long as C needs them (a function that keeps the pointer, a view that is kept) and constant strings with a NUL byte",
	RunDespiteErrors: true,
	Run:              runCstr,
}

// keeperFuncs and destructorArgs mirror internal/lower/keeps.go (Phase 12): the functions that keep
// the pointer of a string argument. Keep the two in step.
var keeperFuncs = map[string]string{
	"openlog":          "the logger keeps the identifier",
	"getopt":           "getopt keeps the option string",
	"getopt_long":      "getopt keeps the option string",
	"getopt_long_only": "getopt keeps the option string",
	"InitWindow":       "raylib keeps the title (it makes the window again from it)",
	"SetWindowTitle":   "raylib keeps the title",
}

var destructorArgs = map[string]int{
	"sqlite3_bind_text": 4, "sqlite3_bind_text16": 4, "sqlite3_bind_text64": 4,
	"sqlite3_bind_blob": 4, "sqlite3_bind_blob64": 4,
	"sqlite3_result_text": 3, "sqlite3_result_text16": 3, "sqlite3_result_text16le": 3,
	"sqlite3_result_text16be": 3, "sqlite3_result_text64": 3,
	"sqlite3_result_blob": 3, "sqlite3_result_blob64": 3,
}

var conventionWords = []string{"static", "borrow", "nocopy", "noalloc", "unowned"}

// keepsPointer is lower.keepsPointer: whether the call of the C function name with nargs arguments,
// whose argument n is text(n), lets the callee keep the pointer of a string argument.
func keepsPointer(name string, nargs int, text func(n int) string) (string, bool) {
	last := lastName(name)
	if why, ok := keeperFuncs[last]; ok {
		return why, true
	}
	if d, ok := destructorArgs[last]; ok {
		if d < nargs && !strings.HasSuffix(text(d), "SQLITE_TRANSIENT") {
			return "SQLite reads the bytes later, unless the destructor is SQLITE_TRANSIENT (it copies them)", true
		}
		return "", false
	}
	if last == "curl_easy_setopt" {
		if nargs > 1 && strings.HasSuffix(text(1), "CURLOPT_POSTFIELDS") {
			return "libcurl keeps the pointer of CURLOPT_POSTFIELDS (CURLOPT_COPYPOSTFIELDS copies)", true
		}
		return "", false
	}
	l := strings.ToLower(last)
	for _, w := range conventionWords {
		if strings.Contains(l, w) {
			return "its name says that it keeps the pointer", true
		}
	}
	return "", false
}

// viewTypes are the C++ types that look at characters they do not own.
var viewTypes = map[string]bool{
	"string_view": true, "wstring_view": true, "u8string_view": true, "u16string_view": true,
	"u32string_view": true, "basic_string_view": true, "span": true,
}

// cstrCall returns the helper ("cstr", "wstr" or "CString", "WString") that call calls, or "".
func cgoHelperCall(info *types.Info, call *ast.CallExpr) string {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	return check.CgoHelperOf(sel, info)
}

func runCstr(pass *analysis.Pass) (any, error) {
	info := pass.TypesInfo
	ci := newCinfo(pass)
	for _, f := range pass.Files {
		walkStack(f, func(n ast.Node, stack []ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch h := cgoHelperCall(info, call); h {
			case "cstr", "CString":
				if len(call.Args) == 1 {
					if tv := info.Types[call.Args[0]]; tv.Value != nil && tv.Value.Kind() == constant.String {
						if s := constant.StringVal(tv.Value); strings.IndexByte(s, 0) >= 0 {
							pass.Report(analysis.Diagnostic{
								Pos: call.Pos(), End: call.End(), Category: "cstr",
								Message: fmt.Sprintf("the string given to C.%s has a NUL byte (at %d of %d): a C string ends at the first NUL, so C sees only %q; pass the bytes some other way if C needs all of them", h, strings.IndexByte(s, 0), len(s), s[:strings.IndexByte(s, 0)]),
							})
						}
					}
				}
				return true
			}
			name, isC := ci.name(call.Fun)
			if !isC {
				return true
			}
			for _, a := range call.Args {
				ac, ok := ast.Unparen(a).(*ast.CallExpr)
				if !ok || cgoHelperCall(info, ac) != "cstr" {
					continue
				}
				if why, keeps := keepsPointer(name, len(call.Args), func(n int) string { return types.ExprString(call.Args[n]) }); keeps {
					pass.Report(analysis.Diagnostic{
						Pos: ac.Pos(), End: ac.End(), Category: "cstr",
						Message: fmt.Sprintf("C.cstr gives %s a string that lives only to the end of the statement, but %s: C reads freed memory later; use C.CString(s) and C.free it when %s is done with it", lastName(name), why, lastName(name)),
					})
				}
			}
			if viewTypes[lastName(name)] && storedInStack(stack) {
				for _, a := range call.Args {
					if ac, ok := ast.Unparen(a).(*ast.CallExpr); ok && cgoHelperCall(info, ac) == "cstr" {
						pass.Report(analysis.Diagnostic{
							Pos: call.Pos(), End: call.End(), Category: "cstr",
							Message: fmt.Sprintf("%s(C.cstr(...)) is a view of the C string, which ends with the statement, but the view is kept: the next use reads freed memory; write %s(s) over the Go string, or std.string(s) for a copy", name, name),
						})
					}
				}
			}
			return true
		})
	}
	return nil, nil
}

// storedInStack reports whether the value of the innermost expression of the stack is stored: it is
// the value of an assignment or declaration, a result, a field of a literal, or sent.
func storedInStack(stack []ast.Node) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		switch stack[i].(type) {
		case *ast.ParenExpr:
			continue
		case *ast.AssignStmt, *ast.ValueSpec, *ast.ReturnStmt, *ast.KeyValueExpr, *ast.CompositeLit, *ast.SendStmt:
			return true
		}
		return false
	}
	return false
}

// --- narrowing ---

// narrowingAnalyzer finds conversions to a C number type that cut the value.
//
// C.int(n) with n an int is Go's explicit conversion: it keeps the low 32 bits, and a larger value
// changes silently. Typed interop (Phase 12) accepts the same cut implicitly where C++ does
// (docs/spec/typed-interop.md: "narrowing conversions get a gocpp vet warning instead of an
// error"); that part waits for the typed mode to be the default, because it needs the types of the
// C++ parameters. What this analyzer checks needs only the width of the C type that the user
// names: a conversion C.T(x) to a C integer type (char, short, int, long long, their unsigned forms
// and the int<N>_t types) from a gocpp integer with more bits, or C.float(x) from a float64. Not
// reported: a constant (the compiler checks it), a plain int or uint converted to a type of 32 bits
// or more (C.int(n) is how every int goes to C; a sized type like int64 says that the value can be
// large, and an int cut to 8 or 16 bits is reported), len(...) and cap(...) (they stay below 2^31
// in every program that C can handle), a value masked or reduced to the range of the type
// (x & 0xff, x % 100), C.long and C.ulong (their width depends on the target).
var narrowingAnalyzer = &analysis.Analyzer{
	Name:             "narrowing",
	Doc:              "report a conversion to a C number type that cuts the value (C.int(n) with n an int, C.float(x) with x a float64)",
	RunDespiteErrors: true,
	Run:              runNarrowing,
}

// cWidths are the widths in bits of the C integer types that are the same on every target gocpp
// builds for (char is 8 bits, int 32, long long 64).
var cWidths = map[string]int{
	"char": 8, "schar": 8, "uchar": 8, "short": 16, "ushort": 16, "int": 32, "uint": 32,
	"longlong": 64, "ulonglong": 64,
	"int8_t": 8, "uint8_t": 8, "int16_t": 16, "uint16_t": 16, "int32_t": 32, "uint32_t": 32,
	"int64_t": 64, "uint64_t": 64,
}

func runNarrowing(pass *analysis.Pass) (any, error) {
	info := pass.TypesInfo
	ci := newCinfo(pass)
	sizes := pass.TypesSizes
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 || call.Ellipsis.IsValid() {
				return true
			}
			name, isC := ci.name(call.Fun)
			if !isC || strings.Contains(name, ".") {
				return true
			}
			spelled := types.ExprString(call.Fun) // as the user wrote it: C.int, or the name that the package gave it
			arg := call.Args[0]
			tv := info.Types[arg]
			if tv.Type == nil || tv.Value != nil || tv.IsType() {
				return true
			}
			basic, ok := tv.Type.Underlying().(*types.Basic)
			if !ok || basic.Info()&types.IsUntyped != 0 {
				return true // an untyped value has no size
			}
			if boundedConversion(info, arg, cWidths[name]) {
				return true
			}
			switch {
			case name == "float" && basic.Kind() == types.Float64:
				pass.Report(analysis.Diagnostic{
					Pos: call.Pos(), End: call.End(), Category: "narrowing",
					Message: fmt.Sprintf("%s(%s) narrows a float64 (64 bits) to %s (32 bits): the value is rounded and a large one becomes infinity; keep the double (C.double) if the precision matters", spelled, types.ExprString(arg), spelled),
				})
			case basic.Info()&types.IsInteger != 0 && cWidths[name] > 0:
				bits := int(sizes.Sizeof(basic)) * 8
				if (basic.Kind() == types.Int || basic.Kind() == types.Uint) && cWidths[name] >= 32 {
					return true // int and uint are the plain integers of Go: C.int(n) is how they go to C
				}
				if bits > cWidths[name] {
					pass.Report(analysis.Diagnostic{
						Pos: call.Pos(), End: call.End(), Category: "narrowing",
						Message: fmt.Sprintf("%s(%s) narrows %s (%d bits) to %s (%d bits): a value that does not fit is silently cut; check the range first, or mask it (& 0xff) if the cut is intended",
							spelled, types.ExprString(arg), tv.Type, bits, spelled, cWidths[name]),
					})
				}
			}
			return true
		})
	}
	return nil, nil
}

// boundedConversion reports whether the value of e is known to fit in bits: len(x) and cap(x), a
// value masked with a constant that fits (x & 0xff), or reduced by a constant remainder (x % 100).
func boundedConversion(info *types.Info, e ast.Expr, bits int) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.CallExpr:
		if id, ok := ast.Unparen(x.Fun).(*ast.Ident); ok && (id.Name == "len" || id.Name == "cap") {
			_, builtin := info.Uses[id].(*types.Builtin)
			return builtin
		}
	case *ast.BinaryExpr:
		fits := func(o ast.Expr) bool {
			tv := info.Types[o]
			if tv.Value == nil || tv.Value.Kind() != constant.Int || bits == 0 {
				return false
			}
			v := constant.ToInt(tv.Value)
			limit := constant.Shift(constant.MakeInt64(1), token.SHL, uint(bits))
			return constant.Sign(v) >= 0 && constant.Compare(v, token.LSS, limit)
		}
		switch x.Op {
		case token.AND:
			return fits(x.X) || fits(x.Y)
		case token.REM:
			return fits(x.Y)
		}
	}
	return false
}
