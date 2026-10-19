package gen

import (
	"fmt"
)

// Kind is the kind of a generator type. The generator models only what it needs to produce
// type-correct programs whose behaviour the Go specification fully defines.
type Kind uint8

const (
	KInt    Kind = iota // any integer type, also a defined type over one (Named)
	KFloat              // float32 or float64 (Bits)
	KBool               // bool
	KString             // string
	KArray              // [Len]Elem
	KSlice              // []Elem
	KMap                // map[Key]Elem
	KStruct             // a defined struct type (Name) with Fields
	KPtr                // *Elem
	KIface              // a defined interface type, or any, or error
	KFunc               // a function type (only as variables made by scenarios)
	KChan               // chan Elem
)

// Field is a struct field.
type Field struct {
	Name     string
	T        *Type
	Embedded bool
}

// Method is a method of a defined type. Pure methods have no side effects and are total, so
// expressions may call them.
type Method struct {
	Name    string
	Ptr     bool // pointer receiver
	Params  []*Type
	Result  *Type // nil: no result
	Pure    bool
	Comment string
}

// Type is a generator type.
type Type struct {
	K      Kind
	Name   string // the source spelling, qualified by the program prefix for defined types
	Bits   int    // KInt, KFloat: 8..64 (int and uint: 64)
	Signed bool   // KInt
	Named  bool   // a defined type (struct, interface, or a type over a basic type)
	Elem   *Type  // KArray, KSlice, KMap (value), KPtr, KChan
	Key    *Type  // KMap
	Len    int    // KArray
	Fields []Field
	Meths  []Method
	Under  string // for a defined basic type: the underlying type's name
	// Printable reports that fmt prints the value without an address, so the output is the
	// same in every run: no pointers below the top level, no funcs, no channels.
	NoPtr bool
}

var (
	tBool   = &Type{K: KBool, Name: "bool", NoPtr: true}
	tString = &Type{K: KString, Name: "string", NoPtr: true}
	tF64    = &Type{K: KFloat, Name: "float64", Bits: 64, NoPtr: true}
	tF32    = &Type{K: KFloat, Name: "float32", Bits: 32, NoPtr: true}
	intTs   = mkInts()
)

func mkInts() map[string]*Type {
	m := map[string]*Type{}
	for _, s := range []struct {
		n    string
		bits int
		sgn  bool
	}{
		{"int", 64, true}, {"int8", 8, true}, {"int16", 16, true}, {"int32", 32, true}, {"int64", 64, true},
		{"uint", 64, false}, {"uint8", 8, false}, {"uint16", 16, false}, {"uint32", 32, false}, {"uint64", 64, false},
	} {
		m[s.n] = &Type{K: KInt, Name: s.n, Bits: s.bits, Signed: s.sgn, NoPtr: true}
	}
	return m
}

// intNames lists the integer types, with the weight the generator picks them with.
var intNames = []string{"int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64",
	"int8", "uint8", "int16", "uint16", "int32", "uint32"}

func sliceOf(e *Type) *Type {
	return &Type{K: KSlice, Name: "[]" + e.Name, Elem: e, NoPtr: e.NoPtr}
}

func arrayOf(e *Type, n int) *Type {
	return &Type{K: KArray, Name: fmt.Sprintf("[%d]%s", n, e.Name), Elem: e, Len: n, NoPtr: e.NoPtr}
}

func mapOf(k, v *Type) *Type {
	return &Type{K: KMap, Name: "map[" + k.Name + "]" + v.Name, Key: k, Elem: v, NoPtr: k.NoPtr && v.NoPtr}
}

func ptrTo(e *Type) *Type {
	return &Type{K: KPtr, Name: "*" + e.Name, Elem: e}
}

// min and max value of an integer type as decimal strings.
func intRange(t *Type) (lo, hi string) {
	if t.Signed {
		switch t.Bits {
		case 8:
			return "-128", "127"
		case 16:
			return "-32768", "32767"
		case 32:
			return "-2147483648", "2147483647"
		}
		return "-9223372036854775808", "9223372036854775807"
	}
	switch t.Bits {
	case 8:
		return "0", "255"
	case 16:
		return "0", "65535"
	case 32:
		return "0", "4294967295"
	}
	return "0", "18446744073709551615"
}

// comparable reports whether == is defined for the type without a run-time panic.
func (t *Type) comparable() bool {
	switch t.K {
	case KInt, KFloat, KBool, KString, KPtr, KChan:
		return true
	case KArray:
		return t.Elem.comparable()
	case KStruct:
		for _, f := range t.Fields {
			if !f.T.comparable() {
				return false
			}
		}
		return true
	case KIface:
		return false // dynamic types may not be comparable
	}
	return false
}

func (t *Type) isBasic() bool {
	return t.K == KInt || t.K == KFloat || t.K == KBool || t.K == KString
}
