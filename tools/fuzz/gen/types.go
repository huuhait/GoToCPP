package gen

import (
	"fmt"
	"strings"
)

// The type model of the generator. Types are compared by their String (a named type has a name
// that is unique in the program), which is how Go's identity works for everything the generator
// builds.

type kind int

const (
	kInt kind = iota
	kInt64
	kInt32
	kUint8
	kUint
	kFloat
	kString
	kBool
	kNamed // a named basic type: base is the basic type
	kSlice
	kArray
	kMap
	kPtr
	kStruct
	kIface
	kFunc
	kChan
	kAny
)

type typ struct {
	kind kind
	name string // a named type
	base *typ   // kNamed: the basic type

	elem, key *typ // slice, array, map, ptr, chan; key of a map
	n         int  // array length

	fields  []*field // struct: the own fields, embedded ones included
	embeds  []*typ   // struct: the embedded structs, by value
	methods []*method

	impls []*typ // interface: the struct types whose pointer implements it

	params, results []*typ // func
}

type field struct {
	name     string
	t        *typ
	embedded bool
}

type method struct {
	name            string
	ptr             bool
	params, results []*typ
}

var (
	tInt    = &typ{kind: kInt}
	tInt64  = &typ{kind: kInt64}
	tInt32  = &typ{kind: kInt32}
	tUint8  = &typ{kind: kUint8}
	tUint   = &typ{kind: kUint}
	tFloat  = &typ{kind: kFloat}
	tString = &typ{kind: kString}
	tBool   = &typ{kind: kBool}
	tAny    = &typ{kind: kAny}

	// basics are the basic types, the simplest first.
	basics = []*typ{tInt, tString, tBool, tFloat, tInt64, tInt32, tUint8, tUint}
)

func (t *typ) String() string {
	switch t.kind {
	case kInt:
		return "int"
	case kInt64:
		return "int64"
	case kInt32:
		return "int32"
	case kUint8:
		return "uint8"
	case kUint:
		return "uint"
	case kFloat:
		return "float64"
	case kString:
		return "string"
	case kBool:
		return "bool"
	case kAny:
		return "any"
	case kNamed, kStruct, kIface:
		return t.name
	case kSlice:
		return "[]" + t.elem.String()
	case kArray:
		return fmt.Sprintf("[%d]%s", t.n, t.elem)
	case kMap:
		return "map[" + t.key.String() + "]" + t.elem.String()
	case kPtr:
		return "*" + t.elem.String()
	case kChan:
		return "chan " + t.elem.String()
	case kFunc:
		return "func(" + typeList(t.params) + ")" + resultList(t.results)
	}
	panic("gen: bad type")
}

func typeList(ts []*typ) string {
	s := make([]string, len(ts))
	for i, t := range ts {
		s[i] = t.String()
	}
	return strings.Join(s, ", ")
}

func resultList(ts []*typ) string {
	switch len(ts) {
	case 0:
		return ""
	case 1:
		return " " + ts[0].String()
	}
	return " (" + typeList(ts) + ")"
}

// under returns the basic type of a named basic type, else t.
func (t *typ) under() *typ {
	if t.kind == kNamed {
		return t.base
	}
	return t
}

func (t *typ) isInteger() bool {
	switch t.under().kind {
	case kInt, kInt64, kInt32, kUint8, kUint:
		return true
	}
	return false
}

func (t *typ) isNumeric() bool { return t.isInteger() || t.under().kind == kFloat }
func (t *typ) isString() bool  { return t.under().kind == kString }
func (t *typ) isBool() bool    { return t.under().kind == kBool }
func (t *typ) isOrdered() bool { return t.isNumeric() || t.isString() }

// isScalar reports whether t is a basic type or a named basic type.
func (t *typ) isScalar() bool { return t.isNumeric() || t.isString() || t.isBool() }

// isUnsigned reports whether t is an unsigned integer type.
func (t *typ) isUnsigned() bool {
	k := t.under().kind
	return k == kUint8 || k == kUint
}

// comparable reports whether == is defined on t.
func (t *typ) comparable() bool {
	switch t.kind {
	case kSlice, kMap, kFunc:
		return false
	case kArray:
		return t.elem.comparable()
	case kStruct:
		for _, f := range t.fields {
			if !f.t.comparable() {
				return false
			}
		}
	}
	return true
}

// nilable reports whether nil is a value of t.
func (t *typ) nilable() bool {
	switch t.kind {
	case kSlice, kMap, kPtr, kFunc, kChan, kIface, kAny:
		return true
	}
	return false
}

// A promoted is a field or a method that a struct has, with the path of embedded fields that
// leads to it ("" for an own member).
type member struct {
	path string // "E1.E2." or ""
	f    *field
	m    *method
}

// allFields returns the fields of the struct t, own and promoted (their names are unique in the
// program, so no selector is ambiguous), with the selector path.
func (t *typ) allFields() []member {
	var out []member
	var walk func(t *typ, path string)
	walk = func(t *typ, path string) {
		for _, f := range t.fields {
			if f.embedded {
				walk(f.t, path+f.name+".")
			} else {
				out = append(out, member{path: path, f: f})
			}
		}
	}
	if t.kind == kStruct {
		walk(t, "")
	}
	return out
}

// allMethods returns the methods of t (a struct or a named type), own and promoted.
func (t *typ) allMethods() []member {
	var out []member
	var walk func(t *typ, path string)
	walk = func(t *typ, path string) {
		for _, m := range t.methods {
			out = append(out, member{path: path, m: m})
		}
		for _, f := range t.fields {
			if f.embedded {
				walk(f.t, path+f.name+".")
			}
		}
	}
	if t.kind == kStruct || t.kind == kNamed {
		walk(t, "")
	}
	return out
}

// methodNames returns the names of the methods of t, own and promoted.
func (t *typ) methodNames() map[string]bool {
	out := map[string]bool{}
	for _, m := range t.allMethods() {
		out[m.m.name] = true
	}
	return out
}

func sameSig(a, b []*typ) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].String() != b[i].String() {
			return false
		}
	}
	return true
}

// implements reports whether *s has all the methods of the interface i.
func implements(s, i *typ) bool {
	have := s.allMethods()
	for _, im := range i.methods {
		ok := false
		for _, h := range have {
			if h.m.name == im.name && sameSig(h.m.params, im.params) && sameSig(h.m.results, im.results) {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return true
}
