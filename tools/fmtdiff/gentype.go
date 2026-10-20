package main

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

// The random values of a program: types built from a small grammar (slices, arrays, maps,
// structs, nil pointers, interfaces, the named types of the prelude) with values for them, so
// that the verbs meet nestings the catalog does not have (a map of arrays of structs with
// unexported fields, %#v of a slice of interfaces of maps, ...).

type rtype struct {
	kind   string // basic, named, slice, array, map, struct, ptr, iface
	name   string // basic and named: the Go spelling
	elem   *rtype
	key    *rtype
	n      int
	fields []rfield
}

type rfield struct {
	name     string
	t        *rtype
	embedded bool
}

func (t *rtype) String() string {
	switch t.kind {
	case "basic", "named":
		return t.name
	case "slice":
		return "[]" + t.elem.String()
	case "array":
		return fmt.Sprintf("[%d]%s", t.n, t.elem)
	case "map":
		return "map[" + t.key.String() + "]" + t.elem.String()
	case "ptr":
		return "*" + t.elem.String()
	case "iface":
		return t.name
	case "struct":
		var b strings.Builder
		b.WriteString("struct{")
		for i, f := range t.fields {
			if i > 0 {
				b.WriteString("; ")
			}
			if f.embedded {
				b.WriteString(f.t.String())
			} else {
				b.WriteString(f.name + " " + f.t.String())
			}
		}
		b.WriteString("}")
		return b.String()
	}
	return "?"
}

var basicTypes = []string{"bool", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64", "complex128", "string", "rune", "byte"}

// the named types of the prelude that a random type can use, with how to write a value
var namedTypes = []struct {
	name string
	vals []string
}{
	{"MyInt", []string{"0", "7", "-3"}},
	{"MyStr", []string{`""`, `"s"`, `"a b"`}},
	{"Color", []string{"0", "1", "4"}},
	{"StrV", []string{"StrV{1}", "StrV{}"}},
	{"StrP", []string{"StrP{2}"}},
	{"ErrV", []string{"ErrV{1}"}},
	{"Pt", []string{"Pt{1, 2}", "Pt{}", "Pt{-1, 3}"}},
	{"Inner", []string{`Inner{1, "x"}`, "Inner{}"}},
	{"GoS", []string{"GoS{1}"}},
	{"Fmtr", []string{"Fmtr{1}"}},
	{"Empty", []string{"Empty{}"}},
	{"MyBytes", []string{`MyBytes("ab")`, "MyBytes(nil)"}},
	{"MyBool", []string{"true", "false"}},
	{"MyF", []string{"1.5", "0"}},
}

var mapKeys = []string{"int", "string", "bool", "uint8", "float64", "Pt", "MyStr", "Color", "StrV", "[2]int", "rune", "int8"}

func (g *Gen) randType(r *rand.Rand, depth int) *rtype {
	if depth <= 0 {
		return g.leafType(r)
	}
	switch x := r.IntN(100); {
	case x < 22:
		return g.leafType(r)
	case x < 40:
		return &rtype{kind: "slice", elem: g.randType(r, depth-1)}
	case x < 50:
		return &rtype{kind: "array", n: r.IntN(4), elem: g.randType(r, depth-1)}
	case x < 64:
		k := &rtype{kind: "basic", name: pick(r, mapKeys)}
		return &rtype{kind: "map", key: k, elem: g.randType(r, depth-1)}
	case x < 84:
		n := 1 + r.IntN(3)
		st := &rtype{kind: "struct"}
		for i := 0; i < n; i++ {
			name := fmt.Sprintf("F%d", i)
			if r.IntN(3) == 0 {
				name = fmt.Sprintf("f%d", i)
			}
			f := rfield{name: name, t: g.randType(r, depth-1)}
			if i == 0 && r.IntN(8) == 0 {
				e := pick(r, []string{"Pt", "Inner", "StrV", "ErrV"})
				f = rfield{name: e, t: &rtype{kind: "named", name: e}, embedded: true}
			}
			st.fields = append(st.fields, f)
		}
		return st
	case x < 91:
		return &rtype{kind: "ptr", elem: g.randType(r, depth-1)}
	default:
		return &rtype{kind: "iface", name: pick(r, []string{"any", "any", "error", "fmt.Stringer"})}
	}
}

func (g *Gen) leafType(r *rand.Rand) *rtype {
	if r.IntN(100) < 40 {
		n := pick(r, namedTypes)
		return &rtype{kind: "named", name: n.name}
	}
	return &rtype{kind: "basic", name: pick(r, basicTypes)}
}

func basicLit(r *rand.Rand, name string) string {
	switch name {
	case "bool":
		return pick(r, []string{"true", "false"})
	case "string":
		return pick(r, []string{`""`, `"a"`, `"hi there"`, `"é世"`, `"\xff"`, `"x\ty"`})
	case "float32", "float64":
		return pick(r, []string{"0", "1", "-1.5", "0.1", "1e21", "1e-7", "100000", "2.5"})
	case "complex128":
		return pick(r, []string{"0", "1+2i", "-1.5i"})
	case "rune":
		return pick(r, []string{"'a'", "0x263a", "10", "0", "-1"})
	case "byte", "uint8":
		return pick(r, []string{"0", "1", "65", "255"})
	case "int8":
		return pick(r, []string{"0", "1", "-128", "127"})
	case "int16", "int32", "int64", "int":
		return pick(r, []string{"0", "1", "-1", "42", "1000"})
	}
	return pick(r, []string{"0", "1", "7", "255", "1000"}) // unsigned
}

func (g *Gen) randValue(r *rand.Rand, t *rtype, depth int) string {
	switch t.kind {
	case "basic":
		return basicLit(r, t.name)
	case "named":
		for _, n := range namedTypes {
			if n.name == t.name {
				return pick(r, n.vals)
			}
		}
	case "slice":
		if r.IntN(5) == 0 {
			return t.String() + "(nil)"
		}
		n := r.IntN(4)
		return t.String() + "{" + g.elems(r, t.elem, n) + "}"
	case "array":
		return t.String() + "{" + g.elems(r, t.elem, t.n) + "}"
	case "map":
		if r.IntN(6) == 0 {
			return t.String() + "(nil)"
		}
		n := r.IntN(4)
		var parts []string
		seen := map[string]bool{}
		for i := 0; i < n; i++ {
			k := g.mapKey(r, t.key.name)
			if seen[k] {
				continue
			}
			seen[k] = true
			parts = append(parts, k+": "+g.randValue(r, t.elem, depth-1))
		}
		return t.String() + "{" + strings.Join(parts, ", ") + "}"
	case "struct":
		var parts []string
		for _, f := range t.fields {
			parts = append(parts, g.randValue(r, f.t, depth-1))
		}
		return t.String() + "{" + strings.Join(parts, ", ") + "}"
	case "ptr":
		return "(" + t.String() + ")(nil)"
	case "iface":
		switch t.name {
		case "error":
			return pick(r, []string{"error(nil)", "error(ErrV{2})", "error(ErrV{3})"})
		case "fmt.Stringer":
			return pick(r, []string{"fmt.Stringer(nil)", "fmt.Stringer(StrV{4})", "fmt.Stringer(Color(1))"})
		}
		if depth > 0 && r.IntN(100) < 70 {
			// an interface holding a value of a random simpler type
			et := g.randType(r, 1)
			if et.kind != "ptr" && et.kind != "iface" {
				return "any(" + g.randValue(r, et, 1) + ")"
			}
		}
		return pick(r, []string{"any(nil)", "any(1)", `any("s")`, "any(2.5)", "any(Pt{1, 2})", "any([]int{1})"})
	}
	return "nil"
}

func (g *Gen) elems(r *rand.Rand, t *rtype, n int) string {
	var parts []string
	for i := 0; i < n; i++ {
		parts = append(parts, g.randValue(r, t, 2))
	}
	return strings.Join(parts, ", ")
}

func (g *Gen) mapKey(r *rand.Rand, name string) string {
	switch name {
	case "Pt":
		return pick(r, []string{"Pt{1, 2}", "Pt{}", "Pt{-1, 3}", "Pt{1, 1}"})
	case "StrV":
		return pick(r, []string{"StrV{1}", "StrV{2}", "StrV{}"})
	case "[2]int":
		return pick(r, []string{"[2]int{1, 2}", "[2]int{}", "[2]int{0, 5}"})
	case "Color":
		return pick(r, []string{"0", "1", "2"})
	case "MyStr":
		return pick(r, []string{`"a"`, `"b"`, `"c"`})
	case "float64":
		return pick(r, []string{"0", "1.5", "-2", "1e10"})
	case "bool":
		return pick(r, []string{"true", "false"})
	case "uint8", "int8":
		return pick(r, []string{"0", "1", "5", "100"})
	}
	if name == "string" {
		return pick(r, []string{`"a"`, `"b"`, `"c"`, `"é"`, `""`})
	}
	return pick(r, []string{"0", "1", "2", "-1", "100"})
}

// withExtras returns a copy of g with random values added.
func (g *Gen) withExtras(r *rand.Rand) *Gen {
	c := &Gen{Vals: append([]Val(nil), g.Vals...)}
	n := 24
	for i := 0; i < n; i++ {
		t := g.randType(r, 1+r.IntN(3))
		if t.kind == "ptr" || t.kind == "iface" && r.IntN(2) == 0 {
			continue
		}
		v := g.randValue(r, t, 3)
		idx := len(c.Vals)
		ts := t.String()
		if t.kind == "iface" {
			ts = t.name
		}
		c.Vals = append(c.Vals, Val{Op: fmt.Sprintf("r%d", idx), Decl: ts, Init: v, Class: "comp", Comp: true, Describe: ts + " = " + v, PtrIn: false})
		c.Extra = append(c.Extra, idx)
	}
	c.Class = classOf(c.Vals)
	return c
}
