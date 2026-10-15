package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/load"
)

// The C++ names of the C/C++ names of the source (Phase 04 README §2.3, step 4.3):
//
//	cstdio.printf, ctime.time (header packages)   ::printf, ::time (fully qualified: a gocpp
//	                                              name can't hide them)
//	cstdio.EOF, cerrno.errno (macros)             EOF, errno (macros work only unqualified)
//	std.vector, fs.path (namespaces, renames)     ::std::vector, ::std::filesystem::path
//	glm.vec3 (a header package and a namespace)   ::glm::vec3 (the namespace first, then the
//	                                              C-level name: ::vec3)
//	std.chrono.steady_clock.now, std.string.npos  ::std::chrono::steady_clock::now,
//	(a type before the name)                      ::std::string::npos
//	std.cout.flush (a value before the name)      ::std::cout.flush
//	stat.struct_stat, lib.enum_x, lib.union_y     struct ::stat, enum ::x, union ::y (cgo's tags)
//	std.vector[int].iterator                      ::std::vector<int64_t>::iterator
//	C.int, C.ulonglong, C.size_t                  int, unsigned long long, size_t
//
// The kind probe (interop.go) says what each prefix of a path is: after a namespace or a type
// comes ::, after a value comes . (a member of an object). A name that the probe found nothing
// for is the user's error (a typo, or a header that isn't imported), reported here, at its
// place: the check can't know it. A parenthesized value, (lib.x).m, is not a path: a member of
// it is a member access on a C/C++ value, which comes with gp::deref and the parenthesis escape
// of §6 (steps 4.4 and 4.5).
//
// Every name of a C/C++ name that lowering writes is kept (cNames), so that the macro
// protection (macros.go) never #undefs a macro that the code uses through a header package.

// cBuiltins are the C types of C (cgo's names) and their C++ spelling.
var cBuiltins = map[string]string{
	"char":      "char",
	"schar":     "signed char",
	"uchar":     "unsigned char",
	"short":     "short",
	"ushort":    "unsigned short",
	"int":       "int",
	"uint":      "unsigned int",
	"long":      "long",
	"ulong":     "unsigned long",
	"longlong":  "long long",
	"ulonglong": "unsigned long long",
	"float":     "float",
	"double":    "double",
	"size_t":    "size_t",
}

// A cRef is the C++ of a C/C++ name of the source.
type cRef struct {
	// x is the C++: a *cppir.Name (::std::vector, EOF, ::std::vector<int64_t>, struct ::stat), or
	// a *cppir.Member for a member of a value (::std::cout.flush).
	x cppir.Expr

	// kind is what it names: Namespace, Type, Template, Value, ValueTemplate or Macro, as the
	// kind probe says. An instance of a template (std.vector[int]) is a Type, a member of a
	// value or an instance of a value template is a Value, and a member of a type after
	// template arguments (std.vector[int].iterator) is Unknown: a nested type or a static
	// member, both reached with ::.
	kind interop.Kind

	// path is the path that the kind probe knows the name by (std::chrono::steady_clock), or
	// "" after template arguments or a member of a value: then nothing after it is probed.
	path string

	// kinds are the kind probe's answers for the file of the name.
	kinds map[string]interop.Kind

	// src is the name as the user wrote it, for messages: std.chrono.steady_clock.
	src string
}

// isValue reports whether the name is a value, whose members are reached with . (a macro is
// taken as one: RAYWHITE.r).
func (r cRef) isValue() bool {
	return r.kind == interop.Value || r.kind == interop.ValueTemplate || r.kind == interop.Macro
}

// cUse returns the use of a header package or a namespace whose selector is sel (C.printf,
// C.std.vector after the loader's rewrite), or nil.
func (g *gen) cUse(sel *ast.SelectorExpr) *load.CppUse {
	if g.cuses == nil {
		g.cuses = make(map[*ast.SelectorExpr]*load.CppUse)
		g.useKinds = make(map[*load.CppUse]map[string]interop.Kind)
		for _, f := range g.pkg.Load.Files {
			if f.Rewrite == nil {
				continue
			}
			for _, u := range f.Rewrite.Uses {
				if (u.Kind == load.HeaderUse || u.Kind == load.NamespaceUse) && !u.Bare {
					g.cuses[u.Sel] = u
					if g.interop != nil {
						g.useKinds[u] = g.interop.kinds[f]
					}
				}
			}
		}
	}
	return g.cuses[sel]
}

// isC reports whether the name x is the package C (C in C.int, and the C that the loader's
// rewrite put in C.printf).
func (g *gen) isC(x ast.Expr) bool {
	id, ok := x.(*ast.Ident)
	if !ok {
		return false
	}
	pn, ok := g.info.Uses[id].(*types.PkgName)
	return ok && pn.Imported().Path() == "C"
}

// cRefOf returns the C++ of e if e is a C/C++ name (a selector chain on a header package, a
// namespace or C, template arguments after one, or a member after those), and false if it is
// not one. It reports an error, and returns a placeholder, for a name that the headers don't
// declare.
func (g *gen) cRefOf(e ast.Expr) (cRef, bool) {
	switch x := e.(type) {
	case *ast.SelectorExpr:
		if u := g.cUse(x); u != nil {
			return g.cBase(u, x.Sel), true
		}
		if g.isC(x.X) {
			return g.cBuiltin(x), true
		}
		r, ok := g.cRefOf(x.X)
		if !ok {
			return cRef{}, false
		}
		return g.cMember(r, x.Sel), true
	case *ast.IndexExpr, *ast.IndexListExpr:
		ix := unpackIndex(x)
		r, ok := g.cRefOf(ix.x)
		if !ok {
			return cRef{}, false
		}
		switch r.kind {
		case interop.Template, interop.ValueTemplate:
		default:
			// An index of a C/C++ value (lib.arr[3]) is an operator: step 4.4.
			return cRef{}, false
		}
		name, ok := r.x.(*cppir.Name)
		if !ok || name.Args != nil {
			return cRef{}, false
		}
		args := make([]cppir.Node, len(ix.indices))
		for i, a := range ix.indices {
			args[i] = g.templateArg(a)
		}
		out := cRef{x: g.cNode(&cppir.Name{Name: name.Name, Args: args}).(*cppir.Name), kind: interop.Type, kinds: r.kinds, src: r.src + "[...]"}
		if r.kind == interop.ValueTemplate {
			out.kind = interop.Value
		}
		return out, true
	}
	return cRef{}, false
}

// An index is an index expression with one or more indices: x[a] or x[a, b].
type index struct {
	x       ast.Expr
	indices []ast.Expr
}

func unpackIndex(e ast.Expr) index {
	switch x := e.(type) {
	case *ast.IndexExpr:
		return index{x.X, []ast.Expr{x.Index}}
	case *ast.IndexListExpr:
		return index{x.X, x.Indices}
	}
	return index{}
}

// cBuiltin returns the C++ of C.x, a name that the user wrote on C: a C type (C.int).
func (g *gen) cBuiltin(sel *ast.SelectorExpr) cRef {
	if s, ok := cBuiltins[sel.Sel.Name]; ok {
		return cRef{x: g.cName(s, sel.Sel.Pos()), kind: interop.Type, src: "C." + sel.Sel.Name}
	}
	g.unsupported(sel.Pos(), "C."+sel.Sel.Name+" (C has only C's built-in types and the helpers; the helpers come in steps 4.9 and 4.16)")
	return cRef{x: placeholder, kind: interop.Value, src: "C." + sel.Sel.Name}
}

// cBase returns the C++ of the first name after the use u of a header package or a
// namespace: printf in cstdio.printf, vector in std.vector, path in fs.path.
func (g *gen) cBase(u *load.CppUse, name *ast.Ident) cRef {
	kinds := g.useKinds[u]
	if g.interop == nil {
		g.errorf(name.Pos(), "internal error: C/C++ names without the compiler probes (lower.ProbeInterop)")
		return cRef{x: placeholder, kind: interop.Value}
	}
	src := u.Ident.Name + "." + name.Name
	if u.Kind == load.NamespaceUse {
		ns := strings.TrimPrefix(u.Namespace, "::")
		return g.cStep(cRef{kind: interop.Namespace, path: ns, kinds: kinds, src: u.Ident.Name, x: g.cName("::"+ns, u.Ident.Pos())}, name, src)
	}
	// A header package: a macro, then the namespace of its name (glm.vec3 → ::glm::vec3), then
	// the C-level name.
	hdr := u.Ident.Name
	if k := kinds[hdr+"::"+name.Name]; kinds[hdr] == interop.Namespace && kinds[name.Name] != interop.Macro && found(k) && k != interop.Macro {
		return g.cStep(cRef{kind: interop.Namespace, path: hdr, kinds: kinds, src: hdr, x: g.cName("::"+hdr, u.Ident.Pos())}, name, src)
	}
	if tag, ok := tagName(name.Name); ok {
		return cRef{x: g.cName(tag, name.Pos()), kind: interop.Type, kinds: kinds, src: src}
	}
	switch k := kinds[name.Name]; k {
	case interop.Macro:
		return cRef{x: g.cName(name.Name, name.Pos()), kind: k, kinds: kinds, src: src}
	default:
		if !g.knownKind(k, name, src, "::"+name.Name) {
			return cRef{x: placeholder, kind: interop.Value, src: src}
		}
		return cRef{x: g.cName("::"+name.Name, name.Pos()), kind: k, path: name.Name, kinds: kinds, src: src}
	}
}

// found reports whether the kind probe found something for a path.
func found(k interop.Kind) bool {
	return k != interop.NotFound && k != interop.Unknown && k != interop.NotNamespace
}

// cMember returns the C++ of the name sel after r: with :: after a namespace or a type, with
// . after a value.
func (g *gen) cMember(r cRef, sel *ast.Ident) cRef {
	src := r.src + "." + sel.Name
	if r.isValue() {
		// A member of a value: gp::deref passes an object through and dereferences a raw
		// pointer with Go's nil check (gocpp/interop.hpp), as Go's x.m does.
		g.noteCName(sel.Name, sel.Pos())
		m := &cppir.Member{X: call("gp::deref", r.x), Name: sel.Name}
		return cRef{x: g.cNode(m).(*cppir.Member), kind: interop.Value, kinds: r.kinds, src: src}
	}
	if r.path == "" {
		if r.kind == interop.Unknown {
			// After a member of a type that comes after template arguments
			// (std.vector[int].iterator.x): the probe can't tell whether it is a type or a value.
			g.unsupported(sel.Pos(), "a member of "+r.src+", which the kind probe can't tell a type or a value (step 4.5)")
			return cRef{x: placeholder, kind: interop.Value, src: src}
		}
		// After template arguments: a nested type or a static member, both with ::.
		return cRef{x: g.cName(cppir.ExprString(r.x)+"::"+sel.Name, sel.Pos()), kind: interop.Unknown, kinds: r.kinds, src: src}
	}
	return g.cStep(r, sel, src)
}

// cStep returns the C++ of the name sel after the namespace or type r, whose kind the probe
// knows by r's path.
func (g *gen) cStep(r cRef, sel *ast.Ident, src string) cRef {
	if r.kind == interop.Template {
		g.userError(sel.Pos(), "%s is a C++ template: give it its template arguments before .%s (%s[T].%s)", r.src, sel.Name, r.src, sel.Name)
		return cRef{x: placeholder, kind: interop.Value, src: src}
	}
	path := r.path + "::" + sel.Name
	k := r.kinds[path]
	if !g.knownKind(k, sel, src, "::"+path) {
		return cRef{x: placeholder, kind: interop.Value, src: src}
	}
	return cRef{x: g.cName(cppir.ExprString(r.x)+"::"+sel.Name, sel.Pos()), kind: k, path: path, kinds: r.kinds, src: src}
}

// knownKind reports whether k, the kind of the C++ path cxx that the user wrote as src, is
// something that lowering can translate, and reports the error if it isn't: nothing of that
// name is declared (NotFound), or the probe had no answer (Unknown).
func (g *gen) knownKind(k interop.Kind, at *ast.Ident, src, cxx string) bool {
	switch k {
	case interop.NotFound, interop.NotNamespace:
		g.userError(at.Pos(), "undefined: %s (the imported headers declare no %s)", src, cxx)
		return false
	case interop.Unknown:
		g.errorf(at.Pos(), "internal error: the compiler probe has no answer for %s (%s)", src, cxx)
		return false
	}
	return true
}

// cName returns the C++ name s of a C/C++ name of the source at pos, and records it for the
// macro protection (macros.go: cNodes, cNames).
func (g *gen) cName(s string, pos token.Pos) *cppir.Name {
	g.noteCName(s, pos)
	return g.cNode(cxxName(s)).(*cppir.Name)
}

// cNode records the node of a C/C++ name (a *cppir.Name, *cppir.TypeName or *cppir.Member) for
// the macro protection, which prints the code without them, and returns it.
func (g *gen) cNode(n cppir.Node) cppir.Node {
	g.cNodes = append(g.cNodes, n)
	return n
}

// noteCName records the names of a C/C++ name that lowering writes (macros.go), with the place of
// the first use of each.
func (g *gen) noteCName(s string, pos token.Pos) {
	if g.cNames == nil {
		g.cNames = make(map[string]token.Pos)
	}
	for _, w := range strings.FieldsFunc(s, func(r rune) bool { return !isIdentRune(r) }) {
		if _, ok := g.cNames[w]; !ok {
			g.cNames[w] = pos
		}
	}
}

// isIdentRune reports whether r can be in a C++ identifier (ASCII only: gocpp's and C's names).
func isIdentRune(r rune) bool {
	return r == '_' || 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9'
}

// userError records an error of the user's code at pos, if it is the first (Error.User).
func (g *gen) userError(pos token.Pos, format string, args ...any) {
	if g.err != nil {
		return
	}
	g.errorf(pos, format, args...)
	g.err.User = true
}
