// Package gen makes random well-typed gocpp programs (Phase 15 step 15.8): programs that go/types
// accepts, for the fuzzers of the pipeline. The mutations of a byte-level fuzzer almost always
// break the syntax or the types of a program at once, so they exercise the parser and the type
// checker's error paths, but rarely the lowering and the emitter. A program of this generator is
// valid by construction, and the choices that make it are read from a byte string (FromBytes), so
// that the coverage-guided engine mutates the choices and not the text: a mutated input is a
// different valid program.
//
// The programs use structs (with embedded structs and methods), named basic types, interfaces
// (satisfied by pointers to structs), slices, arrays, maps, pointers, channels, function values
// and closures, defer, panic and recover, generic functions, goroutines, select, switch, type
// switches, labeled loops, multiple results and variadic functions. They are never run: the
// fuzzers only check, lower, emit and compile them (the differential tester of step 15.9 is the
// one that runs programs, with its own generator).
//
// Rules that keep a program valid, and so shape the generator: every variable is used right
// after its declaration (_ = v); every name is unique in the program; arithmetic that could be a
// constant overflow, a constant division by zero, a constant conversion error or a constant
// index out of range always has a variable in it; a global initializer calls nothing (no
// initialization cycle); the cases of a switch are distinct constants.
package gen

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

// A Rand is the source of the generator's choices: Intn(n) returns a number in [0, n). The
// choices are ordered so that 0 is the simplest, which is what a byte string that has run out gives.
type Rand interface {
	Intn(n int) int
}

type pcg struct{ r *rand.Rand }

func (p pcg) Intn(n int) int {
	if n <= 1 {
		return 0
	}
	return p.r.IntN(n)
}

type byteRand struct {
	b []byte
	i int
	x uint64 // the state of the generator that goes on when the bytes are used up
}

func (b *byteRand) Intn(n int) int {
	if n <= 1 {
		return 0
	}
	if b.i >= len(b.b) {
		// The bytes are used up: go on with a fixed sequence (xorshift), so that every loop that
		// asks "again?" ends, which an always-zero source would not allow.
		if b.x == 0 {
			b.x = 0x9e3779b97f4a7c15 ^ uint64(len(b.b))
		}
		b.x ^= b.x << 13
		b.x ^= b.x >> 7
		b.x ^= b.x << 17
		return int(b.x>>1) % n
	}
	v := int(b.b[b.i])
	b.i++
	if n > 256 && b.i < len(b.b) {
		v = v<<8 | int(b.b[b.i])
		b.i++
	}
	return v % n
}

// Options say which features the programs use and how big they are.
type Options struct {
	Types, Funcs int // how many type declarations and functions (0: 3 to 6 of each)

	Interfaces   bool
	Embedding    bool
	Generics     bool // generic functions
	GenericTypes bool // generic types: Stack[T], Pair[K, V]
	Goroutines   bool // go, channels, select
	RangeFunc    bool // range over a function
	Gaps         bool // with Extras: also the constructs that the emitter does not lower yet (conversions between string and a slice of a named byte type), see tools/fuzz/gaps.go
	Extras       bool // forwarding f(g()), iota constants, shifts, runes, goto, method expressions, anonymous structs, named slice and map types
}

// Default returns the options of the fuzz targets: every feature that the compiler has.
func Default() Options {
	return Options{Interfaces: true, Embedding: true, Generics: true, Goroutines: true, Extras: true}
}

// FromSeed returns the program for a seed.
func FromSeed(seed uint64, o Options) []byte {
	return Program(pcg{rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}, o)
}

// FromBytes returns the program that the choices in data make: any byte string gives a valid
// program, and a longer one a bigger program.
func FromBytes(data []byte, o Options) []byte {
	return Program(&byteRand{b: data}, o)
}

// Program returns a program for the choices of r.
func Program(r Rand, o Options) []byte {
	g := &gen{r: r, o: o, used: map[string]bool{}}
	return g.program()
}

type variable struct {
	name   string
	t      *typ
	global bool
}

type fn struct {
	name            string
	params, results []*typ
	variadic        bool // the last parameter is ...elem (its type in params is the slice)
	fwd             *fn  // the function returns the call of fwd: return fwd(args)
}

type gen struct {
	r Rand
	o Options

	n    int // for unique names
	used map[string]bool

	structs []*typ
	named   []*typ
	ifaces  []*typ
	funcs   []*fn
	globals []*variable
	scalarV map[string]*variable // a global variable for every scalar type

	scopes [][]*variable
	static bool // a global initializer: no calls, no function literals

	out    strings.Builder
	ind    int
	budget int // statements left

	res     []*typ // results of the function being written
	loops   int    // enclosing for statements
	breaks  int    // enclosing for, switch and select statements
	labels  []string
	needFmt bool
	gfuncs  map[string]bool

	// The extras (extras.go).
	consts    map[string][]string // constants by the String of their type
	cgroups   []cgroup
	constDecl strings.Builder
	fwds      []fwdPair
	fwdMeths  []fwdMeth
	myb       *mybTypes
}

func (g *gen) intn(n int) int    { return g.r.Intn(n) }
func (g *gen) chance(k int) bool { return g.r.Intn(k) == 0 }

func (g *gen) name(prefix string) string {
	g.n++
	return fmt.Sprintf("%s%d", prefix, g.n)
}

func (g *gen) line(format string, args ...any) {
	g.out.WriteString(strings.Repeat("\t", g.ind))
	fmt.Fprintf(&g.out, format, args...)
	g.out.WriteByte('\n')
}

func (g *gen) push() { g.scopes = append(g.scopes, nil) }
func (g *gen) pop()  { g.scopes = g.scopes[:len(g.scopes)-1] }

func (g *gen) declare(name string, t *typ) *variable {
	v := &variable{name: name, t: t}
	g.scopes[len(g.scopes)-1] = append(g.scopes[len(g.scopes)-1], v)
	return v
}

// vars returns the variables in sight whose type is t.
func (g *gen) varsOf(t *typ) []*variable {
	var out []*variable
	s := t.String()
	for _, sc := range g.scopes {
		for _, v := range sc {
			if v.t.String() == s {
				out = append(out, v)
			}
		}
	}
	for _, v := range g.globals {
		if v.t.String() == s {
			out = append(out, v)
		}
	}
	return out
}

func (g *gen) allVars() []*variable {
	var out []*variable
	for _, sc := range g.scopes {
		out = append(out, sc...)
	}
	return append(out, g.globals...)
}

// program writes the whole program.
func (g *gen) program() []byte {
	g.scalarV = map[string]*variable{}
	g.gfuncs = map[string]bool{}
	g.consts = map[string][]string{}
	g.universe()

	var types strings.Builder
	g.writeTypes(&types)

	// The package-level variables: one of every scalar type, and some others, with initializers
	// that call nothing.
	var vars strings.Builder
	g.static = true
	g.out.Reset()
	g.push()
	for _, t := range append(append([]*typ{}, basics...), g.named...) {
		v := &variable{name: g.name("g"), t: t, global: true}
		g.scalarV[t.String()] = v
		e := g.leafLit(t)
		g.line("var %s %s = %s", v.name, t, e.s)
		g.globals = append(g.globals, v)
	}
	for i := g.intn(5); i > 0; i-- {
		t := g.valueType(2, nil)
		v := &variable{name: g.name("g"), t: t, global: true}
		e := g.expr(t, 3)
		g.line("var %s %s = %s", v.name, t, e.s)
		g.globals = append(g.globals, v)
	}
	g.pop()
	g.static = false
	vars.WriteString(g.out.String())
	g.out.Reset()

	g.budget = 40 + 20*g.intn(8)
	if g.o.Funcs > 0 {
		g.budget += 10 * g.o.Funcs
	}
	g.writeMethods()
	g.writeFuncs()
	g.writeMain()
	g.writeGenerics()
	body := g.out.String()

	var b strings.Builder
	b.WriteString("package main\n\n")
	if g.needFmt {
		b.WriteString("import \"fmt\"\n\n")
	}
	b.WriteString(types.String())
	b.WriteString("\n")
	b.WriteString(g.constDecl.String())
	b.WriteString(vars.String())
	b.WriteString("\n")
	b.WriteString(body)
	return []byte(b.String())
}

// universe makes the types, the signatures of the methods and the functions.
func (g *gen) universe() {
	n := g.o.Types
	if n == 0 {
		n = 3 + g.intn(4)
	}
	for i := 0; i < n; i++ {
		switch c := g.intn(10); {
		case c < 6 || len(g.structs) == 0:
			g.newStruct()
		case c < 8:
			g.newNamed()
		default:
			if g.o.Interfaces {
				g.newIface()
			} else {
				g.newStruct()
			}
		}
	}
	for _, i := range g.ifaces {
		i.impls = nil
		for _, s := range g.structs {
			if implements(s, i) {
				i.impls = append(i.impls, s)
			}
		}
	}
	if g.o.Extras {
		g.newConsts()
	}
	nf := g.o.Funcs
	if nf == 0 {
		nf = 3 + g.intn(4)
	}
	for i := 0; i < nf; i++ {
		f := &fn{name: g.name("f")}
		for j := g.intn(4); j > 0; j-- {
			f.params = append(f.params, g.valueType(2, nil))
		}
		if len(f.params) > 0 && g.chance(5) {
			f.variadic = true
			e := g.valueType(1, nil)
			f.params[len(f.params)-1] = &typ{kind: kSlice, elem: e}
		}
		for j := g.intn(3); j > 0; j-- {
			f.results = append(f.results, g.valueType(2, nil))
		}
		g.funcs = append(g.funcs, f)
	}
	if g.o.Extras {
		g.newForwarders()
	}
}

func (g *gen) newNamed() {
	b := basics[g.intn(len(basics))]
	if b.kind == kBool && g.chance(2) {
		b = tInt
	}
	t := &typ{kind: kNamed, name: g.name("N"), base: b}
	if g.chance(2) {
		t.methods = append(t.methods, g.newMethod(false))
	}
	g.named = append(g.named, t)
}

func (g *gen) newMethod(ptrOK bool) *method {
	m := &method{name: g.name("m"), ptr: ptrOK && g.chance(2)}
	for j := g.intn(3); j > 0; j-- {
		m.params = append(m.params, g.valueType(1, nil))
	}
	for j := g.intn(3); j > 0; j-- {
		m.results = append(m.results, g.valueType(1, nil))
	}
	return m
}

func (g *gen) newStruct() *typ {
	t := &typ{kind: kStruct, name: g.name("T")}
	// Embedded structs: methods of the embedded types must not clash.
	if g.o.Embedding && len(g.structs) > 0 && g.chance(3) {
		for k := 1 + g.intn(2); k > 0; k-- {
			e := g.structs[g.intn(len(g.structs))]
			clash := false
			names := t.methodNames()
			for n := range e.methodNames() {
				if names[n] {
					clash = true
				}
			}
			for _, f := range t.fields {
				if f.t == e {
					clash = true
				}
			}
			if !clash {
				t.fields = append(t.fields, &field{name: e.name, t: e, embedded: true})
				t.embeds = append(t.embeds, e)
			}
		}
	}
	for k := 1 + g.intn(4); k > 0; k-- {
		t.fields = append(t.fields, &field{name: g.name("x"), t: g.valueType(2, t)})
	}
	for k := g.intn(4); k > 0; k-- {
		t.methods = append(t.methods, g.newMethod(true))
	}
	g.structs = append(g.structs, t)
	return t
}

// newIface makes an interface of some methods of a struct, and a second struct with the same methods.
func (g *gen) newIface() {
	var withM []*typ
	for _, s := range g.structs {
		if len(s.methods) > 0 {
			withM = append(withM, s)
		}
	}
	if len(withM) == 0 {
		g.newStruct()
		return
	}
	s := withM[g.intn(len(withM))]
	it := &typ{kind: kIface, name: g.name("I")}
	for _, m := range s.methods {
		if len(it.methods) == 0 || g.chance(2) {
			it.methods = append(it.methods, &method{name: m.name, params: m.params, results: m.results})
		}
	}
	g.ifaces = append(g.ifaces, it)
	if g.chance(2) {
		c := &typ{kind: kStruct, name: g.name("T")}
		c.fields = append(c.fields, &field{name: g.name("x"), t: g.valueType(1, nil)})
		for _, m := range it.methods {
			c.methods = append(c.methods, &method{name: m.name, ptr: g.chance(2), params: m.params, results: m.results})
		}
		g.structs = append(g.structs, c)
	}
}

// valueType returns a random type for a variable, a field or a parameter. self is the struct that
// is being declared, which may be used only behind a pointer, slice, map, channel or func.
func (g *gen) valueType(d int, self *typ) *typ {
	if d <= 0 {
		return basics[g.intn(len(basics))]
	}
	for {
		switch c := g.intn(16); c {
		case 0, 1:
			return tInt
		case 2:
			return basics[g.intn(len(basics))]
		case 3:
			if len(g.named) > 0 {
				return g.named[g.intn(len(g.named))]
			}
		case 4, 5:
			return &typ{kind: kSlice, elem: g.valueType(d-1, self)}
		case 6:
			return &typ{kind: kMap, key: g.keyType(), elem: g.valueType(d-1, self)}
		case 7:
			if t := g.pickStruct(self); t != nil {
				return &typ{kind: kPtr, elem: t}
			}
			if self != nil {
				return &typ{kind: kPtr, elem: self}
			}
		case 8:
			if t := g.pickStruct(self); t != nil {
				return t
			}
		case 9:
			e := g.valueType(d-1, nil)
			return &typ{kind: kArray, n: 1 + g.intn(3), elem: e}
		case 10:
			if len(g.ifaces) > 0 {
				return g.ifaces[g.intn(len(g.ifaces))]
			}
		case 11:
			if g.o.Goroutines {
				return &typ{kind: kChan, elem: g.valueType(d-1, self)}
			}
		case 12:
			f := &typ{kind: kFunc}
			for j := g.intn(3); j > 0; j-- {
				f.params = append(f.params, g.valueType(d-1, self))
			}
			if g.chance(2) {
				f.results = append(f.results, g.valueType(d-1, self))
			}
			return f
		case 13:
			return tAny
		case 14:
			return tString
		default:
			return tInt
		}
	}
}

func (g *gen) pickStruct(self *typ) *typ {
	var c []*typ
	for _, s := range g.structs {
		if s != self {
			c = append(c, s)
		}
	}
	if len(c) == 0 {
		return nil
	}
	return c[g.intn(len(c))]
}

func (g *gen) keyType() *typ {
	switch g.intn(6) {
	case 0, 1:
		return tInt
	case 2, 3:
		return tString
	case 4:
		return tInt64
	}
	if len(g.named) > 0 {
		n := g.named[g.intn(len(g.named))]
		if n.base.kind != kFloat {
			return n
		}
	}
	return tUint8
}

// writeTypes writes the type declarations.
func (g *gen) writeTypes(b *strings.Builder) {
	for _, t := range g.named {
		fmt.Fprintf(b, "type %s %s\n\n", t.name, t.base)
	}
	for _, t := range g.structs {
		fmt.Fprintf(b, "type %s struct {\n", t.name)
		for _, f := range t.fields {
			if f.embedded {
				fmt.Fprintf(b, "\t%s\n", f.t)
			} else if g.o.Extras && g.chance(4) {
				fmt.Fprintf(b, "\t%s %s `db:\"%s,opt\" gocpp:\"%s\"`\n", f.name, f.t, f.name, f.name)
			} else {
				fmt.Fprintf(b, "\t%s %s\n", f.name, f.t)
			}
		}
		b.WriteString("}\n\n")
	}
	for _, t := range g.ifaces {
		fmt.Fprintf(b, "type %s interface {\n", t.name)
		for _, m := range t.methods {
			fmt.Fprintf(b, "\t%s(%s)%s\n", m.name, typeList(m.params), resultList(m.results))
		}
		b.WriteString("}\n\n")
	}
	g.usesFmtAlways()
}

func (g *gen) usesFmtAlways() { g.needFmt = true }
