package probe

import (
	"encoding/json"
	"math/big"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
)

// The declarations of step 12.3: what clang says about the records, enums and
// functions that a program names. They come from filtered dumps of the real
// declarations (decls.go), not from the expression probes.

// Access is the access of a member or a base.
type Access string

const (
	Public    Access = "public"
	Protected Access = "protected"
	Private   Access = "private"
)

// A TypeText is a type as clang printed it, and the structured form of it.
// T is nil when the text could not be read (Err says why); then the type
// stays opaque for the mapper.
type TypeText struct {
	Text string      `json:"text"`
	T    *ctype.Type `json:"-"`
	Err  string      `json:"err,omitempty"`
}

// A Field is a non-static data member.
type Field struct {
	Name   string   `json:"name"`
	Type   TypeText `json:"type"`
	Access Access   `json:"access"`
	// Bits is the width of a bit-field, 0 for an ordinary member.
	Bits    int  `json:"bits,omitempty"`
	Mutable bool `json:"mutable,omitempty"`
	// Anon says the member comes from an anonymous union or struct of the record
	// (a field "ui" of "union { int ui; float uf; };", at any depth); Union says that the
	// field shares its storage with others, because an anonymous aggregate around it is a
	// union (so lo and hi of "union { struct { int lo, hi; }; long both; };" are Union
	// fields, but lo and hi do not overlap each other). Union is therefore coarse: it says
	// "some union is around this field", not which fields overlap which; the direct members
	// of a record that is itself a union have Union false (Record.Tag says "union").
	Anon  bool `json:"anon,omitempty"`
	Union bool `json:"union,omitempty"`
}

// A Static is a static data member.
type Static struct {
	Name      string   `json:"name"`
	Type      TypeText `json:"type"`
	Access    Access   `json:"access"`
	Constexpr bool     `json:"constexpr,omitempty"`
}

// A Param is a parameter of a method or function.
type Param struct {
	Name    string `json:"name,omitempty"`
	Default bool   `json:"default,omitempty"`
	// Array says the parameter is written as an array (`float col[3]`: its type is the pointer
	// `float *`, but the callee takes several elements), or that how it is written could not be
	// read (a macro's place): the lowering never passes the address of one variable for it.
	Array bool `json:"array,omitempty"`
}

// MethodKind says what a member function is.
type MethodKind string

const (
	KindMethod     MethodKind = "method"
	KindCtor       MethodKind = "ctor"
	KindDtor       MethodKind = "dtor"
	KindConversion MethodKind = "conversion"
	KindOperator   MethodKind = "operator"
)

// A Method is a member function (also a constructor, destructor, conversion
// or operator).
type Method struct {
	Name string     `json:"name"`
	Kind MethodKind `json:"kind"`
	// Sig is the function type: the result, the parameters, the qualifiers
	// (const, &, noexcept) of "int (int, int) const noexcept". For a
	// constructor the result is void.
	Sig    TypeText `json:"sig"`
	Params []Param  `json:"params,omitempty"`
	Access Access   `json:"access"`

	Static    bool `json:"static,omitempty"`
	Virtual   bool `json:"virtual,omitempty"`
	Pure      bool `json:"pure,omitempty"`
	Override  bool `json:"override,omitempty"`
	Final     bool `json:"final,omitempty"`
	Constexpr bool `json:"constexpr,omitempty"`
	Variadic  bool `json:"variadic,omitempty"`
	Deleted   bool `json:"deleted,omitempty"`
	Defaulted bool `json:"defaulted,omitempty"`
	// Implicit says the compiler declared it (the implicit copy constructor).
	Implicit bool `json:"implicit,omitempty"`
	// Template says it is a member template: the signature mentions its template
	// parameters and says nothing about a call.
	Template bool `json:"template,omitempty"`
}

// A Base is a base class.
type Base struct {
	Type    TypeText `json:"type"`
	Access  Access   `json:"access"`
	Virtual bool     `json:"virtual,omitempty"`
}

// A Record is a class, struct or union.
type Record struct {
	// Name is the canonical spelling of the type ("std::vector<long>").
	Name string `json:"name"`
	// Scope is the namespace or class that holds it ("std", "geo"; "" for the
	// global scope): the types in its members are written relative to it.
	Scope string `json:"scope,omitempty"`
	// Tag is "struct", "class" or "union".
	Tag string `json:"tag"`
	// Complete says the definition is known; an incomplete record (a forward
	// declaration of an opaque handle: struct _IO_FILE) has no members here.
	Complete bool `json:"complete"`
	// Partial is set (and Complete is false) when the definition is known but a member
	// could not be modelled (an anonymous struct that the extraction cannot place): the
	// members are then not reported, and Size and Align are still the compiler's.
	Partial string `json:"partial,omitempty"`
	// Size and Align are sizeof and alignof, -1 when the record is incomplete.
	Size  int64 `json:"size"`
	Align int64 `json:"align"`

	Fields  []Field  `json:"fields,omitempty"`
	Statics []Static `json:"statics,omitempty"`
	Methods []Method `json:"methods,omitempty"`
	Bases   []Base   `json:"bases,omitempty"`

	// Types are the member typedefs and aliases (name to the type clang desugars it
	// to): "size_type": "unsigned long". Nested are the records and enums declared
	// inside it, by name.
	Types  map[string]string `json:"types,omitempty"`
	Nested []string          `json:"nested,omitempty"`

	// Traits are the facts of clang's definition data that matter: aggregate,
	// polymorphic, abstract, empty, standardLayout, trivial, triviallyCopyable,
	// pod.
	Traits map[string]bool `json:"traits,omitempty"`

	// Template says the record is an instance of a class template.
	Template bool `json:"template,omitempty"`
}

// Field returns the field called name, or nil.
func (r *Record) Field(name string) *Field {
	for i := range r.Fields {
		if r.Fields[i].Name == name {
			return &r.Fields[i]
		}
	}
	return nil
}

// MethodsNamed returns the methods called name (the overloads).
func (r *Record) MethodsNamed(name string) []*Method {
	var out []*Method
	for i := range r.Methods {
		if r.Methods[i].Name == name {
			out = append(out, &r.Methods[i])
		}
	}
	return out
}

// An Enumerator is a constant of an enum.
type Enumerator struct {
	Name  string   `json:"name"`
	Value *big.Int `json:"-"`
}

// MarshalJSON writes the value as text (exact for any size).
func (e Enumerator) MarshalJSON() ([]byte, error) {
	v := "?"
	if e.Value != nil {
		v = e.Value.String()
	}
	return json.Marshal(struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}{e.Name, v})
}

// An Enum is an enumeration type.
type Enum struct {
	Name  string `json:"name"`
	Scope string `json:"scope,omitempty"`
	// Scoped says "enum class". Underlying is the integer type that stores it
	// (std::underlying_type), asked of the compiler.
	Scoped     bool         `json:"scoped,omitempty"`
	Underlying TypeText     `json:"underlying"`
	Size       int64        `json:"size"`
	Values     []Enumerator `json:"values"`
}

// An Overload is one function that a name stands for.
type Overload struct {
	// Kind is the kind of declaration: "function", "template" (a function
	// template: its signature is dependent), "variable", or the clang kind
	// of something else ("CXXRecordDecl": the name is a class).
	Kind string `json:"kind"`
	// Sig is the function type (the type of a variable for "variable").
	Sig TypeText `json:"sig"`

	// FormatKind and FormatParam say, for a variadic function, that its parameter number
	// FormatParam (from 1) is a format string by the compiler's own rule, a format attribute
	// (__attribute__((format(printf, ...))), IM_FMTARGS in Dear ImGui): FormatKind is "printf" or
	// "scanf". Found by a compiler run over generated calls (formatChecks); empty when the function
	// has no such attribute, or when the run did not tell.
	FormatKind  string `json:"formatKind,omitempty"`
	FormatParam int    `json:"formatParam,omitempty"`

	// ArrayParams has an entry for every parameter, true for a parameter that is written as an
	// array, or whose spelling is not known (arrayparams.go). It is nil when no call passes an
	// address (DeclConfig.AddrFuncs) and when the compiler's answer is missing: the lowering treats
	// a pointer parameter of such an overload as an array.
	ArrayParams []bool `json:"arrayParams,omitempty"`
}

// A FuncSet is the overload set of a name.
type FuncSet struct {
	// Name is the qualified name that was asked for ("geo::f", "fopen").
	Name string `json:"name"`
	// Scope is the namespace of the name: the types in the signatures are
	// written relative to it.
	Scope     string     `json:"scope,omitempty"`
	Overloads []Overload `json:"overloads"`
}

// A DeclKind says what a DeclQuery asks for.
type DeclKind string

const (
	// DeclType asks for the record or enum that a type name stands for. Name is
	// the canonical spelling (clang's) of a class or enum type, as the
	// Result.Base of a probe says it: "std::vector<long>", "Rect",
	// "geo::Shape".
	DeclType DeclKind = "type"
	// DeclFunc asks for the overload set of a qualified function name: "fopen",
	// "std::min", "geo::f".
	DeclFunc DeclKind = "func"
)

// A DeclQuery names a declaration to extract.
type DeclQuery struct {
	Kind DeclKind `json:"kind"`
	Name string   `json:"name"`
}

// A DeclTable is the answer to a list of queries.
type DeclTable struct {
	Records map[string]*Record  `json:"records,omitempty"`
	Enums   map[string]*Enum    `json:"enums,omitempty"`
	Funcs   map[string]*FuncSet `json:"funcs,omitempty"`

	// Canon maps every type name that was asked for to its canonical spelling
	// (clang's): "FILE" to "_IO_FILE", "GLenum" to "unsigned int", "geo::Vec" to
	// itself. Records and Enums are keyed by the canonical spelling.
	Canon map[string]string `json:"canon,omitempty"`

	// Errors maps a query ("type Rect", "func geo::f") to why there is no
	// answer: the compiler's message, "not a class or enum", a limit.
	Errors map[string]string `json:"errors,omitempty"`

	// HeaderErrors are the compiler's errors that belong to no query.
	HeaderErrors []Diag `json:"headerErrors,omitempty"`

	// Runs is the number of compiler runs this table needed (0 when all came from
	// the memo), Deps the headers they read.
	Runs   int      `json:"runs"`
	Deps   []string `json:"deps,omitempty"`
	Cached bool     `json:"cached,omitempty"`

	Timing Timing `json:"timing"`
	Stats  Stats  `json:"stats"`
}

// QueryKey returns the key of a query in Errors: "type Rect", "func geo::f".
func QueryKey(q DeclQuery) string { return string(q.Kind) + " " + q.Name }
