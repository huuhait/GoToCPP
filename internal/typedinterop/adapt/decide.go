package adapt

import (
	"go/constant"
	"go/types"
	"math/big"
	"strings"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
)

// The refusals of the rows: what Decide says when an argument that a row would adapt breaks the
// rule of the row. Each message names the Go source and says the way out.

// decideCStr: a runtime string to a const char* parameter. It is refused when the callee keeps the
// pointer, and at the format place of a printf-like function.
func decideCStr(c *Call, i int, key Key) Decision {
	if d, bad := stringKeeps(c, i, key); bad {
		return d
	}
	if d, bad := formatGuard(c, i); bad {
		return d
	}
	return convert(key)
}

// decideWide: a string to a wide character pointer. It is refused when the callee keeps the
// pointer.
func decideWide(c *Call, i int, key Key) Decision {
	if d, bad := stringKeeps(c, i, key); bad {
		return d
	}
	return convert(key)
}

// stringKeeps reports the refusal when the callee is known to keep the pointer of the string
// argument i or to hand back a pointer into it, for a narrow or a wide string alike (keeps.go): a
// function on the list, a method of a class that has a pointer member of the character type, a
// char pointer that the call stores into (&end), a function whose result points into the string.
func stringKeeps(c *Call, i int, key Key) (Decision, bool) {
	chars := NarrowChars
	char := "char"
	text := c.Source(i)
	retain := "use C.CString(" + text + ")"
	holder := "C.cstr"
	if key.Row == RowWide {
		chars = WideChars(key.Name)
		char = key.Name
		retain, holder = retainWide(char, text)
	}
	argText := c.Source
	if why, keeps := KeepsPointer(c.Name, len(c.Args), argText); keeps {
		return refuse(i, "%s keeps the pointer that it is given (%s), and a Go string's bytes "+
			"live only until the end of the statement: %s and C.free it when the library "+
			"is done with it", c.Name, why, retain), true
	}
	if c.MethodKeepsCharPtr != nil {
		if why, keeps := c.MethodKeepsCharPtr(chars); keeps {
			return refuse(i, "%s may keep the pointer that it is given (%s), and a Go string's "+
				"bytes live only until the end of the statement: %s and C.free it when the "+
				"object is done with it", c.Name, why, retain), true
		}
	}
	if out := charPtrOutParam(c, i, chars); out >= 0 {
		return refuse(out, "%s is given the address of a %s pointer (%s) next to the "+
			"string %s: the pointer that it stores can point into the string, whose C string "+
			"lives only until the end of the statement: %s and C.free it when you are "+
			"done with the pointer", c.Name, char, c.Source(out), text, retain), true
	}
	if aliasFuncs[LastName(c.Name)] && c.AliasUseOK != nil && !c.AliasUseOK() {
		return refuse(AtCall, "%s returns a pointer into its argument %s, which as a C string "+
			"lives only until the end of the statement: use it in the statement, or keep the "+
			"string alive with a variable, h := %s(%s)", c.Name, text, holder, text), true
	}
	return Decision{}, false
}

// retainWide returns the advice for a wide string that a callee keeps, and the holder that keeps it
// for the statements after: C.WString and C.wstr are for wchar_t; there is no helper for char16_t
// and char32_t (the string would have to be copied into memory from C.malloc).
func retainWide(char, text string) (advice, holder string) {
	if char == "wchar_t" {
		return "use C.WString(" + text + ")", "C.wstr"
	}
	return "copy the code units of " + text + " into memory from C.malloc", "C.cstr"
}

// charPtrOutParam returns the index of the argument of the call c, other than the string argument
// number skip, that is the address of a char pointer (&end of a *C.char variable: char **), or -1:
// the function can store a pointer into the string there (strtol, sqlite3_prepare_v2's tail).
func charPtrOutParam(c *Call, skip int, cs CharSet) int {
	for j, a := range c.Args {
		if j != skip && a.CharPtrAddr != "" && cs.HasName(a.CharPtrAddr) {
			return j
		}
	}
	return -1
}

// formatGuard reports the refusal of a runtime string as the format of a printf-like function: the
// parameter is a format string by the compiler's own rule, so a % in the text is a conversion, and
// nothing in the Go types says whether it should be (the format-string bug).
func formatGuard(c *Call, i int) (Decision, bool) {
	reached := 0
	for _, s := range c.Sigs {
		if i >= len(s.T.Params) {
			continue // a variadic overload: the argument is not its format
		}
		if s.FmtKind != "printf" || s.FmtParam != i+1 {
			return Decision{}, false // an overload with no format there: C++ would choose among them
		}
		reached++
	}
	if reached == 0 {
		return Decision{}, false
	}
	text := c.Source(i)
	return refuse(i, "%s is printf-like (its parameter %d is a format string), and %s "+
		"would be read as a format: a %% in its text is a conversion (the format-string bug); "+
		"pass it as an argument, %s, or write C.cstr(%s) when it is a format on purpose",
		c.Name, i+1, text, formatHint(c, i), text), true
}

// formatHint returns the call c with the runtime string number i passed as an argument of "%s":
// the callee as written, the format argument replaced by "%s" and the string next to it.
func formatHint(c *Call, i int) string {
	args := make([]string, 0, len(c.Args)+1)
	for j := range c.Args {
		if j == i {
			args = append(args, `"%s"`, c.Source(j))
			continue
		}
		args = append(args, c.Source(j))
	}
	return c.Source(AtCall) + "(" + strings.Join(args, ", ") + ")"
}

// decideScalarPtr: &x, x a variable of a Go number type, to a pointer to a C number type. The
// callee reads and perhaps writes the temporary that the lowering makes, so the copy back must be
// lossless (the rules are in ADR-130).
func decideScalarPtr(m *ftype.Mapper, c *Call, i int, key Key) Decision {
	a := c.Args[i]
	kind := a.VarKind
	target := m.Target()
	name := key.Name
	isConst := key.IsConst
	if GoCName(kind, target) == name {
		return leave() // the same C++ type: the pointer converts by itself
	}
	cm := m.MapIn(&ctype.Type{Kind: ctype.Builtin, Name: name}, "")
	if cm.Type == nil {
		return leave()
	}
	cb, ok := cm.Type.Underlying().(*types.Basic)
	if !ok {
		return leave()
	}
	goNum, _ := numericOf(kind, target)
	cNum, ok := numericOf(cb.Kind(), target)
	if !ok {
		return leave()
	}
	text := a.Var
	switch {
	case goNum.float != cNum.float:
		return refuse(i, "%s takes a pointer to %s, and &%s is a pointer to %s: "+
			"C converts neither way through a pointer; declare %s with the C type (C.%s)",
			c.Name, name, text, types.Typ[kind].Name(), text, CgoName(name))
	case isConst && !fits(goNum, cNum) && goNum.float:
		return refuse(i, "%s takes a pointer to const %s, and %s (%s) is wider than %s: "+
			"the copy would round it; declare %s with the C type (C.%s)",
			c.Name, name, text, types.Typ[kind].Name(), name, text, CgoName(name))
	case !isConst && !fits(cNum, goNum):
		return refuse(i, "%s takes a pointer to %s that it may write, and &%s points to %s "+
			"that cannot hold every %s (the copy's value would be cut when it is stored back): "+
			"declare %s with the C type (C.%s)",
			c.Name, name, text, types.Typ[kind].Name(), name, text, CgoName(name))
	}
	return convert(key)
}

// decideVoidOut: &p, p a *C.T, to a void** parameter. The variable gets the pointer when the call
// ends, which a deferred or spawned call cannot do.
func decideVoidOut(c *Call, i int, key Key) Decision {
	if c.Deferred {
		return refuse(i, "%s: the address of a pointer variable for a void** parameter "+
			"cannot be the argument of a deferred or spawned call (the variable gets the pointer "+
			"when the call ends): call it in a function and defer that", c.Name)
	}
	return convert(key)
}

// decideEnum: an untyped integer constant to an enum parameter is cast, if the value is one that
// C++ defines for the enum.
func decideEnum(m *ftype.Mapper, c *Call, i int, key Key) Decision {
	a := c.Args[i]
	canon := key.Name
	v := constant.ToInt(a.Const.Value)
	if v.Kind() != constant.Int {
		return leave()
	}
	n := new(big.Int)
	switch x := constant.Val(v).(type) {
	case int64:
		n.SetInt64(x)
	case *big.Int:
		n.Set(x)
	default:
		return leave()
	}
	lo, hi, ok := enumRange(m, canon)
	if !ok {
		return leave()
	}
	if n.Cmp(lo) < 0 || n.Cmp(hi) > 0 {
		return refuse(i, "%s: the constant %s is outside the values that C++ defines for "+
			"the enum %s (%s to %s): pass an enumerator by name", c.Name, n, canon, lo, hi)
	}
	return convert(key)
}

// enumRange returns the values that C++ defines for the enum canon: the range of its underlying
// type for an enum class, else the range of the smallest bit-field that holds every enumerator
// ([dcl.enum]; a value outside it is undefined behavior for an enum with no fixed type).
func enumRange(m *ftype.Mapper, canon string) (lo, hi *big.Int, ok bool) {
	e := m.Decls().Enums[canon]
	if e == nil {
		return nil, nil, false
	}
	target := m.Target()
	if e.Scoped {
		if e.Underlying.T == nil {
			return nil, nil, false
		}
		um := m.MapIn(e.Underlying.T, "")
		if um.Type == nil {
			return nil, nil, false
		}
		b, isBasic := um.Type.Underlying().(*types.Basic)
		if !isBasic {
			return nil, nil, false
		}
		nu, good := numericOf(b.Kind(), target)
		if !good {
			return nil, nil, false
		}
		return intRange(nu)
	}
	lowest, highest := new(big.Int), new(big.Int)
	for _, ev := range e.Values {
		if ev.Value == nil {
			return nil, nil, false
		}
		if ev.Value.Cmp(lowest) < 0 {
			lowest.Set(ev.Value)
		}
		if ev.Value.Cmp(highest) > 0 {
			highest.Set(ev.Value)
		}
	}
	if lowest.Sign() >= 0 {
		bits := highest.BitLen()
		hi = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(bits)), big.NewInt(1))
		return big.NewInt(0), hi, true
	}
	for bits := 1; bits < 64; bits++ {
		half := new(big.Int).Lsh(big.NewInt(1), uint(bits-1))
		lo = new(big.Int).Neg(half)
		hi = new(big.Int).Sub(half, big.NewInt(1))
		if lowest.Cmp(lo) >= 0 && highest.Cmp(hi) <= 0 {
			return lo, hi, true
		}
	}
	return nil, nil, false
}

// decideNumConst: an untyped number constant to a number parameter is Go's check: the value must
// be representable in the parameter's type (300 for a char, -1 for unsigned, 2.5 for int and 1e60
// for float are errors, as in cgo); C++ would wrap or truncate silently. The argument itself is
// passed on.
func decideNumConst(c *Call, i int, key Key) Decision {
	a := c.Args[i]
	why, ok := Representable(a.Const, key.Kind)
	if ok {
		return leave()
	}
	to := "C." + CgoName(key.Name)
	desc := ConstDesc(c.Source(i), a.Const, a.Const.Value)
	return refuse(i, "cannot use %s as %s value in argument to %s%s", desc, to, c.Name, why)
}
