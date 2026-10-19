package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/typedinterop"
	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// Typed interop in the lowering (Phase 12 step 12.6, ADR-087).
//
// Opaque mode (Phase 04) writes every C/C++ expression almost as it is and lets the C++ compiler
// decide: gp::deref(x).m for a member (the runtime picks `.` or `->` by the type), assignments
// for a keyed literal, gp::cstr(s) only where the user asks. Typed mode has the type of each
// C/C++ expression from clang (the probe, steps 12.2 to 12.4), and uses it where it is certain:
//
//	x.m, x a class or a reference              x.m                         (typedMember)
//	x.m, x a raw pointer                       gp::nn(x)->m                (Go's nil check, as gp::deref's)
//	lib.Rec{b: 2, a: 1}                        ::Rec{.a = 1, .b = 2}       (typedLit: declaration order)
//	cstdio.puts(s), s a Go string              ::puts(gp::cstr(s).gp_ptr()) (typedArgs; the "keeps the
//	                                           pointer" functions stay explicit: keeps.go)
//	v.at(0) = 5                                accepted: a T& result is an lvalue (typedAssignable)
//	var n int = cstring.strlen(s)              accepted: C++'s conversions (typedConvertible)
//
// Every rule is a refinement of the opaque lowering: where the probe has no answer for an
// expression (not probed, failed, recovered, an incomplete type, a cross build with headers that
// are not the standard library's), the opaque lowering runs unchanged. The mode is off unless the
// driver gives the package's Interop a Report (Interop.SetTyped); the default output is
// byte-identical. The compatibility rule of the phase holds: typed mode never rejects what
// opaque mode accepts, and it accepts more where C++ does.

// SetTyped gives the Interop the typed information of its package: the report of
// typedinterop.Analyze with Options.Decls (the references, the probe's table, the declarations
// and the foreign view). cross says that the program is built for another system than this one,
// so the headers that the probes read are not the target's.
func (in *Interop) SetTyped(rep *typedinterop.Report, cross bool) {
	if in == nil || rep == nil || rep.Table == nil || rep.Mapper == nil || rep.Set == nil {
		return
	}
	if len(rep.IncludeErrors) > 0 || len(rep.Table.HeaderErrors) > 0 {
		return // the headers do not compile for the probe: nothing it says is certain
	}
	in.typed = &typedInfo{rep: rep, cross: cross}
}

// A typedInfo is the typed information of one package.
type typedInfo struct {
	rep   *typedinterop.Report
	cross bool
}

// typedOn reports whether typed lowering is on for the package.
func (g *gen) typedOn() bool {
	return g.interop != nil && g.interop.typed != nil && !g.inShared0()
}

// inShared0 is true while a shared instance is being made: its AST nodes are copies that the
// collector never saw.
func (g *gen) inShared0() bool { return g.inShared > 0 || g.infoPrivate }

// A tval is the typed view of one expression.
type tval struct {
	ftype.Value
	ref *probe.Result
}

// tvalOf returns what the probe says about the expression e (the node the collector made a
// reference of), or nil: not typed, not probed, failed, recovered (clang printed an error for
// it), or opaque in the foreign view.
func (g *gen) tvalOf(e ast.Expr) *tval {
	if !g.typedOn() {
		return nil
	}
	t := g.interop.typed
	ref := t.rep.Set.RefOf(e)
	if ref == nil {
		if p, ok := e.(*ast.ParenExpr); ok {
			return g.tvalOf(p.X)
		}
		return nil
	}
	r := t.rep.Table.Result(ref.ID)
	if r == nil || !r.OK || r.Recovered != "" || ref.ID >= len(t.rep.Values) {
		return nil
	}
	v := t.rep.Values[ref.ID]
	if v.Kind == ftype.Opaque || v.C == nil {
		return nil
	}
	return &tval{Value: v, ref: r}
}

// cTypeOf returns the C++ type of the typed value without its reference, or nil.
func (v *tval) cType() *ctype.Type {
	if v == nil || v.C == nil {
		return nil
	}
	return v.C.StripRef()
}

// typedMember returns the C++ of the member access sel = x.name, where x is the C++ of sel.X,
// from the C++ type of sel.X, or nil when the type is not certain (the caller writes
// gp::deref(x).name, which the runtime decides by the type, cMemberOf):
//
//   - a class, struct or union (also by reference, and a smart pointer or an iterator, whose own
//     members stay reachable as in opaque mode): x.name;
//   - a raw pointer to one: gp::nonnull(x)->name, Go's nil check and then C++'s ->;
//   - a member that the class does not have and the pointee of its operator-> does: x->name
//     (gp::smartnn(x)->name for a smart pointer or an optional, which checks for nil). Only when
//     the compiler said so: the probe of the access failed with "no member named name"
//     (noMemberFailure). The extracted members miss using-declarations and enumerators, so they
//     cannot prove that the class lacks a name; the compiler's answer can.
//
// A pointer to a pointer, to void or to a function, and the types of the runtime's own (a
// gocpp pointer that a template returns) are left to gp::deref.
func (fg *funcGen) typedMemberOf(sel *ast.SelectorExpr, x cppir.Expr) cppir.Expr {
	if fg.thunk > 0 {
		// In a deferred or spawned call the object is what the frame stored (funcGen.thunk): a
		// pointer to it, or a copy. gp::deref gives the object in both cases, and only the member
		// that the smart pointer's pointee has needs the arrow.
		if v := fg.g.tvalOf(fg.g.memberObject(sel)); v != nil && v.Kind == ftype.Record && !isRuntimeType(v.cType()) && fg.g.arrowMember(sel, v) {
			fg.g.noteCName(sel.Sel.Name, sel.Sel.Pos())
			return fg.g.cNode(&cppir.Member{X: call("gp::smartnn", call("gp::deref", x)), Arrow: true, Name: sel.Sel.Name}).(*cppir.Member)
		}
		return nil
	}
	return fg.g.typedMember(sel, x)
}

func (g *gen) typedMember(sel *ast.SelectorExpr, x cppir.Expr) cppir.Expr {
	name := sel.Sel
	v := g.tvalOf(g.memberObject(sel))
	if v == nil {
		return nil
	}
	g.noteCName(name.Name, name.Pos())
	switch {
	case v.Kind == ftype.Record && !isRuntimeType(v.cType()):
		if g.arrowMember(sel, v) {
			return g.cNode(&cppir.Member{X: call("gp::smartnn", x), Arrow: true, Name: name.Name}).(*cppir.Member)
		}
		return g.cNode(&cppir.Member{X: x, Name: name.Name}).(*cppir.Member)
	case v.Kind == ftype.Pointer && v.Elem != nil && v.Elem.Kind == ftype.Record && !isRuntimeType(v.Elem.C):
		return g.cNode(&cppir.Member{X: call("gp::nonnull", x), Arrow: true, Name: name.Name}).(*cppir.Member)
	}
	return nil
}

// memberObject returns the expression whose typed view is the object of the member access sel: sel.X,
// or, for a member that a struct takes from an embedded C++ class (check/promoted.go,
// cppmembers.go), the name sel.Sel, which the collector made the reference of the embedded field's
// class of (collect/members.go). The C++ of the object is the path of the embedded fields
// (fieldSteps), so its type is that of the field, not sel.X's.
func (g *gen) memberObject(sel *ast.SelectorExpr) ast.Expr {
	if g.typedOn() && g.pkg.PromotedPath(sel) != nil && g.interop.typed.rep.Set.RefOf(sel.Sel) != nil {
		return sel.Sel
	}
	return sel.X
}

// arrowMember reports whether the member access sel on the class value v is x->name: the compiler
// has no member of that name in the class (noMemberFailure) and the pointee of the class's
// operator-> has one (ftype.Mapper.ArrowMember).
func (g *gen) arrowMember(sel *ast.SelectorExpr, v *tval) bool {
	return g.noMemberFailure(sel) && g.interop.typed.rep.Mapper.ArrowMember(v.Type, sel.Sel.Name)
}

// noMemberRe matches clang's "no member named 'm' in 'T'" (with the hint that clang may add).
var noMemberRe = regexp.MustCompile(`^no member named '([^']+)' in '[^']+'(?:;.*)?$`)

// noMemberFailure reports whether the probe of the expression that holds the access sel failed with
// "no member named <name>" (or recovered from that error): the call that has sel as its function, else sel itself. That is the
// compiler saying that the class of sel.X has no member of that name, whatever the extracted
// declarations know.
func (g *gen) noMemberFailure(sel *ast.SelectorExpr) bool {
	var e ast.Expr = sel
	for p := g.parentOf(sel); p != nil; p = g.parentOf(p) {
		if pe, ok := p.(*ast.ParenExpr); ok {
			e = pe
			continue
		}
		if c, ok := p.(*ast.CallExpr); ok && ast.Unparen(c.Fun) == ast.Expr(sel) {
			e = c
		}
		break
	}
	t := g.interop.typed
	ref := t.rep.Set.RefOf(e)
	if ref == nil {
		return false
	}
	r := t.rep.Table.Result(ref.ID)
	if r == nil || r.Skipped != "" {
		return false
	}
	// Clang recovers from "no member named m in T; did you mean to use '->'": the probe has a type
	// and Recovered has the error. Otherwise the probe failed and Error has it.
	msg := r.Error
	if r.OK {
		msg = r.Recovered
	}
	m := noMemberRe.FindStringSubmatch(msg)
	return m != nil && m[1] == sel.Sel.Name
}

// isRuntimeType reports whether t is a type of gocpp's runtime (gp::ptr, gp::string): those have
// no C++ members of the program's.
func isRuntimeType(t *ctype.Type) bool {
	l := t.Leaf()
	return l == nil || l.Kind != ctype.Named || len(l.Path) == 0 || l.Path[0].Name == "gp" || l.Path[0].Name == "gocpp_probe_internal_pr"
}

// Conversions where C++'s rules apply (README §2.3, the compatibility rule). Opaque mode asks the
// user for an explicit conversion wherever a C/C++ number meets a gocpp one (the rules of cgo:
// cguard.go); typed mode knows the C++ type of the C/C++ side, accepts what C++ converts, and writes
// the conversion itself, gp::cgo::as<T>(x), which is Go's T(x) (defined for every value, where a
// C++ cast of a float out of range is not):
//
//	var n int = cstring.strlen(s)     int64_t n = gp::cgo::as<int64_t>(::strlen(s))
//	var c C.int = n                   int c = gp::cgo::as<int>(n)
//	cstring.strlen(s) + len(s)        ::strlen(s) + gp::len(s)          (C++'s usual arithmetic conversions)
//	it + n                            it + n                            (pointer and iterator arithmetic)
//
// The narrowing that this accepts (a size_t into an int, an int64 into a char) is what a C++
// program does silently; `gocpp vet` (Phase 13) warns about it.

// numericGo reports whether t is a predeclared Go number (not complex), the types that a
// conversion can name without a named type of the program.
func numericGo(t types.Type) bool {
	b, ok := t.(*types.Basic)
	return ok && b.Info()&types.IsNumeric != 0 && b.Info()&types.IsComplex == 0
}

// typedScalarOK reports whether the C/C++ value e, going to a Go variable, parameter or result of
// the type to, is accepted by C++'s conversion rules in typed mode: a C++ arithmetic value (or an
// enum that converts implicitly) into a Go number, a C++ bool into bool. It records the conversion
// that the lowering writes.
func (g *gen) typedScalarOK(e ast.Expr, to types.Type) bool {
	v := g.tvalOf(e)
	if v == nil || !(v.Kind == ftype.Scalar || v.Kind == ftype.Enum && !v.Scoped) {
		return false
	}
	switch b, ok := to.(*types.Basic); {
	case ok && b.Kind() == types.Bool:
		return v.Kind == ftype.Scalar && v.C != nil && v.C.StripRef().Kind == ctype.Builtin && v.C.StripRef().Name == "bool"
	case numericGo(to):
		if v.Kind == ftype.Scalar && sameScalar(v.Type, to) {
			return true
		}
		g.noteTypedConv(e, g.cxxType(to, e.Pos()))
		return true
	}
	return false
}

// typedNumberToC reports whether the gocpp number e, going to the C/C++ type written te (C.int, C.double),
// is accepted by C++'s conversion rules in typed mode, and records the conversion.
func (g *gen) typedNumberToC(e ast.Expr, te ast.Expr) bool {
	if !g.typedOn() || te == nil {
		return false
	}
	sel, ok := ast.Unparen(te).(*ast.SelectorExpr)
	if !ok || !g.isC(sel.X) {
		return false
	}
	cpp, ok := cBuiltins[sel.Sel.Name]
	if !ok {
		return false
	}
	t := g.info.TypeOf(e)
	if t == nil || !complete(t) || !numericGo(t) {
		return false
	}
	g.noteTypedConv(e, typeName(cpp))
	return true
}

// noteTypedConv records that the value e is converted to the C++ type to when it is lowered (fg.expr).
func (g *gen) noteTypedConv(e ast.Expr, to cppir.Type) {
	if g.tconv == nil {
		g.tconv = make(map[ast.Expr]cppir.Type)
	}
	g.tconv[e] = to
}

// typedConvert returns x, the C++ of e, converted as noteTypedConv recorded.
func (fg *funcGen) typedConvert(e ast.Expr, x cppir.Expr) cppir.Expr {
	if to, ok := fg.g.tconv[e]; ok {
		return templateCall("gp::cgo::as", []cppir.Node{to}, x)
	}
	return x
}

// typedOperandsOK reports whether the operator op between the gocpp number typed and the C/C++ value
// cv (the other operand) is C++'s to compute in typed mode: a C++ number or enum (the usual
// arithmetic conversions), or a pointer or iterator with + and - (pointer arithmetic).
func (g *gen) typedOperandsOK(op token.Token, typed, cv ast.Expr) bool {
	if !basicNumeric(g.info.TypeOf(typed)) {
		return false
	}
	v := g.tvalOf(cv)
	if v == nil {
		return false
	}
	switch v.Kind {
	case ftype.Scalar:
		return true
	case ftype.Enum:
		return !v.Scoped
	case ftype.Pointer, ftype.Record:
		return op == token.ADD || op == token.SUB
	}
	return false
}

// basicNumeric reports whether t is a number of a (possibly named) basic type.
func basicNumeric(t types.Type) bool {
	b := underBasic(t)
	return b != nil && b.Info()&types.IsNumeric != 0 && b.Info()&types.IsComplex == 0
}

// typedIndexKey returns cx, the C++ of the index of y = x[k], converted to the key of the container
// when k is a gocpp string and every operator[] of x takes a std::string (m["k"] with a constant
// is a C literal already): m[key] of a std.map[std.string, int].
func (fg *funcGen) typedIndexKey(y *ast.IndexExpr, cx cppir.Expr) cppir.Expr {
	g := fg.g
	if !g.typedOn() || !g.goStringValue(y.Index) {
		return cx
	}
	sigs, ok := g.methodSigs(y.X, "operator[]")
	if !ok {
		return cx
	}
	if c := sigConvs(sigs, 1); c[0] == convStdString {
		return &cppir.Call{Fun: cxxName("::std::string"), Args: []cppir.Expr{cx}}
	}
	return cx
}

// typedStar returns the C++ of *x, x a C/C++ value, from its C++ type: *gp::nonnull(p) for a raw
// pointer to an object (Go's nil check, then C++'s *), *x for an iterator, a smart pointer or any other
// class with operator*; nil when the type is not certain (gp::cgo::star decides by the type).
func (fg *funcGen) typedStar(x *ast.StarExpr) cppir.Expr {
	v := fg.g.tvalOf(x.X)
	if v == nil {
		return nil
	}
	switch {
	case v.Kind == ftype.Pointer && v.Elem != nil && v.Elem.Kind != ftype.Func && v.Elem.C != nil && !isVoid(v.Elem.C):
		return &cppir.Unary{Op: "*", X: call("gp::nonnull", fg.expr(x.X))}
	case v.Kind == ftype.Record && !isRuntimeType(v.cType()):
		return &cppir.Unary{Op: "*", X: fg.expr(x.X)}
	}
	return nil
}

// isVoid reports whether t is void.
func isVoid(t *ctype.Type) bool {
	return t.Kind == ctype.Builtin && t.Name == "void"
}
