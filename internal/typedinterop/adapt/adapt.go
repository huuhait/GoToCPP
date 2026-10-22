// Package adapt decides what happens to the argument of a call into C/C++ in typed interop
// (ADR-130, ADR-134): the one table of argument adaptations, and the errors of its rows.
//
// Typed mode knows the C++ type of every parameter of a function it calls (clang says it), so an
// argument that C++ would refuse, and whose meaning is certain from the two types alone, is
// converted by the compiler; an argument that breaks a rule of its row (a constant that does not
// fit the parameter, a runtime string at a format place) is an error. Both are decided here, by
// functions of data only: the Go argument (its class, its constant, the text it was written as),
// the callee (its overloads: the parameter types clang gave, with their spelling) and the mapper
// that holds the declarations. Nothing in this package knows the syntax tree of the program, the
// code that is written or the compiler that is run.
//
// Two callers use the decision:
//
//   - the lowering (internal/lower) writes the C++ of an adapted argument and reports a refusal as
//     an error of the build;
//   - the interop checker (internal/typedinterop/icheck) and the language server report the same
//     refusals, with the same words at the same place, without lowering anything, so that
//     `gocpp check`, `gocpp vet` and the editor show what `gocpp build` would stop at.
//
// So a rule is written once: a new row is a class of argument, a function that says what a
// parameter makes of it, and its refusals, here, and its C++ in the lowering.
package adapt

import (
	"fmt"
	"go/types"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
)

// A Class is the kind of Go argument that a row adapts, decided from the Go types alone.
type Class int

const (
	ClassNone        Class = iota
	ClassString            // a Go string that is not a constant
	ClassStringConst       // a Go string constant (already a C literal)
	ClassAddrScalar        // &x, x a variable (or field) of a Go number type
	ClassAddrCPtr          // &p, p a variable (or field) of type *C.T
	ClassNumConst          // an untyped number constant (an integer, a rune or a float)
	ClassFloat32           // a Go float32 value that is not a constant
	ClassFloat64           // a Go float64 value that is not a constant
)

// The rows of the table (Key.Row).
const (
	RowCStr      = "cstr"
	RowStdString = "stdstring"
	RowWide      = "wide"
	RowScalarPtr = "scalarptr"
	RowVoidOut   = "voidout"
	RowEnum      = "enum"
	RowNumConst  = "numconst"
	RowFloatConv = "floatconv"
)

// A Verdict is what a parameter makes of an argument class.
type Verdict int

const (
	Other  Verdict = iota // C++'s own rules decide, or it is not known: leave the call alone
	Ignore                // can never take it: the overload is no candidate for this argument
	Adapt                 // the row adapts it
)

// A Key says which row and with what detail. The overloads of a call must all have the same key
// for an argument to be adapted.
type Key struct {
	Row string
	// Name is the C++ type that the row works with: the character type of a wide string, the
	// pointee of a scalar pointer, the enum, the integer or float of a conversion.
	Name string
	// IsConst: the pointee of a scalar pointer is const (the callee only reads it).
	IsConst bool
	// Kind is the Go kind of the parameter of a constant (numconst); Name is then how the C type of
	// the parameter is spelled in the messages.
	Kind types.BasicKind
}

// A Sig is one overload of the callee: its function type with the scope its names are written in,
// and what the declaration says about its parameters.
type Sig struct {
	T     *ctype.Type
	Scope string

	// FmtKind and FmtParam: the overload's parameter number FmtParam (from 1) is a format string
	// ("printf" or "scanf") by the compiler's own rule (probe.Overload.FormatKind).
	FmtKind  string
	FmtParam int

	// Arrays says for each parameter whether the declaration writes it as an array
	// (`float col[3]`: a pointer for the compiler, several elements for the callee); nil when it
	// is not known (the probe did not tell). ArrayAt is the question to ask.
	Arrays []bool
}

// ArrayAt reports whether the parameter number i may be an array parameter: it is written as one,
// or it is not known how it is written. The address of one Go variable is not passed for such a
// parameter.
func (s Sig) ArrayAt(i int) bool {
	return i >= len(s.Arrays) || s.Arrays[i]
}

// An Arg is one argument of a call as the decision sees it. Class decides which rows look at it;
// the other fields are the facts of its class.
type Arg struct {
	Class Class

	// Const is the type and value of an untyped constant (ClassNumConst).
	Const types.TypeAndValue

	// Var is the variable of &x as written and VarKind its Go number kind (ClassAddrScalar).
	Var     string
	VarKind types.BasicKind

	// CharPtrAddr is the C++ name of the character type ("char", "wchar_t") of the pointer whose
	// address the argument is (&end of a char pointer), or "": a function that is given the address
	// of such a pointer can store a pointer into a string argument there.
	CharPtrAddr string
}

// A Call is a call of a C/C++ function as the decision sees it.
type Call struct {
	// Name is the callee as the messages call it.
	Name string

	// Source returns the text of the argument number i as the program wrote it, and of the callee
	// for i == AtCall. The messages are made of it.
	Source func(i int) string

	// Sigs are the overloads that the name stands for.
	Sigs []Sig

	// Args are the arguments, in order.
	Args []Arg

	// Deferred says that the call is the call of a defer or go statement: its arguments are
	// evaluated now and the call runs later.
	Deferred bool

	// MethodKeepsCharPtr, if the callee is a method of a class of the program that has a data member
	// that is a pointer to a character of cs, says why a setter of it is likely to store the
	// pointer it is given. AliasUseOK says whether the result of the call is used where it is gone
	// by the end of the statement. They are facts that need the syntax tree or the class's
	// declaration; a caller that does not have them leaves them nil and the rule that needs them
	// does not fire.
	MethodKeepsCharPtr func(cs CharSet) (why string, ok bool)
	AliasUseOK         func() bool
}

// An Outcome is what to do with an argument.
type Outcome int

const (
	// Leave: nothing is adapted, the argument is C++'s (the callee is not known, a row does not
	// apply, or the argument is fine as it is).
	Leave Outcome = iota
	// Convert: the row Decision.Key adapts the argument.
	Convert
	// Refuse: the argument breaks a rule of its row; Decision.Msg is the error and Decision.At the
	// place of it.
	Refuse
)

// AtCall is Decision.At for an error that points at the call, and Call.Source's index of the
// callee.
const AtCall = -1

// A Decision is the answer for one argument.
type Decision struct {
	Outcome Outcome
	Key     Key    // the row, for Convert
	Msg     string // the error, for Refuse
	At      int    // for Refuse: the index of the argument the error points at, or AtCall
}

func leave() Decision { return Decision{} }

func convert(k Key) Decision { return Decision{Outcome: Convert, Key: k} }

func refuse(at int, format string, args ...any) Decision {
	return Decision{Outcome: Refuse, At: at, Msg: fmt.Sprintf(format, args...)}
}

// A Row is one row of the table: the classes of argument that it handles and the function that
// says, for one parameter type, what the row makes of the argument.
type Row struct {
	Name    string
	Classes []Class
	// param says what the C++ parameter p (written in the namespace scope) makes of an argument of
	// class c.
	param func(m *ftype.Mapper, c Class, p *ctype.Type, scope string) (Key, Verdict)
}

// Table is the table of argument adaptations. The order is the order in which the rows look at a
// parameter.
var Table = []*Row{
	{Name: RowCStr, Classes: []Class{ClassString}, param: cstrParam},
	{Name: RowStdString, Classes: []Class{ClassString}, param: stdStringParam},
	{Name: RowWide, Classes: []Class{ClassString, ClassStringConst}, param: wideParam},
	{Name: RowScalarPtr, Classes: []Class{ClassAddrScalar}, param: scalarPtrParam},
	{Name: RowVoidOut, Classes: []Class{ClassAddrCPtr}, param: voidOutParam},
	{Name: RowEnum, Classes: []Class{ClassNumConst}, param: enumParam},
	{Name: RowNumConst, Classes: []Class{ClassNumConst}, param: numConstParam},
	{Name: RowFloatConv, Classes: []Class{ClassFloat32, ClassFloat64}, param: floatConvParam},
}

// rowNamed returns the row called name.
func rowNamed(name string) *Row {
	for _, r := range Table {
		if r.Name == name {
			return r
		}
	}
	return nil
}

func (r *Row) handles(c Class) bool {
	for _, k := range r.Classes {
		if k == c {
			return true
		}
	}
	return false
}

// Decide returns what to do with the argument number i of the call c: whether a row adapts it,
// and the error when it breaks the rule of the row.
//
// An argument is adapted only if EVERY overload that the call can reach (an overload with enough
// parameters, or a variadic one) has the same row, with the same detail, at that place, or never
// takes this class of argument at all (Ignore). Then, for each reachable overload, the unadapted
// call was ill-formed (no conversion exists in C++), and the adapted argument has exactly the
// parameter's type, so overload resolution has one viable function for the argument: the one the
// user meant. If one overload is unknown (a class that might have a constructor, a
// std::string_view, a template, a parameter pack), or two overloads adapt differently, nothing is
// adapted and the call is C++'s.
func Decide(m *ftype.Mapper, c *Call, i int) Decision {
	if i < 0 || i >= len(c.Args) {
		return leave()
	}
	a := &c.Args[i]
	if a.Class == ClassNone || len(c.Sigs) == 0 {
		return leave()
	}
	row, key, ok := adaptationFor(m, a.Class, c.Sigs, i)
	if !ok {
		return leave()
	}
	switch row.Name {
	case RowCStr:
		return decideCStr(c, i, key)
	case RowWide:
		return decideWide(c, i, key)
	case RowScalarPtr:
		return decideScalarPtr(m, c, i, key)
	case RowVoidOut:
		return decideVoidOut(c, i, key)
	case RowEnum:
		return decideEnum(m, c, i, key)
	case RowNumConst:
		return decideNumConst(c, i, key)
	}
	return convert(key)
}

// adaptationFor returns the row that adapts the argument number i of class cl in a call of one of
// the overloads sigs, or false: every overload that the call can reach must either never take the
// argument (Ignore) or have the same row with the same key.
func adaptationFor(m *ftype.Mapper, cl Class, sigs []Sig, i int) (*Row, Key, bool) {
	var found *Row
	var key Key
	for _, s := range sigs {
		choice, viable := sigChoice(m, cl, s, i)
		if !viable {
			continue // fewer parameters: not viable for this call
		}
		switch choice.verdict {
		case Ignore:
			continue
		case Other:
			return nil, key, false
		}
		if found != nil && (choice.row != found || choice.key != key) {
			return nil, key, false
		}
		found = choice.row
		key = choice.key
	}
	return found, key, found != nil
}

// A choice is what one overload makes of an argument: the row and key, and the verdict.
type choice struct {
	row     *Row
	key     Key
	verdict Verdict
}

// sigChoice returns what the overload s makes of the argument number i of class cl; viable is
// false when the overload has too few parameters for the call.
func sigChoice(m *ftype.Mapper, cl Class, s Sig, i int) (c choice, viable bool) {
	switch {
	case i < len(s.T.Params):
		c.row, c.key, c.verdict = classifyParam(m, cl, s.T.Params[i], s.Scope)
		if c.verdict == Adapt && (cl == ClassAddrScalar || cl == ClassAddrCPtr) && s.ArrayAt(i) {
			c.verdict = Other // float col[3]: the callee takes several elements, &x is one
		}
		return c, true
	case s.T.Variadic && cl == ClassString:
		return choice{rowNamed(RowCStr), Key{Row: RowCStr}, Adapt}, true // a char* for %s
	case s.T.Variadic:
		return choice{verdict: Other}, true
	}
	return choice{}, false
}

// classifyParam combines the rows of the class cl for the parameter p: the row that adapts it,
// else Ignore when some row knows that the argument can never convert (every Ignore is a fact
// about the parameter's type), else Other.
func classifyParam(m *ftype.Mapper, cl Class, p *ctype.Type, scope string) (*Row, Key, Verdict) {
	all := Other
	for _, r := range Table {
		if !r.handles(cl) {
			continue
		}
		k, v := r.param(m, cl, p, scope)
		switch v {
		case Adapt:
			return r, k, Adapt
		case Ignore:
			all = Ignore
		}
	}
	return nil, Key{}, all
}

// mapParam maps the parameter type p (written in the namespace scope) to the foreign view.
func mapParam(m *ftype.Mapper, p *ctype.Type, scope string) ftype.Mapped {
	return m.MapIn(p, scope)
}
