// Command mustgen writes random programs that use the `!` forms of Phase 10, each with a hand-written
// plain-Go twin, for the differential test of step 10.5.
//
//	go run ./tests/mustgen -n 20 -seed 1 -out tests/run
//
// writes tests/run/must_gen_NN.gp (a program with `!`, or with a must function that has no `!` in its
// body) and tests/run/testdata/must_twin/must_gen_NN.go (the same program as a Go programmer writes it, with
// if err != nil). It runs the twin with real Go and writes what it prints as the `// Output:` block of
// the .gp file, so the expected output comes from real Go; the harness (TestRun and TestRunVsGo with
// GOCPP_TEST_PHASE=10) then compares what gocpp prints with it.
//
// A program has four fallible functions that print a trace line and fail for some arguments, and
// a function run(seed) of random statements over four int variables: `!` calls in assignments, nested in
// expressions, in if / else if / switch conditions, in the condition of a for loop, in the operands of
// && and ||, as statements, and in defer. The lowering of the twin is the one of the README, written here
// independently of internal/check: every call to the left is hoisted first, a call in the right operand
// of && or || only runs when the left operand does not decide, a call in an else if runs only in the
// else block, a call in a loop condition runs on every iteration, and continue goes to the post
// statement.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// An expr is an int expression: f1!(e) (kind 'c'), a constant ('k'), a variable ('v'), or l + r, l - r.
type expr struct {
	kind byte
	n    int
	l, r *expr
}

// A cond is a bool expression: b1!(e) ('b'), e < f ('<'), l && r ('&'), l || r ('|'), !l ('!').
type cond struct {
	kind byte
	e, f *expr
	l, r *cond
}

// A stmt is a statement of run.
type stmt struct {
	kind  string // assign, assign2, call3, add, if, switch, tagless, for, jump
	k, l  int
	e     *expr
	tag   *expr
	conds []*cond
	body  [][]*stmt // the bodies of the branches, and of the cases
	els   []*stmt
	hasEl bool
	n     int // for: the bound
	id    int // for: the variable
	jump  string
}

type gen struct {
	rng  *rand.Rand
	must bool // the program uses a must function whose calls have no !
	tmp  int
	loop int

	// brk holds, while the twin is written, the statements that a break can leave: nil for a
	// for or a switch of Go, and for a tagless switch (which the twin writes as an if chain) the
	// label after the chain, so that a break there is a goto.
	brk []*brkTarget
}

func (g *gen) top() *brkTarget {
	if len(g.brk) == 0 {
		return nil
	}
	return g.brk[len(g.brk)-1]
}

type brkTarget struct {
	label string
	used  bool
}

func (g *gen) bang() string {
	if g.must {
		return ""
	}
	return "!"
}

func (g *gen) fresh() int { g.tmp++; return g.tmp }

func (g *gen) expr(depth int) *expr {
	switch r := g.rng.Intn(10); {
	case r < 3 || depth == 0:
		if g.rng.Intn(4) == 0 {
			return &expr{kind: 'k', n: g.rng.Intn(10)}
		}
		return &expr{kind: 'v', n: g.rng.Intn(4)}
	case r < 6:
		return &expr{kind: "+-"[g.rng.Intn(2)], l: g.expr(depth - 1), r: g.expr(depth - 1)}
	default:
		return &expr{kind: 'c', l: g.expr(depth - 1)}
	}
}

func (g *gen) cond(depth int) *cond {
	switch r := g.rng.Intn(10); {
	case r < 3 || depth == 0:
		if g.rng.Intn(3) == 0 {
			return &cond{kind: '<', e: g.expr(1), f: g.expr(1)}
		}
		return &cond{kind: 'b', e: g.expr(1)}
	case r < 6:
		return &cond{kind: "&|"[g.rng.Intn(2)], l: g.cond(depth - 1), r: g.cond(depth - 1)}
	case r < 7:
		return &cond{kind: '!', l: g.cond(depth - 1)}
	default:
		return &cond{kind: 'b', e: g.expr(depth - 1)}
	}
}

func (g *gen) list(depth int) []*stmt {
	n := 1 + g.rng.Intn(3)
	var out []*stmt
	for i := 0; i < n; i++ {
		out = append(out, g.stmt(depth))
	}
	return out
}

func (g *gen) stmt(depth int) *stmt {
	r := g.rng.Intn(16)
	if depth == 0 && r >= 7 {
		r = g.rng.Intn(7)
	}
	switch {
	case r < 2:
		return &stmt{kind: "assign", k: g.rng.Intn(4), e: g.expr(2)}
	case r < 3:
		k, l := g.rng.Intn(4), g.rng.Intn(4)
		for l == k {
			l = g.rng.Intn(4)
		}
		return &stmt{kind: "assign2", k: k, l: l, e: g.expr(1)}
	case r < 4:
		return &stmt{kind: "call3", e: g.expr(1)}
	case r < 5:
		return &stmt{kind: "add", k: g.rng.Intn(4), e: g.expr(2)}
	case r < 7:
		if g.loop > 0 {
			c := &cond{kind: '<', e: g.expr(1), f: &expr{kind: 'k', n: 3 + g.rng.Intn(5)}}
			return &stmt{kind: "jump", conds: []*cond{c}, jump: []string{"continue", "break"}[g.rng.Intn(2)]}
		}
		return &stmt{kind: "assign", k: g.rng.Intn(4), e: g.expr(1)}
	case r < 10:
		n := 1 + g.rng.Intn(3)
		s := &stmt{kind: "if", hasEl: g.rng.Intn(2) == 0}
		for i := 0; i < n; i++ {
			s.conds = append(s.conds, g.cond(2))
			s.body = append(s.body, g.list(depth-1))
		}
		if s.hasEl {
			s.els = g.list(depth - 1)
		}
		return s
	case r < 12:
		s := &stmt{kind: "switch", tag: g.expr(2)}
		for i := 0; i < 2; i++ {
			s.body = append(s.body, g.list(depth-1))
		}
		s.els = g.list(depth - 1)
		return s
	case r < 13:
		n := 1 + g.rng.Intn(3)
		s := &stmt{kind: "tagless"}
		for i := 0; i < n; i++ {
			s.conds = append(s.conds, g.cond(2))
			s.body = append(s.body, g.list(depth-1))
		}
		s.hasEl = g.rng.Intn(2) == 0
		if s.hasEl {
			s.els = g.list(depth - 1)
		}
		return s
	default:
		s := &stmt{kind: "for", n: 1 + g.rng.Intn(3), id: g.fresh()}
		if g.rng.Intn(4) != 0 {
			s.conds = []*cond{g.cond(2)}
		}
		g.loop++
		s.body = [][]*stmt{g.list(depth - 1)}
		g.loop--
		return s
	}
}

// --- the program with `!`

type out struct {
	b      *strings.Builder
	indent int
}

func (o *out) line(f string, a ...any) {
	o.b.WriteString(strings.Repeat("\t", o.indent))
	fmt.Fprintf(o.b, f, a...)
	o.b.WriteByte('\n')
}

func (g *gen) gpExpr(e *expr) string {
	switch e.kind {
	case 'k':
		return fmt.Sprint(e.n)
	case 'v':
		return fmt.Sprintf("v%d", e.n)
	case 'c':
		return "f1" + g.bang() + "(" + g.gpExpr(e.l) + ")"
	}
	return "(" + g.gpExpr(e.l) + " " + string(e.kind) + " " + g.gpExpr(e.r) + ")"
}

func (g *gen) gpCond(c *cond) string {
	switch c.kind {
	case 'b':
		return "b1" + g.bang() + "(" + g.gpExpr(c.e) + ")"
	case '<':
		return g.gpExpr(c.e) + " < " + g.gpExpr(c.f)
	case '!':
		return "!(" + g.gpCond(c.l) + ")"
	}
	return "(" + g.gpCond(c.l) + " " + string(c.kind) + string(c.kind) + " " + g.gpCond(c.r) + ")"
}

func (g *gen) gpList(o *out, list []*stmt) {
	for _, s := range list {
		g.gpStmt(o, s)
	}
}

func (g *gen) gpStmt(o *out, s *stmt) {
	in := func(f func()) { o.indent++; f(); o.indent-- }
	switch s.kind {
	case "assign":
		o.line("v%d = %s", s.k, g.gpExpr(s.e))
	case "assign2":
		o.line("v%d, v%d = f2%s(%s)", s.k, s.l, g.bang(), g.gpExpr(s.e))
	case "call3":
		o.line("f3%s(%s)", g.bang(), g.gpExpr(s.e))
	case "add":
		o.line("v%d += %s", s.k, g.gpExpr(s.e))
	case "jump":
		o.line("if %s {", g.gpCond(s.conds[0]))
		o.line("\t%s", s.jump)
		o.line("}")
	case "if":
		for i, c := range s.conds {
			if i == 0 {
				o.line("if %s {", g.gpCond(c))
			} else {
				o.line("} else if %s {", g.gpCond(c))
			}
			in(func() { g.gpList(o, s.body[i]) })
		}
		if s.hasEl {
			o.line("} else {")
			in(func() { g.gpList(o, s.els) })
		}
		o.line("}")
	case "switch":
		o.line("switch %s %% 3 {", g.gpExpr(s.tag))
		for i, b := range s.body {
			o.line("case %d:", i)
			in(func() { g.gpList(o, b) })
		}
		o.line("default:")
		in(func() { g.gpList(o, s.els) })
		o.line("}")
	case "tagless":
		o.line("switch {")
		for i, c := range s.conds {
			o.line("case %s:", g.gpCond(c))
			in(func() { g.gpList(o, s.body[i]) })
		}
		if s.hasEl {
			o.line("default:")
			in(func() { g.gpList(o, s.els) })
		}
		o.line("}")
	case "for":
		head := fmt.Sprintf("i%d := 0; i%d < %d", s.id, s.id, s.n)
		if len(s.conds) > 0 {
			head += " && " + g.gpCond(s.conds[0])
		}
		o.line("for %s; i%d++ {", head, s.id)
		in(func() { g.gpList(o, s.body[0]) })
		o.line("}")
	}
}

// --- the twin: plain Go

func hasCall(e *expr) bool {
	return e != nil && (e.kind == 'c' || hasCall(e.l) || hasCall(e.r))
}

func condHasCall(c *cond) bool {
	return c != nil && (c.kind == 'b' || hasCall(c.e) || hasCall(c.f) || condHasCall(c.l) || condHasCall(c.r))
}

// goExpr writes the statements that make the calls of e in Go's order and returns the Go expression
// for its value.
func (g *gen) goExpr(o *out, e *expr) string {
	switch e.kind {
	case 'k':
		return fmt.Sprint(e.n)
	case 'v':
		return fmt.Sprintf("v%d", e.n)
	case 'c':
		a := g.goExpr(o, e.l)
		t := g.fresh()
		o.line("t%d, err := f1(%s)", t, a)
		o.line("if err != nil {")
		o.line("\treturn 0, err")
		o.line("}")
		return fmt.Sprintf("t%d", t)
	}
	l := g.goExpr(o, e.l)
	r := g.goExpr(o, e.r)
	return "(" + l + " " + string(e.kind) + " " + r + ")"
}

func (g *gen) goCond(o *out, c *cond) string {
	switch c.kind {
	case 'b':
		a := g.goExpr(o, c.e)
		t := g.fresh()
		o.line("t%d, err := b1(%s)", t, a)
		o.line("if err != nil {")
		o.line("\treturn 0, err")
		o.line("}")
		return fmt.Sprintf("t%d", t)
	case '<':
		l := g.goExpr(o, c.e)
		r := g.goExpr(o, c.f)
		return l + " < " + r
	case '!':
		return "!(" + g.goCond(o, c.l) + ")"
	}
	l := g.goCond(o, c.l)
	if !condHasCall(c.r) {
		r := g.goCond(o, c.r)
		return "(" + l + " " + string(c.kind) + string(c.kind) + " " + r + ")"
	}
	t := g.fresh()
	o.line("c%d := %s", t, l)
	if c.kind == '&' {
		o.line("if c%d {", t)
	} else {
		o.line("if !c%d {", t)
	}
	o.indent++
	r := g.goCond(o, c.r)
	o.line("c%d = %s", t, r)
	o.indent--
	o.line("}")
	return fmt.Sprintf("c%d", t)
}

func (g *gen) twinList(o *out, list []*stmt) {
	for _, s := range list {
		g.twinStmt(o, s)
	}
}

// twinChain writes the if chain conds[i:], with the bodies and, at the end, els (when there is one).
func (g *gen) twinChain(o *out, s *stmt, i int) {
	if i == len(s.conds) {
		if s.hasEl {
			g.twinList(o, s.els)
		}
		return
	}
	val := g.goCond(o, s.conds[i])
	o.line("if %s {", val)
	o.indent++
	g.twinList(o, s.body[i])
	o.indent--
	if i+1 < len(s.conds) || s.hasEl {
		o.line("} else {")
		o.indent++
		g.twinChain(o, s, i+1)
		o.indent--
	}
	o.line("}")
}

func (g *gen) twinStmt(o *out, s *stmt) {
	o.line("{")
	o.indent++
	defer func() {
		o.indent--
		o.line("}")
	}()
	switch s.kind {
	case "assign":
		v := g.goExpr(o, s.e)
		o.line("v%d = %s", s.k, v)
	case "assign2":
		v := g.goExpr(o, s.e)
		t := g.fresh()
		o.line("a%d, b%d, err := f2(%s)", t, t, v)
		o.line("if err != nil {")
		o.line("\treturn 0, err")
		o.line("}")
		o.line("v%d, v%d = a%d, b%d", s.k, s.l, t, t)
	case "call3":
		v := g.goExpr(o, s.e)
		o.line("if err := f3(%s); err != nil {", v)
		o.line("\treturn 0, err")
		o.line("}")
	case "add":
		v := g.goExpr(o, s.e)
		o.line("v%d += %s", s.k, v)
	case "jump":
		v := g.goCond(o, s.conds[0])
		o.line("if %s {", v)
		if t := g.top(); s.jump == "break" && t != nil {
			t.used = true
			o.line("\tgoto %s", t.label)
		} else {
			o.line("\t%s", s.jump)
		}
		o.line("}")
	case "if":
		g.twinChain(o, s, 0)
	case "tagless":
		// A break in a case leaves the switch: the chain is a block, and break is goto after it.
		t := &brkTarget{label: fmt.Sprintf("sw%d", g.fresh())}
		g.brk = append(g.brk, t)
		var cb strings.Builder
		g.twinChain(&out{b: &cb, indent: o.indent}, s, 0)
		g.brk = g.brk[:len(g.brk)-1]
		o.b.WriteString(cb.String())
		if t.used {
			o.indent--
			o.line("}")
			o.line("%s:", t.label)
			o.line("{")
			o.indent++
		}
	case "switch":
		v := g.goExpr(o, s.tag)
		o.line("switch %s %% 3 {", v)
		g.brk = append(g.brk, nil)
		defer func() { g.brk = g.brk[:len(g.brk)-1] }()
		for i, b := range s.body {
			o.line("case %d:", i)
			o.indent++
			g.twinList(o, b)
			o.indent--
		}
		o.line("default:")
		o.indent++
		g.twinList(o, s.els)
		o.indent--
		o.line("}")
	case "for":
		o.line("for i%d := 0; ; i%d++ {", s.id, s.id)
		g.brk = append(g.brk, nil)
		defer func() { g.brk = g.brk[:len(g.brk)-1] }()
		o.indent++
		o.line("if !(i%d < %d) {", s.id, s.n)
		o.line("\tbreak")
		o.line("}")
		if len(s.conds) > 0 {
			v := g.goCond(o, s.conds[0])
			o.line("if !(%s) {", v)
			o.line("\tbreak")
			o.line("}")
		}
		g.twinList(o, s.body[0])
		o.indent--
		o.line("}")
	}
}

const prelude = `var ErrBad = errors.New("bad")

func f1(n int) (int, error) {
	fmt.Println("f1", n)
	if n%5 == 3 {
		return 0, fmt.Errorf("f1(%d): %w", n, ErrBad)
	}
	return n*2 + 1, nil
}

func f2(n int) (int, int, error) {
	fmt.Println("f2", n)
	if n%6 == 2 {
		return 0, 0, fmt.Errorf("f2(%d): %w", n, ErrBad)
	}
	return n + 1, n - 1, nil
}

func f3(n int) error {
	fmt.Println("f3", n)
	if n%7 == 4 {
		return fmt.Errorf("f3(%d): %w", n, ErrBad)
	}
	return nil
}

func b1(n int) (bool, error) {
	fmt.Println("b1", n)
	if n%8 == 5 {
		return false, fmt.Errorf("b1(%d): %w", n, ErrBad)
	}
	return n%2 == 0, nil
}
`

const mainFunc = `
func main() {
	for seed := 0; seed < 10; seed++ {
		v, err := run(seed)
		fmt.Println("run", seed, v, err, errors.Is(err, ErrBad))
	}
}
`

// program returns the .gp and the twin text of program i.
func program(seed int64, i int) (gp, twin string) {
	g := &gen{rng: rand.New(rand.NewSource(seed + int64(i)*7919)), must: i%3 == 1}
	body := g.list(3)
	for len(body) < 3 {
		body = append(body, g.stmt(2))
	}
	// 0 to 2 defers with a constant argument; they run at exit and may fail it.
	var defers []int
	for n := g.rng.Intn(3); n > 0; n-- {
		defers = append(defers, g.rng.Intn(30))
	}
	header := "package main\n\nimport (\n\t\"errors\"\n\t\"fmt\"\n)\n\n" + prelude + "\n"
	var gb, tb strings.Builder
	fmt.Fprintf(&gb, "// phase: 10\n\n// Generated by tests/mustgen (seed %d, program %d): ", seed, i)
	if g.must {
		gb.WriteString("a must function without ! in its body;\n")
	} else {
		gb.WriteString("! calls in random places;\n")
	}
	gb.WriteString("// its twin in testdata/must_twin/ is the same program in plain Go.\n")
	gb.WriteString(header)
	tb.WriteString(header)
	if g.must {
		gb.WriteString("func run!(seed int) int {\n")
	} else {
		gb.WriteString("func run(seed int) (int, error) {\n")
	}
	tb.WriteString("func run(seed int) (res int, err error) {\n")
	decl := "\tv0, v1, v2, v3 := seed, seed*3+1, seed%4, 7-seed\n"
	gb.WriteString(decl)
	tb.WriteString(decl)
	for _, d := range defers {
		fmt.Fprintf(&gb, "\tdefer f3%s(%d)\n", g.bang(), d)
		fmt.Fprintf(&tb, "\tdefer func() {\n\t\tif e := f3(%d); err == nil {\n\t\t\terr = e\n\t\t}\n\t}()\n", d)
	}
	g.gpList(&out{b: &gb, indent: 1}, body)
	g.tmp = 100
	g.twinList(&out{b: &tb, indent: 1}, body)
	ret := "(v0*1000 + v1*100 + v2*10 + v3) % 100000"
	if g.must {
		fmt.Fprintf(&gb, "\treturn %s\n}\n", ret)
	} else {
		fmt.Fprintf(&gb, "\treturn %s, nil\n}\n", ret)
	}
	fmt.Fprintf(&tb, "\treturn %s, nil\n}\n", ret)
	gb.WriteString(mainFunc)
	tb.WriteString(mainFunc)
	return gb.String(), tb.String()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "mustgen:", err)
	os.Exit(1)
}

func main() {
	n := flag.Int("n", 20, "number of programs")
	seed := flag.Int64("seed", 1, "seed")
	dir := flag.String("out", "tests/run", "output directory (tests/run)")
	mode := flag.String("mode", "gen", "gen (random statements) or ord (the order of evaluation around ! calls, ord.go)")
	flag.Parse()
	if *mode == "ord" {
		ordMain(*n, *seed, *dir)
		return
	}
	if err := os.MkdirAll(filepath.Join(*dir, "testdata", "must_twin"), 0o755); err != nil {
		fail(err)
	}
	tmp, err := os.MkdirTemp("", "mustgen")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(tmp)
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module t\n\ngo 1.25\n"), 0o644); err != nil {
		fail(err)
	}
	for i := 0; i < *n; i++ {
		gp, twin := program(*seed, i)
		name := fmt.Sprintf("must_gen_%02d", i)
		if err := os.WriteFile(filepath.Join(tmp, "main.go"), []byte(twin), 0o644); err != nil {
			fail(err)
		}
		cmd := exec.Command("go", "run", ".")
		cmd.Dir = tmp
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=")
		res, err := cmd.CombinedOutput()
		if err != nil {
			fail(fmt.Errorf("%s: the twin does not run: %v\n%s", name, err, res))
		}
		var ob strings.Builder
		ob.WriteString("\n// Output:\n")
		for _, l := range strings.Split(strings.TrimRight(string(res), "\n"), "\n") {
			if l == "" {
				ob.WriteString("//\n")
			} else {
				ob.WriteString("// " + l + "\n")
			}
		}
		if err := os.WriteFile(filepath.Join(*dir, name+".gp"), []byte(gp+ob.String()), 0o644); err != nil {
			fail(err)
		}
		if err := os.WriteFile(filepath.Join(*dir, "testdata", "must_twin", name+".go"), []byte(twin), 0o644); err != nil {
			fail(err)
		}
	}
}
