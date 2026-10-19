package collect

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/token"
	"io"
	"slices"
	"sort"
	"strings"
)

// A Kind is the shape of a reference: the root of its expression.
type Kind string

// The shapes. A reference is one of them, and its Expr has the matching root
// node (see ir.go).
const (
	KindPath    Kind = "path"    // a name of a header package or a namespace: raylib.InitWindow, std.vector, cstdio.EOF
	KindInst    Kind = "inst"    // X[args] after a C/C++ name: template arguments, or an index (the kind probe decides)
	KindCall    Kind = "call"    // a call, a constructor or a conversion of a C/C++ type: f(x), std.string("a")
	KindMember  Kind = "member"  // x.m on a value, or a member after a type
	KindIndex   Kind = "index"   // v[i] on a value
	KindSlice   Kind = "slice"   // v[i:j] (not probed: C++ has no slice expression)
	KindUnary   Kind = "unary"   // -x, !x, ^x
	KindBinary  Kind = "binary"  // x + y, x == y, x &^ y
	KindDeref   Kind = "deref"   // *p, or *T in a type
	KindAddr    Kind = "addr"    // &x
	KindParen   Kind = "paren"   // (x): always a value
	KindLiteral Kind = "literal" // T{...}
	KindLocal   Kind = "local"   // a variable declared from a C/C++ value: x := f(); its type is what auto gives
	KindType    Kind = "type"    // a Go type or a gocpp container of C/C++ elements, written as a type: []C.int
	KindOther   Kind = "other"   // anything else (not probed)
)

// A Context says where a reference is used: what the expression around it
// is. The checker (step 12.5) uses it to decide what a probe must mirror.
type Context string

// The contexts of a use.
const (
	CtxStmt     Context = "stmt"     // an expression statement
	CtxCallee   Context = "callee"   // the function of a call
	CtxTemplate Context = "template" // the X of X[a, b] (more than one argument: always a template)
	CtxArg      Context = "arg"      // an argument of a call
	CtxConvert  Context = "convert"  // the operand of a conversion to a Go type (Target)
	CtxOperand  Context = "operand"  // an operand of an operator
	CtxMember   Context = "member"   // the X of x.m
	CtxIndex    Context = "index"    // the X or the index of v[i]
	CtxInit     Context = "init"     // the value of a declaration (x := v, var x T = v)
	CtxAssign   Context = "assign"   // the value of an assignment
	CtxReturn   Context = "return"   // a result
	CtxCond     Context = "cond"     // the condition of if, for or switch
	CtxRange    Context = "range"    // the X of for range
	CtxConstant Context = "constant" // a constant declaration, an array length or a case value: a macro here must be a constant
	CtxElement  Context = "element"  // an element or a key of a composite literal
	CtxType     Context = "type"     // a type position: a declaration, a field, a literal's type
	CtxOther    Context = "other"
)

// A Site is one place that uses a reference.
type Site struct {
	File string // relative to the package folder when it is in it
	Line int
	Col  int

	Ctx Context

	// Target is the Go type that the value goes to when the context says so
	// (a conversion int(x), var n int = x, an argument of a Go function, a
	// result): the text of the type. "" otherwise. The checker uses it for
	// the compatibility rule (narrowing conversions).
	Target string

	// Text is the expression as written in the .gp file (after the
	// loader's rewrite: C.std.vector for std.vector).
	Text string
}

// A Ref is one reference shape: one C/C++ expression, type or name that the
// package uses, with all the places that use it. Two uses with the same C++
// meaning are one Ref: the Key is the canonical text of the expression.
type Ref struct {
	// ID is the index of the reference in Set.Refs. The probe names its
	// declarations gp_probe_<ID>. IDs follow the order of the keys, so they
	// don't depend on the order of the source.
	ID int

	Kind Kind

	// Key is the canonical text of Expr: equal for equal references.
	Key string

	// Expr is the reference as C++ pieces (ir.go).
	Expr Expr

	// Written is the name as the user wrote it, for a KindPath
	// ("std.chrono.steady_clock.now"), else "".
	Written string

	// Incomplete is "" or the reason why no probe can be written: a part has
	// no C++ spelling (a Go struct as an argument, a function literal). The
	// probe skips such a reference; it stays opaque.
	Incomplete string

	// Standalone is false for a name (a path, a member, an instance) that is
	// only the function of calls or the template of instances: the call or
	// instance reference has its type, and the name alone is not an
	// expression, so it is not probed.
	Standalone bool

	// WantValue says that a use needs a compile-time value: a macro or
	// expression in a constant context (CtxConstant). The probe adds a
	// constant probe for it.
	WantValue bool

	// Sites are the uses, in source order.
	Sites []Site

	h string // memo of hash
}

func (r *Ref) hash() string {
	if r.h == "" {
		sum := sha256.Sum256([]byte(r.Key))
		r.h = hex.EncodeToString(sum[:4])
	}
	return r.h
}

// A Set is the references of one package: the result of Collect.
type Set struct {
	// Dir is the package folder; the files of the sites are relative to it.
	Dir string

	// Refs are the references, sorted by Key. Refs[i].ID == i.
	Refs []*Ref

	// Locals holds, for a variable declared from a C/C++ value, the
	// reference of its type: Refs entries of KindLocal, by declaring name
	// position, for tools that show the type of a variable.
	byNode map[any]*Ref
	byKey  map[string]*Ref
}

// Ref returns the reference with the key, or nil.
func (s *Set) Ref(key string) *Ref { return s.byKey[key] }

// RefOf returns the reference of the AST node n (an expression that the
// collector made a reference of, a use of a local variable (the reference of
// its type), or the name that declares a local variable), or nil. n is a
// node of the files that Collect read.
func (s *Set) RefOf(n any) *Ref { return s.byNode[n] }

// KindPaths returns the paths that the kind probe (interop.ProbeKinds) has
// to be asked about before the references are probed: every path that a
// reference uses, and each prefix of it (std::chrono, std::chrono::steady_clock
// for std::chrono::steady_clock::now), sorted and each once. Names of cgo's
// own (C.int) are not paths.
func (s *Set) KindPaths() []string {
	seen := make(map[string]bool)
	for _, r := range s.Refs {
		Walk(r.Expr, func(e Expr) bool {
			if p, ok := e.(*Path); ok && p.Ns != "C" {
				for _, k := range p.Prefixes() {
					seen[k] = true
				}
			}
			return true
		})
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// HeaderNames returns the names of the header packages that the references
// go through (oop for oop.scale), sorted and each once. A name can be a C++
// namespace too: the caller asks the kind probe about the names, and then
// about NamespacePaths for those that are.
func (s *Set) HeaderNames() []string {
	seen := make(map[string]bool)
	for _, r := range s.Refs {
		Walk(r.Expr, func(e Expr) bool {
			if p, ok := e.(*Path); ok && p.Ns == "" && p.Hdr != "" {
				seen[p.Hdr] = true
			}
			return true
		})
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// NamespacePaths returns the kind paths in the namespaces ns (header package
// names that are namespaces too): oop::scale for oop.scale and each prefix of
// longer paths, sorted and each once.
func (s *Set) NamespacePaths(ns map[string]bool) []string {
	seen := make(map[string]bool)
	for _, r := range s.Refs {
		Walk(r.Expr, func(e Expr) bool {
			if p, ok := e.(*Path); ok && p.Ns == "" && ns[p.Hdr] {
				for _, k := range p.Prefixes() {
					seen[p.Hdr+"::"+k] = true
				}
			}
			return true
		})
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Probeable returns the references that a probe can be written for (not
// Incomplete, and standalone), in ID order.
func (s *Set) Probeable() []*Ref {
	var out []*Ref
	for _, r := range s.Refs {
		if r.Incomplete == "" && r.Standalone {
			out = append(out, r)
		}
	}
	return out
}

// Format writes the set as text, one line per reference and one indented
// line per site (when sites is true): the form of the golden files.
func (s *Set) Format(w io.Writer, sites bool) {
	for _, r := range s.Refs {
		flags := ""
		if r.Incomplete != "" {
			flags += " incomplete(" + r.Incomplete + ")"
		}
		if !r.Standalone {
			flags += " name-only"
		}
		if r.WantValue {
			flags += " want-value"
		}
		fmt.Fprintf(w, "%d %s %s%s\n", r.ID, r.Kind, r.Key, flags)
		if !sites {
			continue
		}
		for _, st := range r.Sites {
			t := ""
			if st.Target != "" {
				t = " -> " + st.Target
			}
			fmt.Fprintf(w, "    %s:%d:%d %s%s  %s\n", st.File, st.Line, st.Col, st.Ctx, t, st.Text)
		}
	}
}

// String returns Format(w, true).
func (s *Set) String() string {
	var b strings.Builder
	s.Format(&b, true)
	return b.String()
}

// A SiteJSON is a Site as the developer tool prints it.
type SiteJSON struct {
	File   string  `json:"file"`
	Line   int     `json:"line"`
	Col    int     `json:"col"`
	Ctx    Context `json:"ctx"`
	Target string  `json:"target,omitempty"`
	Text   string  `json:"text"`
}

// A RefJSON is a Ref as the developer tool prints it.
type RefJSON struct {
	ID         int        `json:"id"`
	Kind       Kind       `json:"kind"`
	Key        string     `json:"key"`
	Written    string     `json:"written,omitempty"`
	Incomplete string     `json:"incomplete,omitempty"`
	NameOnly   bool       `json:"nameOnly,omitempty"`
	WantValue  bool       `json:"wantValue,omitempty"`
	Sites      []SiteJSON `json:"sites"`
}

// JSON returns the reference in the form of the developer tool's table.
func (r *Ref) JSON() RefJSON {
	j := RefJSON{ID: r.ID, Kind: r.Kind, Key: r.Key, Written: r.Written, Incomplete: r.Incomplete, NameOnly: !r.Standalone, WantValue: r.WantValue}
	for _, st := range r.Sites {
		j.Sites = append(j.Sites, SiteJSON(st))
	}
	return j
}

// MarshalJSON writes the references with their sites.
func (s *Set) MarshalJSON() ([]byte, error) {
	out := make([]RefJSON, len(s.Refs))
	for i, r := range s.Refs {
		out[i] = r.JSON()
	}
	return json.Marshal(out)
}

// finish sorts the references and the sites, drops duplicate sites and
// numbers the references.
func (s *Set) finish() {
	slices.SortFunc(s.Refs, func(a, b *Ref) int { return strings.Compare(a.Key, b.Key) })
	for i, r := range s.Refs {
		r.ID = i
		slices.SortStableFunc(r.Sites, func(a, b Site) int {
			if c := strings.Compare(a.File, b.File); c != 0 {
				return c
			}
			if a.Line != b.Line {
				return a.Line - b.Line
			}
			return a.Col - b.Col
		})
		r.Sites = slices.CompactFunc(r.Sites, func(a, b Site) bool { return a == b })
	}
}

// pos returns the position of p in the form of a Site.
func sitePos(fset *token.FileSet, dir string, p token.Pos) (file string, line, col int) {
	ps := fset.Position(p)
	file = ps.Filename
	if dir != "" {
		if rel, ok := strings.CutPrefix(file, strings.TrimSuffix(dir, "/")+"/"); ok {
			file = rel
		}
	}
	return file, ps.Line, ps.Column
}

// NewSet returns the Set of references that a tool or a test writes by hand
// (no Go source): each Ref needs only its Expr (and WantValue, Written,
// Sites when it matters). The Key, Kind, Standalone and Incomplete fields
// are filled in, equal expressions are merged, and the IDs follow the keys.
func NewSet(refs ...*Ref) *Set {
	s := &Set{byNode: make(map[any]*Ref), byKey: make(map[string]*Ref)}
	for _, r := range refs {
		key := r.Expr.String()
		if old := s.byKey[key]; old != nil {
			old.WantValue = old.WantValue || r.WantValue
			old.Sites = append(old.Sites, r.Sites...)
			continue
		}
		c := *r
		c.Key, c.Kind, c.Standalone = key, kindOf(c.Expr), true
		c.Incomplete = incompleteOf(c.Expr)
		s.byKey[key] = &c
		s.Refs = append(s.Refs, &c)
	}
	s.finish()
	return s
}
