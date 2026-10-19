package gen

import (
	"fmt"
	"math/bits"
	"strconv"
	"strings"
)

// The expression generator makes TOTAL, PURE expressions: no operation in them can panic (a
// division has a divisor that is never zero, an index is reduced into range, a pointer is
// known to be non-nil) and none has a side effect. Run-time panics are only made by the
// dedicated scenarios, with exactly one possible panic point each, so which panic a program
// reports never depends on the order in which an implementation evaluates operands (the
// unspecified corner that docs/spec/language.md D22 describes).
//
// Every binary operation has an operand that is not a constant, so no constant expression
// overflows (a compile error in Go) and nothing divides a constant by zero.

func (g *Gen) roll(w ...int) int {
	total := 0
	for _, x := range w {
		total += x
	}
	x := g.rn(total)
	for i, c := range w {
		if x < c {
			return i
		}
		x -= c
	}
	return len(w) - 1
}

// ---- literals ----

// intRaw returns a decimal literal that fits the integer type, favouring boundary values.
func (g *Gen) intRaw(t *Type) string {
	b := t.Bits
	var u uint64 // magnitude pattern
	mask := ^uint64(0)
	if b < 64 {
		mask = (uint64(1) << b) - 1
	}
	switch g.roll(30, 10, 12, 8, 40) {
	case 0: // small
		return strconv.Itoa(g.between(-9, 20)*b2i(t.Signed) + g.between(0, 20)*(1-b2i(t.Signed)))
	case 1: // boundaries
		lo, hi := intRange(t)
		return pick(g, []string{lo, hi, "0", "1"})
	case 2: // near boundary
		u = mask >> 1
		if !t.Signed {
			u = mask
		}
		d := uint64(g.between(0, 3))
		if t.Signed && g.chance(50) {
			return strconv.FormatInt(-int64(u)-1+int64(d), 10)
		}
		return strconv.FormatUint(u-d, 10)
	case 3: // power of two
		k := g.rn(b)
		if t.Signed && k == b-1 {
			k--
		}
		return strconv.FormatUint(uint64(1)<<k, 10)
	}
	u = g.r.Uint64() & mask
	if t.Signed {
		v := int64(u<<(64-b)) >> (64 - b) // sign-extend
		return strconv.FormatInt(v, 10)
	}
	return strconv.FormatUint(u, 10)
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func paren(raw string) string {
	if strings.HasPrefix(raw, "-") {
		return "(" + raw + ")"
	}
	return raw
}

// floatRaw returns a decimal literal of a float with a short exact binary expansion.
func (g *Gen) floatRaw() string {
	k := g.between(-400, 400)
	den := pick(g, []int{1, 2, 4, 8, 16})
	v := float64(k) / float64(den)
	s := strconv.FormatFloat(v, 'g', -1, 64)
	if !strings.ContainsAny(s, ".e") {
		s += ".0"
	}
	return s
}

// strRaw returns a quoted string literal: ASCII, multi-byte UTF-8, escapes, invalid UTF-8.
func (g *Gen) strRaw() string {
	words := []string{"", "a", "go", "gocpp", "Hello, World", "x y", "héllo", "日本語", "😀", "naïve", "ß", "Ω≈ç√",
		"tab\there", "line\nbreak", "quote\"s", "back\\slash", "\x00nul", "\xff\xfe", "ab\xc3", "\xe6\x97", "é", " ",
		"\U0001F600x", "z", "0123456789", "abcdefghijklmnopqrstuvwxyz", "Zürich", "\x80", "\xf0\x9f", "mixed é\xffz",
		"\xe0\x80\x80", "\xe0\x9f\xbf", "\xed\xa0\x80", "\xf0\x80\x80\x80", "\xf4\x90\x80\x80", "\xc0\xaf"}
	var b strings.Builder
	n := g.between(1, 3)
	for i := 0; i < n; i++ {
		b.WriteString(pick(g, words))
	}
	return strconv.Quote(b.String())
}

// strNonEmpty is a literal of at least one byte.
func (g *Gen) strNonEmpty() string {
	for {
		s := g.strRaw()
		if s != `""` {
			return s
		}
	}
}

// lit returns a typed constant of a basic type: int8(5), "abc", true, float64(0.5).
func (g *Gen) lit(t *Type) string {
	switch t.K {
	case KInt:
		raw := g.intRaw(t)
		if t.Named {
			return t.Name + "(" + raw + ")"
		}
		return t.Name + "(" + raw + ")"
	case KFloat:
		return t.Name + "(" + g.floatRaw() + ")"
	case KBool:
		return pick(g, []string{"true", "false"})
	case KString:
		return g.strRaw()
	}
	panic("lit: not basic: " + t.Name)
}

// ---- type pool ----

func (g *Gen) randIntType() *Type {
	n := pick(g, intNames)
	return intTs[n]
}

// randBasic picks a basic type.
func (g *Gen) randBasic() *Type {
	switch g.roll(60, 12, 14, 8, 6) {
	case 0:
		return g.randIntType()
	case 1:
		return tString
	case 2:
		return tBool
	case 3:
		if g.has("floats") || true {
			if g.chance(30) {
				return tF32
			}
			return tF64
		}
	}
	// a defined integer type, when the program has one
	var named []*Type
	for _, t := range g.types {
		if t.K == KInt && t.Named {
			named = append(named, t)
		}
	}
	if len(named) > 0 {
		return pick(g, named)
	}
	return g.randIntType()
}

// randType makes a random value type for variables and struct fields: no pointers, funcs or
// channels, so every value prints the same in every run.
func (g *Gen) randType(depth int) *Type {
	if depth <= 0 {
		return g.randBasic()
	}
	switch g.roll(50, 10, 10, 7, 8, 5) {
	case 0:
		return g.randBasic()
	case 1:
		if g.has("arrays") {
			return arrayOf(g.randType(depth-1), g.between(1, 4))
		}
	case 2:
		if g.has("slices") {
			return sliceOf(g.randType(depth - 1))
		}
	case 3:
		if g.has("maps") {
			return mapOf(g.randKey(), g.randType(depth-1))
		}
	case 4, 5:
		if g.has("structs") {
			var ss []*Type
			for _, t := range g.types {
				if t.K == KStruct {
					ss = append(ss, t)
				}
			}
			if len(ss) > 0 && g.chance(70) {
				return pick(g, ss)
			}
			if depth > 1 {
				return g.newStruct(depth - 1)
			}
		}
	}
	return g.randBasic()
}

// randKey picks a map key type.
func (g *Gen) randKey() *Type {
	switch g.roll(50, 30, 8, 12) {
	case 0:
		return g.randIntType()
	case 1:
		return tString
	case 2:
		return tBool
	}
	if g.has("arrays") && g.chance(50) {
		return arrayOf(pick(g, []*Type{intTs["int8"], intTs["uint8"], tString}), 2)
	}
	return intTs["int"]
}

// newStruct declares a struct type with random fields.
func (g *Gen) newStruct(depth int) *Type {
	name := g.topName("T")
	t := &Type{K: KStruct, Name: name, Named: true, NoPtr: true}
	n := g.between(1, 5)
	names := []string{"A", "B", "C", "D", "E", "F", "x", "y", "n", "s"}
	g.shuffle(names)
	for i := 0; i < n; i++ {
		ft := g.randType(depth)
		t.Fields = append(t.Fields, Field{Name: names[i], T: ft})
		t.NoPtr = t.NoPtr && ft.NoPtr
	}
	// register before use by later types
	g.types = append(g.types, t)
	var b strings.Builder
	fmt.Fprintf(&b, "type %s struct {\n", name)
	for _, f := range t.Fields {
		fmt.Fprintf(&b, "\t%s %s\n", f.Name, f.T.Name)
	}
	b.WriteString("}\n")
	g.top.WriteString(b.String())
	return t
}

func (g *Gen) shuffle(s []string) {
	g.r.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
}

// newNamedInt declares "type pN_Kk int8" with a String method.
func (g *Gen) newNamedBasic(base *Type) *Type {
	name := g.topName("N")
	t := &Type{K: base.K, Name: name, Named: true, Bits: base.Bits, Signed: base.Signed, Under: base.Name, NoPtr: true}
	g.top.WriteString(fmt.Sprintf("type %s %s\n", name, base.Name))
	g.types = append(g.types, t)
	return t
}

// ---- paths: readable expressions rooted in variables ----

// Path is a variable or part of one that can be read (and assigned).
type Path struct {
	E    string
	T    *Type
	Root *Var
}

func (g *Gen) walk(root *Var, e string, t *Type, depth int, out *[]Path) {
	*out = append(*out, Path{e, t, root})
	if depth >= 3 {
		return
	}
	switch t.K {
	case KStruct:
		for _, f := range t.Fields {
			g.walk(root, e+"."+f.Name, f.T, depth+1, out)
		}
	case KArray:
		for i := 0; i < t.Len && i < 3; i++ {
			g.walk(root, fmt.Sprintf("%s[%d]", e, i), t.Elem, depth+1, out)
		}
	case KSlice:
		if depth == 0 && root.MinLen > 0 {
			for i := 0; i < root.MinLen && i < 3; i++ {
				g.walk(root, fmt.Sprintf("%s[%d]", e, i), t.Elem, depth+1, out)
			}
		}
	case KPtr:
		if depth == 0 && root.NonNil {
			if t.Elem.K == KStruct {
				for _, f := range t.Elem.Fields {
					g.walk(root, e+"."+f.Name, f.T, depth+1, out)
				}
			} else if t.Elem.isBasic() {
				g.walk(root, "(*"+e+")", t.Elem, depth+1, out)
			}
		}
	}
}

// pathsOf returns the readable (and assignable) places of exactly type t.
func (g *Gen) pathsOf(t *Type) []Path {
	var out []Path
	for _, v := range g.allVars() {
		g.walk(v, v.Name, v.T, 0, &out)
	}
	var res []Path
	for _, p := range out {
		if p.T.Name == t.Name {
			res = append(res, p)
		}
	}
	return res
}

// writable returns the places of exactly type t that statements may assign to: not the
// counters of loops.
func (g *Gen) writable(t *Type) []Path {
	var res []Path
	for _, p := range g.pathsOf(t) {
		if !p.Root.RO {
			res = append(res, p)
		}
	}
	return res
}

// readPath returns a place of type t, marking its variable used, or "".
func (g *Gen) readPath(t *Type) string {
	ps := g.pathsOf(t)
	if len(ps) == 0 {
		return ""
	}
	p := pick(g, ps)
	p.Root.Used = true
	return p.E
}

// tempVar declares a variable of type t holding a fresh value and returns its name.
func (g *Gen) tempVar(t *Type) string {
	v := g.declare("t", t, g.valInit(t))
	return v.Name
}

// valInit is an initialiser expression (no variables needed) of type t.
func (g *Gen) valInit(t *Type) string {
	switch t.K {
	case KInt, KFloat, KBool, KString:
		return g.lit(t)
	}
	return g.val(t, 1)
}

// nc returns a non-constant expression of basic type t.
func (g *Gen) nc(t *Type, d int) string {
	if p := g.readPath(t); p != "" && g.chance(75) {
		return p
	}
	s, c := g.x(t, d)
	if !c {
		return s
	}
	return g.tempVar(t)
}

// ---- expression soup ----

// expr returns a total pure expression of exactly type t.
func (g *Gen) expr(t *Type, d int) string {
	switch t.K {
	case KInt, KFloat, KBool, KString:
		s, _ := g.x(t, d)
		return s
	}
	return g.val(t, d)
}

// x also reports whether the expression is a constant.
func (g *Gen) x(t *Type, d int) (string, bool) {
	switch t.K {
	case KInt:
		return g.intX(t, d)
	case KFloat:
		return g.floatX(t, d)
	case KBool:
		return g.boolX(d)
	case KString:
		return g.stringX(d)
	}
	return g.val(t, d), false
}

func matchingEnd(s string) bool {
	depth := 0
	for i, c := range s {
		if c == '(' {
			depth++
		} else if c == ')' {
			depth--
			if depth == 0 && i != len(s)-1 {
				return false
			}
		}
	}
	return depth == 0
}

// par wraps a compound operand in parentheses.
func par(s string) string {
	if strings.HasPrefix(s, "(") && matchingEnd(s) {
		return s
	}
	for _, c := range s {
		if !(c == '_' || c == '.' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return "(" + s + ")"
		}
	}
	return s
}

// idx returns an int expression in [0, n) for n > 0 given as a constant.
func (g *Gen) idxConst(n int) string {
	if g.chance(40) {
		return strconv.Itoa(g.rn(n))
	}
	u := g.nc(intTs[pick(g, []string{"uint8", "uint16", "uint32", "uint", "uint64"})], 1)
	return fmt.Sprintf("int(%s %% %d)", par(u), n) // reduce in the unsigned type: int(uint64) can be negative
}

// idxDyn returns an int expression in [0, len(s)) for a slice or string known to be non-empty.
func (g *Gen) idxDyn(s string) string {
	u := g.nc(intTs[pick(g, []string{"uint8", "uint16", "uint32", "uint", "uint64"})], 1)
	return fmt.Sprintf("int(uint(%s)%%uint(len(%s)))", u, s)
}

func (g *Gen) intLeaf(t *Type) (string, bool) {
	if g.chance(55) {
		if p := g.readPath(t); p != "" {
			return p, false
		}
	}
	if g.chance(50) {
		if g.chance(50) {
			return g.lit(t), true
		}
		return t.Name + "(" + g.intRaw(t) + ")", true
	}
	return g.lit(t), true
}

var arith = []string{"+", "-", "*", "&", "|", "^", "&^", "+", "-", "*"}

func (g *Gen) intX(t *Type, d int) (string, bool) {
	if d <= 0 {
		return g.intLeaf(t)
	}
	switch g.roll(16, 30, 9, 11, 10, 5, 4, 5, 5, 4, 4, 3) {
	case 0:
		return g.intLeaf(t)
	case 1: // binary
		a := g.nc(t, d-1)
		var b string
		if g.chance(30) {
			b = paren(g.intRaw(t)) // untyped constant converted to the type of a
		} else {
			b, _ = g.x(t, d-1)
		}
		return fmt.Sprintf("%s %s %s", par(a), pick(g, arith), par(b)), false
	case 2: // division and remainder
		a := g.nc(t, d-1)
		op := pick(g, []string{"/", "%"})
		switch g.roll(5, 3, 2) {
		case 0: // never zero
			b, _ := g.x(t, d-1)
			return fmt.Sprintf("%s %s (%s | 1)", par(a), op, b), false
		case 1: // constant divisor
			raw := g.intRaw(t)
			for raw == "0" {
				raw = g.intRaw(t)
			}
			return fmt.Sprintf("%s %s %s", par(a), op, paren(raw)), false
		default:
			if t.Signed {
				return fmt.Sprintf("%s %s %s", par(a), op, "(-1)"), false
			}
			return fmt.Sprintf("%s %s %s", par(a), op, paren(strconv.Itoa(g.between(1, 9)))), false
		}
	case 3: // shifts
		a := g.nc(t, d-1)
		op := pick(g, []string{"<<", ">>"})
		return fmt.Sprintf("%s %s %s", par(a), op, g.shiftCount(t.Bits)), false
	case 4: // conversion from another integer type
		o := g.randIntType()
		if g.chance(30) && g.has("floats") {
			// float to integer conversion of a value that surely fits
			f := g.boundedFloat(t)
			return fmt.Sprintf("%s(%s)", t.Name, f), false
		}
		a := g.nc(o, d-1)
		return fmt.Sprintf("%s(%s)", t.Name, a), false
	case 5: // unary
		a := g.nc(t, d-1)
		switch g.roll(4, 4, 1) {
		case 0:
			return "-" + par(a), false
		case 1:
			return "^" + par(a), false
		}
		return "+" + par(a), false
	case 6: // untyped constant shifted by a variable count: the constant takes the type
		if g.has("consts") {
			return fmt.Sprintf("%s(1 << %s)", t.Name, par(g.nc(intTs[pick(g, []string{"uint8", "uint16", "uint32", "uint", "uint64"})], 1))), false
		}
		return g.intLeaf(t)
	case 7: // min and max
		if g.has("minmax") {
			a := g.nc(t, d-1)
			b, _ := g.x(t, d-1)
			return fmt.Sprintf("%s(%s, %s)", pick(g, []string{"min", "max"}), a, b), false
		}
		return g.intLeaf(t)
	case 8: // length of something
		if s := g.lenOperand(); s != "" {
			return fmt.Sprintf("%s(%s)", t.Name, s), false
		}
		return g.intLeaf(t)
	case 9: // call of a pure function
		if c := g.call(t, d-1); c != "" {
			return c, false
		}
		return g.intLeaf(t)
	case 10: // conditional
		c := g.boolExpr(d - 1)
		a, _ := g.x(t, d-1)
		b, _ := g.x(t, d-1)
		return fmt.Sprintf("func() %s { if %s { return %s }; return %s }()", t.Name, c, a, b), false
	case 11: // indexing a map, array or slice
		if s := g.indexExpr(t, d-1); s != "" {
			return s, false
		}
		return g.intLeaf(t)
	}
	return g.intLeaf(t)
}

// shiftCount returns a shift count expression that cannot panic: a small constant, or an
// unsigned expression of any width (the count may exceed the width of the shifted value).
func (g *Gen) shiftCount(bits int) string {
	switch g.roll(30, 15, 25, 10, 5) {
	case 0:
		return strconv.Itoa(g.rn(bits))
	case 1:
		return strconv.Itoa(bits + g.rn(70))
	case 2:
		u := g.nc(intTs[pick(g, []string{"uint8", "uint16", "uint32", "uint", "uint64", "uint8"})], 1)
		return par(u)
	case 3: // a signed count made non-negative
		s := g.nc(intTs[pick(g, []string{"int8", "int", "int16", "int32", "int64"})], 1)
		return fmt.Sprintf("uint(%s&0x7f)", par(s))
	}
	return strconv.Itoa(bits - 1)
}

// lenOperand returns len(x) for a string, slice, map or array that the program has, or "".
func (g *Gen) lenOperand() string {
	var cands []string
	for _, p := range g.allPaths() {
		switch p.T.K {
		case KString, KSlice, KMap: // not arrays: len of an array is a constant
			cands = append(cands, p.E)
		}
	}
	if len(cands) == 0 {
		return ""
	}
	e := pick(g, cands)
	for _, p := range g.allPaths() {
		if p.E == e {
			p.Root.Used = true
			break
		}
	}
	return "len(" + e + ")"
}

func (g *Gen) allPaths() []Path {
	var out []Path
	for _, v := range g.allVars() {
		g.walk(v, v.Name, v.T, 0, &out)
	}
	return out
}

// indexExpr reads an element of type t from an array, slice or map.
func (g *Gen) indexExpr(t *Type, d int) string {
	var cands []Path
	for _, p := range g.allPaths() {
		switch p.T.K {
		case KArray, KSlice, KMap:
			if p.T.Elem.Name == t.Name {
				cands = append(cands, p)
			}
		case KString:
			if t.Name == "uint8" && p.Root.MinLen > 0 && p.E == p.Root.Name {
				cands = append(cands, p)
			}
		}
	}
	if len(cands) == 0 {
		return ""
	}
	p := pick(g, cands)
	p.Root.Used = true
	switch p.T.K {
	case KArray:
		return fmt.Sprintf("%s[%s]", p.E, g.idxConst(p.T.Len))
	case KSlice:
		if p.E == p.Root.Name && p.Root.MinLen > 0 {
			return fmt.Sprintf("%s[%s]", p.E, g.idxDyn(p.E))
		}
		return ""
	case KMap:
		return fmt.Sprintf("%s[%s]", p.E, g.keyExpr(p.T.Key, d))
	case KString:
		return fmt.Sprintf("%s[%s]", p.E, g.idxDyn(p.E))
	}
	return ""
}

// keyExpr returns a key of a map: an expression that may or may not be present.
func (g *Gen) keyExpr(k *Type, d int) string {
	switch k.K {
	case KInt, KString, KBool:
		if g.chance(60) {
			return g.keyLit(k, g.rn(4))
		}
		return g.expr(k, d)
	}
	return g.val(k, d)
}

// keyLit returns the i-th distinct constant key of a map key type.
func (g *Gen) keyLit(k *Type, i int) string {
	switch k.K {
	case KInt:
		if k.Signed {
			return k.Name + "(" + strconv.Itoa(i%6-2) + ")"
		}
		return k.Name + "(" + strconv.Itoa(i%6) + ")"
	case KString:
		ks := []string{`"a"`, `"b"`, `""`, `"é"`, `"zz"`, `"k4"`, `"k5"`, `"k6"`}
		return ks[i%8]
	case KBool:
		return []string{"false", "true"}[i%2]
	case KArray:
		parts := make([]string, k.Len)
		for j := range parts {
			if j == 0 {
				parts[j] = g.keyLit(k.Elem, i)
			} else {
				parts[j] = g.keyLit(k.Elem, g.rn(4))
			}
		}
		return k.Name + "{" + strings.Join(parts, ", ") + "}"
	case KStruct:
		parts := make([]string, len(k.Fields))
		for j, f := range k.Fields {
			if j == 0 {
				parts[j] = g.keyLit(f.T, i)
			} else {
				parts[j] = g.valInit(f.T)
			}
		}
		return k.Name + "{" + strings.Join(parts, ", ") + "}"
	}
	return g.valInit(k)
}

// distinctKeys reports whether a map key type's keyLit gives distinct keys for i < n.
func distinctKeyCount(k *Type) int {
	switch k.K {
	case KBool:
		return 2
	case KInt:
		return 6
	case KArray:
		return distinctKeyCount(k.Elem)
	case KStruct:
		if len(k.Fields) > 0 {
			return distinctKeyCount(k.Fields[0].T)
		}
	}
	return 6
}

// call returns a call of a pure function that returns t, or "".
func (g *Gen) call(t *Type, d int) string {
	var fs []*Func
	for _, f := range g.funcs {
		if f.Pure && f.Result != nil && f.Result.Name == t.Name {
			fs = append(fs, f)
		}
	}
	if len(fs) == 0 {
		return g.methodCall(t, d)
	}
	f := pick(g, fs)
	args := make([]string, len(f.Params))
	for i, p := range f.Params {
		args[i] = g.expr(p, d)
	}
	return fmt.Sprintf("%s(%s)", f.Name, strings.Join(args, ", "))
}

// methodCall calls a pure method of a variable that returns t.
func (g *Gen) methodCall(t *Type, d int) string {
	type cand struct {
		v *Var
		m Method
	}
	var cs []cand
	for _, v := range g.allVars() {
		ty := v.T
		if ty.K == KPtr {
			if !v.NonNil {
				continue
			}
			ty = ty.Elem
		}
		for _, m := range ty.Meths {
			if m.Pure && m.Result != nil && m.Result.Name == t.Name {
				if m.Ptr && v.T.K != KPtr && !true {
					continue
				}
				cs = append(cs, cand{v, m})
			}
		}
	}
	if len(cs) == 0 {
		return ""
	}
	c := pick(g, cs)
	c.v.Used = true
	args := make([]string, len(c.m.Params))
	for i, p := range c.m.Params {
		args[i] = g.expr(p, d)
	}
	return fmt.Sprintf("%s.%s(%s)", c.v.Name, c.m.Name, strings.Join(args, ", "))
}

// boundedFloat returns a float64 expression whose absolute value is below 1e9, so converting
// it to any integer type of 32 bits or more is exact in range (smaller types wrap through
// an int64 first, see the caller).
func (g *Gen) boundedFloat(t *Type) string {
	src := intTs["uint8"]
	if t.Signed {
		src = intTs["int8"]
	}
	den := pick(g, []string{"1", "2", "4", "8"})
	return fmt.Sprintf("float64(%s) / %s", par(g.nc(src, 1)), den)
}

// ---- floats ----

func (g *Gen) floatX(t *Type, d int) (string, bool) {
	leaf := func() (string, bool) {
		if g.chance(55) {
			if p := g.readPath(t); p != "" {
				return p, false
			}
		}
		return g.lit(t), true
	}
	if d <= 0 {
		return leaf()
	}
	switch g.roll(10, 30, 8, 12, 6, 5) {
	case 0:
		return leaf()
	case 1:
		a := g.nc(t, d-1)
		b, _ := g.x(t, d-1)
		return fmt.Sprintf("%s %s %s", par(a), pick(g, []string{"+", "-", "*"}), par(b)), false
	case 2: // division by something at least 1
		a := g.nc(t, d-1)
		b := g.nc(t, d-1)
		return fmt.Sprintf("%s / (%s*%s + 1)", par(a), par(b), par(b)), false
	case 3: // from an integer
		o := g.randIntType()
		if o.Bits == 64 {
			o = intTs[pick(g, []string{"int32", "uint32", "int16"})]
		}
		return fmt.Sprintf("%s(%s)", t.Name, g.nc(o, d-1)), false
	case 4:
		a := g.nc(t, d-1)
		return "-" + par(a), false
	case 5: // conversion between float32 and float64
		o := tF64
		if t.Bits == 64 {
			o = tF32
		}
		return fmt.Sprintf("%s(%s)", t.Name, g.nc(o, d-1)), false
	}
	return leaf()
}

// ---- bool ----

func (g *Gen) boolX(d int) (string, bool) {
	leaf := func() (string, bool) {
		if g.chance(50) {
			if p := g.readPath(tBool); p != "" {
				return p, false
			}
		}
		return pick(g, []string{"true", "false"}), true
	}
	if d <= 0 {
		return leaf()
	}
	switch g.roll(8, 40, 15, 5, 8, 6, 4) {
	case 0:
		return leaf()
	case 1: // comparison of two integers
		t := g.randIntType()
		a := g.nc(t, d-1)
		b, _ := g.x(t, d-1)
		return fmt.Sprintf("%s %s %s", par(a), pick(g, []string{"<", "<=", ">", ">=", "==", "!="}), par(b)), false
	case 2: // logic
		a, _ := g.boolX(d - 1)
		b, _ := g.boolX(d - 1)
		if a == "true" || a == "false" {
			a = g.tempVar(tBool)
		}
		return fmt.Sprintf("%s %s %s", par(a), pick(g, []string{"&&", "||"}), par(b)), false
	case 3:
		a, c := g.boolX(d - 1)
		if c {
			a = g.tempVar(tBool)
		}
		return "!" + par(a), false
	case 4: // comparison of strings
		a := g.nc(tString, d-1)
		b, _ := g.x(tString, d-1)
		return fmt.Sprintf("%s %s %s", par(a), pick(g, []string{"<", "<=", ">", ">=", "==", "!="}), par(b)), false
	case 5: // floats
		t := tF64
		if g.chance(30) {
			t = tF32
		}
		a := g.nc(t, d-1)
		b, _ := g.x(t, d-1)
		return fmt.Sprintf("%s %s %s", par(a), pick(g, []string{"<", "<=", ">", ">=", "==", "!="}), par(b)), false
	case 6: // comparable composite values
		if t := g.randComparable(); t != nil {
			a := g.val(t, d-1)
			b := g.val(t, d-1)
			if p := g.readPath(t); p != "" {
				a = p
			}
			return fmt.Sprintf("%s %s %s", par(a), pick(g, []string{"==", "!="}), par(b)), false
		}
	}
	return leaf()
}

func (g *Gen) boolExpr(d int) string {
	s, _ := g.boolX(d)
	return s
}

// randComparable picks an array or struct type whose values can be compared with ==.
func (g *Gen) randComparable() *Type {
	var cs []*Type
	for _, v := range g.allVars() {
		if (v.T.K == KStruct || v.T.K == KArray) && v.T.comparable() {
			cs = append(cs, v.T)
		}
	}
	if len(cs) == 0 {
		return nil
	}
	return pick(g, cs)
}

// ---- strings ----

func (g *Gen) stringX(d int) (string, bool) {
	leaf := func() (string, bool) {
		if g.chance(50) {
			if p := g.readPath(tString); p != "" {
				return p, false
			}
		}
		return g.strRaw(), true
	}
	if d <= 0 {
		return leaf()
	}
	switch g.roll(10, 25, 12, 8, 10, 6, 6) {
	case 0:
		return leaf()
	case 1:
		a := g.nc(tString, d-1)
		b, _ := g.x(tString, d-1)
		return fmt.Sprintf("%s + %s", par(a), par(b)), false
	case 2: // string of a rune
		return fmt.Sprintf("string(rune(%s))", g.nc(g.randIntType(), d-1)), false
	case 3: // a slice of a string whose bounds are in range
		for _, v := range g.allVars() {
			if v.T.K == KString && v.MinLen > 0 {
				v.Used = true
				lo := g.idxDyn(v.Name)
				switch g.rn(3) {
				case 0:
					return fmt.Sprintf("%s[%s:]", v.Name, lo), false
				case 1:
					return fmt.Sprintf("%s[:%s]", v.Name, lo), false
				}
				return fmt.Sprintf("%s[%s:] + %s[:%s]", v.Name, lo, v.Name, lo), false
			}
		}
		return leaf()
	case 4: // Sprintf
		return g.sprintf(d - 1), false
	case 5: // conversions through []byte and []rune
		a := g.nc(tString, d-1)
		if g.chance(50) {
			return fmt.Sprintf("string([]byte(%s))", a), false
		}
		return fmt.Sprintf("string([]rune(%s))", a), false
	case 6:
		c := g.boolExpr(d - 1)
		a, _ := g.x(tString, d-1)
		b, _ := g.x(tString, d-1)
		return fmt.Sprintf("func() string { if %s { return %s }; return %s }()", c, a, b), false
	}
	return leaf()
}

// sprintf returns fmt.Sprintf of one or two values with matching verbs.
func (g *Gen) sprintf(d int) string {
	var parts []string
	var args []string
	n := g.between(1, 2)
	for i := 0; i < n; i++ {
		t := g.randBasic()
		parts = append(parts, g.verbFor(t))
		args = append(args, g.expr(t, d))
	}
	return fmt.Sprintf("fmt.Sprintf(%q, %s)", strings.Join(parts, "|"), strings.Join(args, ", "))
}

// verbFor returns a formatting verb (with flags) that is valid for the basic type t.
func (g *Gen) verbFor(t *Type) string {
	flags := func() string {
		return pick(g, []string{"", "", "", "-", "+", " ", "0", "#"})
	}
	width := func() string {
		if g.chance(35) {
			return strconv.Itoa(g.between(1, 12))
		}
		return ""
	}
	switch t.K {
	case KInt:
		v := pick(g, []string{"d", "d", "v", "x", "X", "o", "b", "c", "q", "U", "O"})
		f := flags()
		if v == "c" || v == "q" || v == "U" {
			if f == "0" {
				f = ""
			}
		}
		return "%" + f + width() + v
	case KFloat:
		prec := ""
		if g.chance(50) {
			prec = "." + strconv.Itoa(g.rn(5))
		}
		v := pick(g, []string{"v", "g", "f", "e", "F", "G"})
		return "%" + pick(g, []string{"", "", "-", "+", " ", "0"}) + width() + prec + v
	case KBool:
		return "%" + pick(g, []string{"", "", "-"}) + width() + pick(g, []string{"t", "v"})
	case KString:
		prec := ""
		if g.chance(25) {
			prec = "." + strconv.Itoa(g.rn(4))
		}
		return "%" + pick(g, []string{"", "", "-", "+", "#", " "}) + width() + prec + pick(g, []string{"s", "v", "q", "x", "X", "s"})
	}
	return "%v"
}

// ---- composite values ----

// val returns an expression of any type t (basic types go through expr).
func (g *Gen) val(t *Type, d int) string {
	switch t.K {
	case KInt, KFloat, KBool, KString:
		return g.expr(t, d)
	}
	if d > 0 && g.chance(35) {
		if p := g.readPath(t); p != "" {
			return p
		}
	}
	if d > 0 && g.chance(8) {
		if c := g.call(t, d-1); c != "" {
			return c
		}
	}
	return g.ctor(t, d)
}

// valMin is val, and the length that a slice value certainly has (0: unknown).
func (g *Gen) valMin(t *Type, d int) (string, int) {
	g.lastCtor, g.lastMin = "", 0
	s := g.val(t, d)
	if t.K == KSlice && s == g.lastCtor {
		return s, g.lastMin
	}
	if t.K == KString {
		if u, err := strconv.Unquote(s); err == nil {
			return s, len(u)
		}
	}
	return s, 0
}

// ctor returns a composite literal (an addressable-looking constructor, never a variable).
func (g *Gen) ctor(t *Type, d int) string {
	switch t.K {
	case KInt, KFloat, KBool, KString:
		return g.expr(t, d)
	case KArray:
		if g.chance(15) {
			// sparse keyed literal
			i := g.rn(t.Len)
			return fmt.Sprintf("%s{%d: %s}", t.Name, i, g.val(t.Elem, d-1))
		}
		parts := make([]string, t.Len)
		for i := range parts {
			parts[i] = g.val(t.Elem, d-1)
		}
		return t.Name + "{" + strings.Join(parts, ", ") + "}"
	case KSlice:
		switch g.roll(70, 8, 8, 6) {
		case 1:
			n := g.between(1, 4)
			out := fmt.Sprintf("make(%s, %d)", t.Name, n)
			g.lastCtor, g.lastMin = out, n
			return out
		case 2:
			return t.Name + "(nil)"
		case 3:
			return t.Name + "{}"
		}
		n := g.between(1, 4)
		parts := make([]string, n)
		for i := range parts {
			parts[i] = g.val(t.Elem, d-1)
		}
		out := t.Name + "{" + strings.Join(parts, ", ") + "}"
		g.lastCtor, g.lastMin = out, n
		return out
	case KMap:
		n := g.between(0, 3)
		if m := distinctKeyCount(t.Key); n > m {
			n = m
		}
		parts := make([]string, n)
		for i := range parts {
			parts[i] = g.keyLit(t.Key, i) + ": " + g.val(t.Elem, d-1)
		}
		return t.Name + "{" + strings.Join(parts, ", ") + "}"
	case KStruct:
		if g.chance(50) {
			parts := make([]string, len(t.Fields))
			for i, f := range t.Fields {
				parts[i] = g.val(f.T, d-1)
			}
			return t.Name + "{" + strings.Join(parts, ", ") + "}"
		}
		var parts []string
		for _, f := range t.Fields {
			if g.chance(80) {
				parts = append(parts, f.Name+": "+g.val(f.T, d-1))
			}
		}
		return t.Name + "{" + strings.Join(parts, ", ") + "}"
	case KPtr:
		if t.Elem.K == KStruct || t.Elem.K == KArray {
			return "&" + g.ctor(t.Elem, d)
		}
		return fmt.Sprintf("func() %s { v := %s; return &v }()", t.Name, g.val(t.Elem, d))
	case KIface:
		if t.Name == "any" {
			bt := g.randType(1)
			return "any(" + g.val(bt, d-1) + ")"
		}
	}
	panic("val: unsupported type " + t.Name)
}

var _ = bits.Len
