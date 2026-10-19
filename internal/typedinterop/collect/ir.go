package collect

import (
	"strconv"
	"strings"
)

// The expression IR.
//
// The collector describes every reference as a small tree of C++ pieces, the
// way the probe has to write it, not as Go syntax: a path, a template
// instance, a call, a member access, an operator, a literal, or a
// representative value (an Arg) for what has a Go type. The trees are
// immutable and print themselves in a canonical form (String), which is the
// key that merges equal references. The probe package prints the same trees
// as C++ (it needs the kind of each name for that: a "::" or a "." after a
// path, a template instance or an index).
//
// Nothing here has a position or a Go type: two references that differ only
// there (f(n) with n an int in one place and a different int variable in
// another) are one reference.

// An Expr is a C++ expression or type of a reference. The types of this
// file are its only implementations.
type Expr interface {
	// String returns the canonical text of the tree: the same for equal trees
	// whatever order or comments the source had.
	String() string
	isExpr()
}

// A Path is a name that comes from C/C++: the C name of a header package or
// the path in a namespace.
type Path struct {
	// Ns is "" for a name of a header package (raylib.InitWindow: Parts is
	// ["InitWindow"]), "C" for a name of cgo's own (C.int, C.struct_stat), or
	// the namespace (::std, ::std::filesystem, after a namespace rename) with
	// Parts the names after it: ["chrono", "steady_clock", "now"].
	Ns    string
	Parts []string

	// Hdr is the name of the header package that the path came through
	// (oop in oop.scale), for the paths with Ns "". The name can be a C++
	// namespace too (a header oop.hpp that declares namespace oop): then the
	// lookup is in the namespace first, as the opaque build does (§2.3 of the
	// Phase 12 plan, lower.cBase). It is not part of String: the key of a
	// reference is the C++ name as the C level has it.
	Hdr string
}

func (*Path) isExpr() {}

// String returns the canonical text: "::printf" for a name of a header
// package, "C.int" for a name of cgo's, "std::chrono::now" for a name in a
// namespace (the leading "::" of Ns is dropped).
func (p *Path) String() string {
	switch p.Ns {
	case "":
		return "::" + strings.Join(p.Parts, "::")
	case "C":
		return "C." + strings.Join(p.Parts, ".")
	}
	return strings.TrimPrefix(p.Ns, "::") + "::" + strings.Join(p.Parts, "::")
}

// KindPath returns the path as the kind probe asks about it
// (interop.ProbeKinds): std::chrono, printf, with no leading "::".
func (p *Path) KindPath() string {
	switch p.Ns {
	case "", "C":
		return strings.Join(p.Parts, "::")
	}
	return strings.TrimPrefix(p.Ns, "::") + "::" + strings.Join(p.Parts, "::")
}

// Prefixes returns the kind paths of the path and of each of its prefixes
// that is a name by itself: std::chrono and std::chrono::steady_clock for
// std::chrono::steady_clock::now. For a name of a header package, which can
// be a namespace too, the name only.
func (p *Path) Prefixes() []string {
	var out []string
	for i := range p.Parts {
		q := &Path{Ns: p.Ns, Hdr: p.Hdr, Parts: p.Parts[:i+1]}
		out = append(out, q.KindPath())
	}
	return out
}

// An Inst is X[args] after a C/C++ name: template arguments if the kind of X
// is a template, else an index (see IndexOrInst). It exists because only
// the kind probe can tell, so the collector keeps both readings.
type Inst struct {
	X    Expr
	Args []Expr
}

func (*Inst) isExpr() {}
func (e *Inst) String() string {
	return "inst(" + e.X.String() + "|" + joinExprs(e.Args) + ")"
}

// A GoType is the C++ spelling of a Go type that C++ can name without
// gocpp's generated code: int64_t for int, double for float64, bool,
// void* for unsafe.Pointer.
type GoType struct{ Cxx string }

func (*GoType) isExpr()          {}
func (e *GoType) String() string { return "go(" + e.Cxx + ")" }

// A Lit is a literal with C's default type (an untyped Go constant passed
// to C/C++: 5 is an int, 2.0 a double, "x" a string literal). Text is the
// C++ text.
type Lit struct{ Text string }

func (*Lit) isExpr()          {}
func (e *Lit) String() string { return "lit(" + e.Text + ")" }

// An Arg stands for a value that is not written as C++: a Go variable, field
// or call, or a local variable that holds a C/C++ value. Its C++ is
// std::declval<T&>() (a variable, field, index or dereference is an lvalue,
// as in the generated code) or std::declval<T>() (a temporary). T is a type:
// a GoType, a Stand, a C/C++ type from the declaration of a variable
// (var p *cstdio.FILE), or the Decay of the value a variable was declared
// from (x := lib.f(): the type of x is what auto gives).
type Arg struct {
	T      Expr
	LValue bool
}

func (*Arg) isExpr() {}
func (e *Arg) String() string {
	if e.LValue {
		return "arg&(" + e.T.String() + ")"
	}
	return "arg(" + e.T.String() + ")"
}

// A Decay is the type that `auto v = X` gives v: std::decay_t of the type of
// the expression X (or of the type X, for the type nodes TupleElem and
// RangeElem). It is the root of the reference of a local variable that is
// declared from a C/C++ value.
type Decay struct{ X Expr }

func (*Decay) isExpr()          {}
func (e *Decay) String() string { return "decay(" + e.X.String() + ")" }

// A DeclRef is the type of a local variable whose reference is Def (a
// Decay), when it is too large to be written inline: the probe names the
// type of Def by its alias. String uses a hash of Def's key, so that the key
// of a chain of variables doesn't grow with the chain.
type DeclRef struct{ Def *Ref }

func (*DeclRef) isExpr() {}
func (e *DeclRef) String() string {
	return "decl#" + e.Def.hash()
}

// A Stand is a gocpp container type that C++ has no name for: the probe
// declares a stand-in that has the operations of the real type (slice, array,
// map, string): gocpp's own runtime types are not in the probe file. Name is
// "slice", "array" (Args: element, length), "map" or "string".
type Stand struct {
	Name string
	Args []Expr
}

func (*Stand) isExpr() {}
func (e *Stand) String() string {
	return "stand:" + e.Name + "(" + joinExprs(e.Args) + ")"
}

// A TupleElem is the I-th element of the tuple (or pair) type that X
// has: a, b := std.minmax(x, y).
type TupleElem struct {
	X Expr
	I int
}

func (*TupleElem) isExpr() {}
func (e *TupleElem) String() string {
	return "tuple(" + e.X.String() + "|" + strconv.Itoa(e.I) + ")"
}

// A RangeElem is what range X gives: the key (or index) with Key, else the
// value (the element, or the mapped value of a map). Both are types.
type RangeElem struct {
	X   Expr
	Key bool
}

func (*RangeElem) isExpr() {}
func (e *RangeElem) String() string {
	if e.Key {
		return "rangekey(" + e.X.String() + ")"
	}
	return "rangeval(" + e.X.String() + ")"
}

// A Call is Fun(Args): a function call, a method call (Fun is a Select), a
// constructor or conversion of a C++ type (Fun is a Path or an Inst), a call
// of an object with operator(). Ellipsis is f(xs...).
type Call struct {
	Fun      Expr
	Args     []Expr
	Ellipsis bool
}

func (*Call) isExpr() {}
func (e *Call) String() string {
	s := "call(" + e.Fun.String() + "|" + joinExprs(e.Args)
	if e.Ellipsis {
		s += "|..."
	}
	return s + ")"
}

// A Select is X.Name: a member of a value (through gp::deref: a pointer is
// dereferenced), or a member after a type (a static member, a nested type;
// the probe decides by the kind of X).
type Select struct {
	X    Expr
	Name string
}

func (*Select) isExpr()          {}
func (e *Select) String() string { return "sel(" + e.X.String() + "|" + e.Name + ")" }

// An Index is X[Idx] when X is a value (an element), after the kind probe
// has not said that X is a template (see IndexOrInst).
type IndexOrInst struct {
	X    Expr
	Args []Expr
}

func (*IndexOrInst) isExpr() {}
func (e *IndexOrInst) String() string {
	return "index(" + e.X.String() + "|" + joinExprs(e.Args) + ")"
}

// A SliceX is X[Lo:Hi:Max]; a missing part is nil.
type SliceX struct{ X, Lo, Hi, Max Expr }

func (*SliceX) isExpr() {}
func (e *SliceX) String() string {
	part := func(x Expr) string {
		if x == nil {
			return "_"
		}
		return x.String()
	}
	return "slice(" + e.X.String() + "|" + part(e.Lo) + "|" + part(e.Hi) + "|" + part(e.Max) + ")"
}

// A Unary is Op X for a unary operator of Go: + - ! ^ (C++ ~) and <-.
type Unary struct {
	Op string
	X  Expr
}

func (*Unary) isExpr()          {}
func (e *Unary) String() string { return "un(" + e.Op + "|" + e.X.String() + ")" }

// A Binary is X Op Y with Go's spelling of the operator (&^ is C++ x & ~y,
// && is &&).
type Binary struct {
	Op   string
	X, Y Expr
}

func (*Binary) isExpr() {}
func (e *Binary) String() string {
	return "bin(" + e.Op + "|" + e.X.String() + "|" + e.Y.String() + ")"
}

// A Star is *X: a dereference of a value, or a pointer to the type X in a
// type or a template argument.
type Star struct{ X Expr }

func (*Star) isExpr()          {}
func (e *Star) String() string { return "star(" + e.X.String() + ")" }

// An Addr is &X.
type Addr struct{ X Expr }

func (*Addr) isExpr()          {}
func (e *Addr) String() string { return "addr(" + e.X.String() + ")" }

// A Paren is (X).
type Paren struct{ X Expr }

func (*Paren) isExpr()          {}
func (e *Paren) String() string { return "paren(" + e.X.String() + ")" }

// An Elt is an element of a Composite literal: Val, with the Key (a field
// name, when KeyName is set; else an expression, a map key).
type Elt struct {
	KeyName string
	Key     Expr
	Val     Expr
}

func (e Elt) String() string {
	switch {
	case e.KeyName != "":
		return e.KeyName + ":" + e.Val.String()
	case e.Key != nil:
		return "[" + e.Key.String() + "]:" + e.Val.String()
	}
	return e.Val.String()
}

// A Composite is the braced initialization T{elts} of a C/C++ type. A
// literal whose type was left out (inside another literal) has no Type
// here: the collector gives it the element type by a member probe instead,
// so the probe only sees literals with their type.
type Composite struct {
	Type Expr
	Elts []Elt
}

func (*Composite) isExpr() {}
func (e *Composite) String() string {
	parts := make([]string, len(e.Elts))
	for i, el := range e.Elts {
		parts[i] = el.String()
	}
	typ := "_"
	if e.Type != nil {
		typ = e.Type.String()
	}
	return "lit{" + typ + "|" + strings.Join(parts, ",") + "}"
}

// An Unknown is a part that has no C++ spelling for the probe: a Go type
// that only gocpp's generated code declares (a struct, a slice), a function
// literal. A reference that has one is Incomplete and is not probed.
type Unknown struct{ Why string }

func (*Unknown) isExpr()          {}
func (e *Unknown) String() string { return "unknown(" + e.Why + ")" }

func joinExprs(es []Expr) string {
	parts := make([]string, len(es))
	for i, e := range es {
		parts[i] = e.String()
	}
	return strings.Join(parts, ",")
}

// Walk calls f for e and, while f returns true, for each of its parts, in
// order. A DeclRef's definition is not entered: it is another reference.
func Walk(e Expr, f func(Expr) bool) {
	if e == nil || !f(e) {
		return
	}
	switch e := e.(type) {
	case *Inst:
		Walk(e.X, f)
		walkAll(e.Args, f)
	case *Arg:
		Walk(e.T, f)
	case *Decay:
		Walk(e.X, f)
	case *Stand:
		walkAll(e.Args, f)
	case *TupleElem:
		Walk(e.X, f)
	case *RangeElem:
		Walk(e.X, f)
	case *Call:
		Walk(e.Fun, f)
		walkAll(e.Args, f)
	case *Select:
		Walk(e.X, f)
	case *IndexOrInst:
		Walk(e.X, f)
		walkAll(e.Args, f)
	case *SliceX:
		Walk(e.X, f)
		Walk(e.Lo, f)
		Walk(e.Hi, f)
		Walk(e.Max, f)
	case *Unary:
		Walk(e.X, f)
	case *Binary:
		Walk(e.X, f)
		Walk(e.Y, f)
	case *Star:
		Walk(e.X, f)
	case *Addr:
		Walk(e.X, f)
	case *Paren:
		Walk(e.X, f)
	case *Composite:
		Walk(e.Type, f)
		for _, el := range e.Elts {
			Walk(el.Key, f)
			Walk(el.Val, f)
		}
	}
}

func walkAll(es []Expr, f func(Expr) bool) {
	for _, e := range es {
		Walk(e, f)
	}
}
