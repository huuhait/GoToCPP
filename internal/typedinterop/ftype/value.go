package ftype

import (
	"go/token"
	"go/types"
	"sort"
	"strings"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// A Value is what the probe result of a reference is in the foreign view: the
// mapped type, and what the expression is (a type, an lvalue, a constant).
type Value struct {
	Mapped

	// Result is the probe result.
	Result *probe.Result

	// IsType says the reference names a type: Mapped is that type.
	IsType bool

	// Category is the value category of an expression (lvalue, xvalue, prvalue).
	Category probe.Category

	// Lvalue says the expression designates an object; Assignable says it can
	// also be assigned to: v.front() = 5 (a T& result, not const, not an array).
	// The README rule: "T& result is an addressable foreign value".
	Lvalue, Assignable bool

	// Const is the compile-time value of a constant (a macro, an enumerator).
	ConstValue *probe.Const
}

// Value maps a probe result. A result that failed or was skipped is Opaque
// with the reason; the interop checker leaves such a reference to the C++
// compiler.
func (m *Mapper) Value(r *probe.Result) Value {
	v := Value{Result: r}
	switch {
	case r.Skipped != "":
		v.Mapped = opaque(nil, "not probed: %s", r.Skipped)
		return v
	case !r.OK:
		v.Mapped = opaque(nil, "the probe failed: %s", r.Error)
		return v
	}
	c, err := ctype.Parse(r.Type)
	if err != nil {
		v.Mapped = opaque(nil, "%v", err)
		return v
	}
	var w *ctype.Type
	if r.Written != "" {
		w, _ = ctype.Parse(r.Written)
	}
	v.Mapped = m.Map(c, w)
	v.Category = r.Category
	v.IsType = r.Form == probe.FormType
	v.ConstValue = r.Value
	if !v.IsType {
		v.Lvalue = r.Category == probe.CatLValue
		v.Assignable = v.Lvalue && !r.Const && v.Kind != Array && v.Kind != Func && v.Kind != Void && v.Kind != Opaque
	}
	return v
}

// String writes the value for tests and the developer tool:
//
//	"type C.int", "lvalue C.long", "prvalue *C.char", "assignable std.string", "opaque: <reason>"
func (v Value) String() string {
	if v.Kind == Opaque {
		return "opaque: " + v.Reason
	}
	t := "void"
	if v.Type != nil {
		t = typeString(v.Type)
	}
	if d := v.Display(); d != t && d != "void" {
		t += " (" + d + ")"
	}
	if v.Incomplete {
		t += " (members unknown)"
	}
	switch {
	case v.IsType:
		return "type " + t
	case v.Assignable:
		return "assignable " + t
	case v.Lvalue:
		return "lvalue " + t
	}
	if v.Category == probe.CatXValue {
		return "xvalue " + t
	}
	return "value " + t
}

// ---------------------------------------------------------------- what to ask for

// Needs lists the declarations to extract for the mapper to map the results
// completely: the classes and enums named by their types (also inside
// pointers, references and template arguments). Names that the mapper
// already knows, or that were asked and have no answer, are left out.
func (m *Mapper) Needs(results []probe.Result) []probe.DeclQuery {
	q := newQueries(m)
	for i := range results {
		r := &results[i]
		if !r.OK {
			continue
		}
		if c, err := ctype.Parse(r.Type); err == nil {
			q.leaves(c, "", true)
		}
		// The typedef names of the written type (FILE for _IO_FILE) are asked for too, so
		// that the mapper can name the type as the program does. Not when the canonical type has
		// nothing of the program's or the C library's (the library's own value_type, iterator),
		// and not the name the canonical type has too (Vec for geo::Vec: the same name, written
		// relative to its namespace).
		if r.Written != "" {
			c, err1 := ctype.Parse(r.Type)
			w, err2 := ctype.Parse(r.Written)
			if err1 == nil && err2 == nil {
				// Only when the type itself (under pointers and references) is not the library's.
				if l := c.Leaf(); l != nil && l.Kind == ctype.Named && (isInternal(l.String()) || l.Path[0].Name == "std") {
					continue
				}
				user := false
				lasts := map[string]bool{}
				c.Walk(func(n *ctype.Type) {
					if n.Kind == ctype.Named {
						lasts[n.Last()] = true
						if !isInternal(n.String()) && n.Path[0].Name != "std" && n.Path[0].Name != "gocpp_probe_internal_pr" {
							user = true
						}
					}
				})
				if user {
					w.Walk(func(n *ctype.Type) {
						if n.Kind == ctype.Named && len(n.Path) > 0 && n.Path[0].Name != "std" && !lasts[n.Last()] {
							q.leaves(n, "", true)
						}
					})
				}
			}
		}
	}
	return q.list()
}

// NeedsMore lists the declarations that the records and functions the mapper
// has refer to and that are not known yet: the types of their public fields
// and bases, and of the parameters and results of their functions. Call it
// after Add until it returns nothing (or a limit is reached): each round
// reaches one more level of types. Names in member types are relative to the
// scope of the declaration, so every enclosing scope is asked.
func (m *Mapper) NeedsMore() []probe.DeclQuery {
	q := newQueries(m)
	for _, key := range sortedRecords(m.decls) {
		rec := m.decls.Records[key]
		if isInternal(rec.Name) || inStd(rec.Scope) {
			continue
		}
		for _, f := range rec.Fields {
			if f.Access == probe.Public && f.Type.T != nil {
				q.leaves(f.Type.T, rec.Scope, false)
			}
		}
		for _, b := range rec.Bases {
			if b.Access == probe.Public && b.Type.T != nil {
				q.leaves(b.Type.T, rec.Scope, false)
			}
		}
	}
	for _, name := range sortedFuncs(m.decls) {
		fs := m.decls.Funcs[name]
		if inStd(fs.Scope) {
			continue
		}
		for _, ov := range fs.Overloads {
			if ov.Sig.T != nil {
				q.leaves(ov.Sig.T, fs.Scope, false)
			}
		}
	}
	return q.list()
}

// inStd reports whether a scope is the standard library's: the types in its
// declarations are its own business (the mapper knows its names, and does not
// follow them into the library).
func inStd(scope string) bool { return scope == "std" || strings.HasPrefix(scope, "std::") }

func sortedRecords(d *probe.DeclTable) []string {
	var ks []string
	for k := range d.Records {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func sortedFuncs(d *probe.DeclTable) []string {
	var ks []string
	for k := range d.Funcs {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

type queries struct {
	m    *Mapper
	seen map[string]bool
	out  []probe.DeclQuery
}

func newQueries(m *Mapper) *queries { return &queries{m: m, seen: map[string]bool{}} }

func (q *queries) list() []probe.DeclQuery {
	sort.Slice(q.out, func(i, j int) bool { return q.out[i].Name < q.out[j].Name })
	return q.out
}

// asked reports whether the declarations have an answer for a type name,
// good or bad.
func (m *Mapper) asked(name string) bool {
	if m.known(name) {
		return true
	}
	_, bad := m.decls.Errors[probe.QueryKey(probe.DeclQuery{Kind: probe.DeclType, Name: name})]
	return bad
}

// leaves asks for the names in t. scope is the scope of the names ("" for
// canonical types, as the results have them); a name in a scope is asked
// for under each enclosing scope, with its template arguments qualified first
// (so the names in the arguments are asked one round before the name).
func (q *queries) leaves(t *ctype.Type, scope string, canonical bool) {
	t.Walk(func(n *ctype.Type) {
		if n.Kind != ctype.Named || isInternal(n.String()) {
			return
		}
		if len(n.Path) > 0 && (n.Path[0].Name == "gocpp_probe_internal_pr" || strings.HasPrefix(n.Path[0].Name, "type-parameter")) {
			return
		}
		if n.Tag == "typename" {
			return
		}
		own := nameKey(n)
		if isTypedefName(own) {
			return
		}
		cands := []string{own}
		if scope != "" && !n.Global {
			// A name inside a library's class is the library's business, and the library's
			// own names (std::string) are known to the mapper without a query.
			if n.Path[0].Name == "std" || strings.HasPrefix(n.Path[0].Name, "_") {
				return
			}
			qn, ok := q.m.qualifyArgs(n, scope)
			if !ok {
				return // an argument is not resolved yet: it is asked for first
			}
			own = nameKey(qn)
			cands = nil
			parts := strings.Split(scope, "::")
			for i := len(parts); i >= 0; i-- {
				cands = append(cands, strings.Join(append(append([]string{}, parts[:i]...), own), "::"))
			}
		}
		for _, c := range cands {
			if q.m.known(c) {
				return // found: no need to look further out
			}
		}
		for _, c := range cands {
			if q.m.asked(c) || q.seen[c] {
				continue
			}
			q.seen[c] = true
			q.out = append(q.out, probe.DeclQuery{Kind: probe.DeclType, Name: c})
		}
	})
}

// isInternal reports whether a name is one of the library's own internals
// (std::_Vector_base, __gnu_cxx::..., a part that starts with two underscores,
// or with an underscore and a capital inside a namespace): they are not asked for. A
// global _IO_FILE is the C library's name for a real type.
func isInternal(name string) bool {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == ':' || r == '<' || r == '>' || r == ',' || r == ' ' || r == '*' || r == '&'
	})
	for i, part := range parts {
		if strings.HasPrefix(part, "__") || i > 0 && len(part) > 1 && part[0] == '_' && part[1] >= 'A' && part[1] <= 'Z' {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- compatibility

// A Verdict is an answer that may be "I do not know".
type Verdict uint8

const (
	Unknown Verdict = iota
	Yes
	No
)

func (v Verdict) String() string { return [...]string{"unknown", "yes", "no"}[v] }

// Compat says whether a value of the foreign view can go where the Go type
// target is wanted (an assignment, a conversion, a result), by the rules of
// C++ and gocpp's runtime, which are more permissive than Go's (the
// compatibility rule of Phase 12: nothing that opaque mode accepts may be
// rejected). It answers No only when no implicit conversion or explicit
// conversion exists in C++ (a record where a number is wanted, a number where
// a pointer is wanted, a void where anything is wanted); Unknown is the answer
// for everything that depends on a conversion operator, a constructor or a type the
// check cannot see. target is the Go type as go/types prints it.
func (v Value) Compat(target string) Verdict {
	if target == "" || target == "invalid type" {
		return Unknown
	}
	if v.Kind == Opaque {
		return Unknown
	}
	class := goClass(target)
	if class == classUnknown {
		return Unknown
	}
	switch v.Kind {
	case Void:
		return No
	case Scalar:
		switch class {
		case classNumber, classBool:
			return Yes
		case classPointer, classFunc:
			if v.nullConstant() {
				return Yes // char *p = NULL
			}
			return No
		case classSlice, classMap:
			if v.nullConstant() {
				return Unknown // a gocpp slice or map from NULL: the runtime's business
			}
			return No
		}
	case Nullptr:
		switch class {
		case classPointer, classFunc:
			return Yes
		case classSlice, classMap, classBool:
			return Unknown
		case classNumber, classString:
			return No
		}
	case Enum:
		switch class {
		case classNumber, classBool:
			if !v.Scoped {
				return Yes
			}
			return Unknown // static_cast works, the implicit conversion does not
		case classPointer, classSlice, classMap, classFunc, classString:
			return No
		}
	case Pointer, FuncPtr:
		switch class {
		case classNumber, classSlice, classMap:
			return No
		case classBool, classPointer:
			return Yes
		case classFunc:
			if v.Kind == FuncPtr {
				return Yes
			}
			return No
		}
	case Record:
		switch class {
		case classNumber, classBool, classPointer, classSlice, classMap, classFunc:
			return Unknown // a conversion operator may exist; only the declaration says
		}
	case Go:
		if tt, ok := evalType(target); ok && v.Type != nil {
			if types.AssignableTo(v.Type, tt) || types.ConvertibleTo(v.Type, tt) {
				return Yes
			}
			if pointerToVoidOut(v.Type, tt) {
				return Yes // the argument adaptation of a void** parameter (lower: voidout)
			}
			return No
		}
	case Array:
		if class == classNumber || class == classBool || class == classString {
			return No
		}
	}
	return Unknown
}

// pointerToVoidOut reports whether from is a pointer to a pointer and to is *unsafe.Pointer: the
// shape that the lowering adapts for a `void **` parameter (&p of a variable p of a pointer type).
func pointerToVoidOut(from, to types.Type) bool {
	outer, ok := from.Underlying().(*types.Pointer)
	if !ok {
		return false
	}
	if _, ok := outer.Elem().Underlying().(*types.Pointer); !ok {
		return false
	}
	target, ok := to.Underlying().(*types.Pointer)
	if !ok {
		return false
	}
	basic, ok := target.Elem().Underlying().(*types.Basic)
	return ok && basic.Kind() == types.UnsafePointer
}

// nullConstant reports whether the value is a null pointer constant: an integer constant
// with the value 0 (the macro NULL is 0 or __null; a constant of type bool or a float is not
// one, and neither is an enumerator).
//
// C++ is stricter than this: only an integer literal 0 (0, 0L, 0u, 0x0, 00) is one, and a
// constant expression such as '\0', (1 - 1) or a constexpr zero is not (both compilers refuse
// `char *p = (1 - 1);`). The probe gives the value of a constant, not its form, so the rule
// is permissive: a zero constant that is not a literal goes to a pointer here (Yes) where the
// C++ compiler refuses it. That cannot reject a program that opaque mode accepts, which is
// the rule Compat serves; the cost is a missed early error.
func (v Value) nullConstant() bool {
	if v.Kind != Scalar || v.ConstValue == nil || v.ConstValue.Kind != "int" || v.ConstValue.Int == nil || v.ConstValue.Int.Sign() != 0 {
		return false
	}
	b, ok := v.Type.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsInteger != 0
}

type goKind uint8

const (
	classUnknown goKind = iota
	classNumber
	classBool
	classString
	classPointer
	classSlice
	classMap
	classFunc
)

// goClass classifies a Go type given as text.
func goClass(t string) goKind {
	switch {
	case t == "bool":
		return classBool
	case t == "string":
		return classString
	case strings.HasPrefix(t, "*"):
		return classPointer
	case strings.HasPrefix(t, "[]"):
		return classSlice
	case strings.HasPrefix(t, "map["):
		return classMap
	case strings.HasPrefix(t, "func("):
		return classFunc
	}
	switch t {
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64", "byte", "rune":
		return classNumber
	}
	return classUnknown
}

// evalType parses a Go type that only uses the universe.
func evalType(s string) (types.Type, bool) {
	tv, err := types.Eval(token.NewFileSet(), nil, token.NoPos, s)
	if err != nil || !tv.IsType() {
		return nil, false
	}
	return tv.Type, true
}
