// Package gen generates random, valid Go programs whose behaviour the Go specification fully
// defines, for differential testing of gocpp against real Go (step 15.9).
//
// A program is a set of top-level declarations and one entry function prog_N. Everything in a
// program has the prefix pN_ (types, functions, scenarios), so any number of programs can live
// in one package: a bundle. Programs only print through fmt and never depend on map iteration
// order, cap() after append growth, addresses, goroutine scheduling, or the other
// implementation-defined corners (lint.go checks this on the generated source).
package gen

import (
	"fmt"
	"go/format"
	"math/rand/v2"
	"sort"
	"strings"
)

// Version identifies the generator: programs of one seed and id are the same only for the same
// Version, so campaign state is kept per Version (difftest -state). Bump it whenever a change
// makes the generator print a different program for some seed.
const Version = 2

// Config says what to generate.
type Config struct {
	Seed     uint64
	Features Features
	// Scenarios is how many scenario functions one program has (0: 2 to 5 at random).
	Scenarios int
	// Fatal makes a program end with an uncaught panic (a fatal tail) with the given chance in
	// percent. Programs with a fatal tail print the same output in a bundle, where the panic
	// is recovered and printed, and exit with code 2 when run alone.
	FatalPercent int
}

// Program is one generated program.
type Program struct {
	ID      int
	Src     string   // top-level declarations, gofmt-formatted
	Entry   string   // the name of the entry function: prog_N
	Tail    string   // "" or the kind of the fatal tail, such as "panic:string"
	Imports []string // standard packages the declarations use
	Used    []string // the scenarios the program is made of
	Lines   int
}

// Var is a variable of the program being built.
type Var struct {
	Name   string
	T      *Type
	MinLen int  // slices and strings: a lower bound of the length
	NonNil bool // pointers, maps: known to be non-nil
	Used   bool
	Depth  int  // len(scopes) where it was declared: appends deeper down may not run
	RO     bool // never assigned by generated statements (a loop counter)
	Param  bool // a parameter or receiver: may stay unused
	NoRead bool // do not use in generated expressions (the variable is being mutated by a goroutine, ...)
}

type scope struct {
	vars []*Var
	loop bool // the block is the body of a for statement
}

// Gen builds one program.
type Gen struct {
	r   *rand.Rand
	f   Features
	id  int
	pfx string // "p17_"

	top  strings.Builder // top-level declarations
	cur  *strings.Builder
	ind  int
	nid  int
	scps []*scope

	types   []*Type // defined struct and basic types
	funcs   []*Func // pure functions
	imports map[string]bool
	used    []string
	tail    string
	cfg     Config

	declared map[string]bool

	lastCtor string // see valMin
	lastMin  int
}

// Func is a top-level function the generator may call from expressions.
type Func struct {
	Name   string
	Params []*Type
	Result *Type
	Pure   bool
}

// Generate makes program number id of the run with the config's seed. The same seed, id,
// features and scenario count always give the same program.
func Generate(cfg Config, id int) (*Program, error) {
	if cfg.Features == nil {
		cfg.Features = FeaturesForPhase(9)
	}
	g := &Gen{
		r:        rand.New(rand.NewPCG(cfg.Seed^0x9e3779b97f4a7c15, uint64(id)*0x2545f4914f6cdd1d+1)),
		f:        cfg.Features,
		id:       id,
		pfx:      fmt.Sprintf("p%d_", id),
		imports:  map[string]bool{"fmt": true},
		declared: map[string]bool{},
		cfg:      cfg,
	}
	g.build()
	src := g.top.String()
	out, err := format.Source([]byte("package main\n\n" + src))
	if err != nil {
		return nil, fmt.Errorf("program %d: generated source does not parse: %v\n%s", id, err, numbered(src))
	}
	body := strings.TrimPrefix(string(out), "package main\n\n")
	imps := make([]string, 0, len(g.imports))
	for k := range g.imports {
		imps = append(imps, k)
	}
	sort.Strings(imps)
	return &Program{ID: id, Src: body, Entry: fmt.Sprintf("prog_%d", id), Tail: g.tail, Imports: imps,
		Used: g.used, Lines: strings.Count(body, "\n")}, nil
}

func numbered(s string) string {
	var b strings.Builder
	for i, l := range strings.Split(s, "\n") {
		fmt.Fprintf(&b, "%4d  %s\n", i+1, l)
	}
	return b.String()
}

// ---- random helpers ----

func (g *Gen) rn(n int) int {
	if n <= 1 {
		return 0
	}
	return g.r.IntN(n)
}

// between returns a number in [lo, hi].
func (g *Gen) between(lo, hi int) int { return lo + g.rn(hi-lo+1) }

// chance is true with the given probability in percent.
func (g *Gen) chance(pct int) bool { return g.rn(100) < pct }

func pick[T any](g *Gen, s []T) T { return s[g.rn(len(s))] }

func (g *Gen) has(f string) bool { return g.f[f] }

type weighted struct {
	w    int
	name string
	f    func()
}

func (g *Gen) choose(ws []weighted) {
	total := 0
	for _, w := range ws {
		total += w.w
	}
	if total == 0 {
		return
	}
	x := g.rn(total)
	for _, w := range ws {
		if x < w.w {
			w.f()
			return
		}
		x -= w.w
	}
}

// ---- names and emission ----

func (g *Gen) fresh(base string) string {
	g.nid++
	return fmt.Sprintf("%s%d", base, g.nid)
}

// topName is a fresh program-level name.
func (g *Gen) topName(base string) string {
	g.nid++
	return fmt.Sprintf("%s%s%d", g.pfx, base, g.nid)
}

func (g *Gen) ln(format string, args ...any) {
	g.cur.WriteString(strings.Repeat("\t", g.ind))
	fmt.Fprintf(g.cur, format, args...)
	g.cur.WriteByte('\n')
}

func (g *Gen) importPkg(p string) { g.imports[p] = true }

// raw writes the lines of text (no formatting), indented.
func (g *Gen) raw(text string) {
	for _, l := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if l == "" {
			g.cur.WriteByte('\n')
			continue
		}
		g.ln("%s", l)
	}
}

// tpl writes the lines of text after replacing every @KEY@ by its value (key, value, ...).
func (g *Gen) tpl(text string, kv ...string) {
	var pairs []string
	for i := 0; i+1 < len(kv); i += 2 {
		pairs = append(pairs, "@"+kv[i]+"@", kv[i+1])
	}
	g.raw(strings.NewReplacer(pairs...).Replace(text))
}

func (g *Gen) topf(format string, args ...any) {
	fmt.Fprintf(&g.top, format, args...)
	g.top.WriteString("\n")
}

// close ends the block, first using every variable of the scope that nothing used.
func (g *Gen) close() {
	g.sweep()
	g.scps = g.scps[:len(g.scps)-1]
	g.ind--
	g.ln("}")
}

func (g *Gen) sweep() {
	sc := g.scps[len(g.scps)-1]
	var unused []string
	for _, v := range sc.vars {
		// Used is only a hint (an expression that read the variable may have been thrown
		// away): every local is used here, which is always valid Go
		if !v.Param {
			unused = append(unused, v.Name)
		}
	}
	for _, n := range unused {
		g.ln("_ = %s", n)
	}
	for _, v := range sc.vars {
		v.Used = true
	}
}

func (g *Gen) pushScope() { g.scps = append(g.scps, &scope{}) }
func (g *Gen) popScope()  { g.scps = g.scps[:len(g.scps)-1] }

// bare opens a scope in the middle of a header the caller wrote ("for ... {").
func (g *Gen) enter(header string) {
	g.ln("%s {", header)
	g.ind++
	g.pushScope()
	g.scps[len(g.scps)-1].loop = strings.HasPrefix(header, "for ") || header == "for"
}

// grow records that the length of v is now at least n more, when that is certain: the
// statement is in the block that declares v (a nested block may not run).
func (g *Gen) grow(v *Var, n int) {
	if v.Depth == len(g.scps) {
		v.MinLen += n
	}
}

// orderIndependent marks a range over a map whose body gives the same result in any order
// (the lint accepts such loops).
const orderIndependent = "// order independent"

// enterRaw is enter for a header that already holds its "{" (and a trailing comment).
func (g *Gen) enterRaw(headerWithBrace string) {
	g.ln("%s", headerWithBrace)
	g.ind++
	g.pushScope()
	g.scps[len(g.scps)-1].loop = true
}

// inLoop reports whether the code being generated may run more than once because of a for
// statement: knowledge that a statement shrinks (a minimum length) is only valid outside.
func (g *Gen) inLoop() bool {
	for _, sc := range g.scps {
		if sc.loop {
			return true
		}
	}
	return false
}

func (g *Gen) add(v *Var) *Var {
	v.Depth = len(g.scps)
	sc := g.scps[len(g.scps)-1]
	sc.vars = append(sc.vars, v)
	return v
}

// declare writes "name := init" and records the var.
func (g *Gen) declare(base string, t *Type, init string) *Var {
	name := g.fresh(base)
	g.ln("%s := %s", name, init)
	// a map is known to be non-nil only when it was made here (a map read from a slice, array,
	// map or struct may be nil)
	nonNil := t.K == KPtr || (t.K == KMap && (strings.HasPrefix(init, t.Name+"{") || strings.HasPrefix(init, "make(")))
	return g.add(&Var{Name: name, T: t, NonNil: nonNil})
}

// declareVal declares a variable with a random value of type t.
func (g *Gen) declareVal(base string, t *Type) *Var {
	init, min := g.valMin(t, 2)
	v := g.declare(base, t, init)
	v.MinLen = min
	return v
}

// bind records a variable that the caller declared itself (a parameter, a range variable).
func (g *Gen) bindRO(name string, t *Type) *Var {
	v := g.bind(name, t)
	v.RO = true
	return v
}

func (g *Gen) bind(name string, t *Type) *Var {
	return g.add(&Var{Name: name, T: t})
}

// vars returns the variables in scope with exactly type t.
func (g *Gen) varsOf(t *Type) []*Var {
	var vs []*Var
	for _, sc := range g.scps {
		for _, v := range sc.vars {
			if !v.NoRead && v.T.Name == t.Name {
				vs = append(vs, v)
			}
		}
	}
	return vs
}

func (g *Gen) allVars() []*Var {
	var vs []*Var
	for _, sc := range g.scps {
		for _, v := range sc.vars {
			if !v.NoRead {
				vs = append(vs, v)
			}
		}
	}
	return vs
}

// ---- program assembly ----

type scenario struct {
	name   string
	weight int
	need   []string // features
	fn     func(g *Gen)
}

var scenarios []scenario

func register(name string, weight int, need []string, fn func(g *Gen)) {
	scenarios = append(scenarios, scenario{name, weight, need, fn})
}

func (g *Gen) enabled(s scenario) bool {
	for _, n := range s.need {
		if !g.f[n] {
			return false
		}
	}
	return true
}

func (g *Gen) build() {
	var avail []scenario
	total := 0
	for _, s := range scenarios {
		if g.enabled(s) {
			avail = append(avail, s)
			total += s.weight
		}
	}
	n := g.cfg.Scenarios
	if n <= 0 {
		n = g.between(2, 5)
	}
	var names []string
	var bodies []string
	if len(avail) > 0 {
		for i := 0; i < n; i++ {
			x := g.rn(total)
			var s scenario
			for _, c := range avail {
				if x < c.weight {
					s = c
					break
				}
				x -= c.weight
			}
			fname := g.topName("s")
			g.used = append(g.used, s.name)
			g.genFunc(fname, s)
			names = append(names, fname)
			bodies = append(bodies, s.name)
		}
	}
	tailFn := ""
	if g.cfg.FatalPercent > 0 && g.chance(g.cfg.FatalPercent) {
		tailFn = g.genTail()
	}
	g.topf("func prog_%d() {", g.id)
	for i, n := range names {
		g.topf("\ttry(%q, %s)", bodies[i], n)
	}
	if tailFn != "" {
		g.topf("\ttail(%s)", tailFn)
	}
	g.topf("}")
	g.topf("")
}

// genFunc generates the function of one scenario into the top level buffer.
func (g *Gen) genFunc(name string, s scenario) {
	saved, savedInd, savedScps := g.cur, g.ind, g.scps
	var b strings.Builder
	g.cur, g.ind, g.scps = &b, 1, nil
	g.pushScope()
	func() {
		defer func() {
			if r := recover(); r != nil {
				panic(fmt.Sprintf("scenario %s of program %d: %v", s.name, g.id, r))
			}
		}()
		s.fn(g)
	}()
	g.sweep()
	g.popScope()
	g.cur, g.ind, g.scps = saved, savedInd, savedScps
	g.topf("// scenario: %s", s.name)
	g.topf("func %s() {", name)
	g.top.WriteString(b.String())
	g.topf("}")
	g.topf("")
}

// topFunc writes a top-level function: sig is everything after "func " up to the body's "{".
// params are bound as variables. The body must end in a terminating statement when the
// function has results, so it sweeps unused variables itself (g.sweep) before returning.
func (g *Gen) topFunc(sig string, params []*Var, body func()) {
	saved, savedInd, savedScps := g.cur, g.ind, g.scps
	var b strings.Builder
	g.cur, g.ind, g.scps = &b, 1, nil
	g.pushScope()
	for _, p := range params {
		g.add(p)
		p.Used = true
		p.Param = true
	}
	body()
	g.popScope()
	g.cur, g.ind, g.scps = saved, savedInd, savedScps
	g.topf("func %s {", sig)
	g.top.WriteString(b.String())
	g.topf("}")
	g.topf("")
}
