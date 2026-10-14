package cppgen

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestRandomOrder is an opt-in differential test of the evaluation order (order.go): it
// generates random programs whose calls print and change package variables, in every place
// that Phase 02 has (arguments, operands, && and ||, conditions of if, else if, for and
// switch, case lists, post statements, range bounds, returns, assignments and declarations),
// next to reads of those variables and to operations that can panic (also substrings,
// concatenations and range over strings, Phase 03), and compares what each
// prints with real Go, on both compilers.
//
//	GOCPP_RANDOM_TESTS=1 go test ./internal/cppgen -run TestRandomOrder
//
// GOCPP_RANDOM_N sets the number of programs (default 40), GOCPP_RANDOM_SEED the first seed
// (default: the time); a failure names the seed of its program, and
// GOCPP_RANDOM_SEED=<seed> GOCPP_RANDOM_N=1 runs it again. GOCPP_RANDOM_DIR=<dir> keeps the
// programs there, as <seed>.gp.
func TestRandomOrder(t *testing.T) {
	if os.Getenv("GOCPP_RANDOM_TESTS") == "" {
		t.Skip("set GOCPP_RANDOM_TESTS=1 to run the random evaluation-order test")
	}
	n := envInt(t, "GOCPP_RANDOM_N", 40)
	seed := int64(envInt(t, "GOCPP_RANDOM_SEED", int(time.Now().UnixNano()%1_000_000_000)))
	t.Logf("seeds %d to %d", seed, seed+int64(n)-1)
	for i := range n {
		s := seed + int64(i)
		t.Run("seed"+strconv.FormatInt(s, 10), func(t *testing.T) {
			t.Parallel()
			src := randomProgram(s)
			if dir := os.Getenv("GOCPP_RANDOM_DIR"); dir != "" {
				if err := os.WriteFile(filepath.Join(dir, strconv.FormatInt(s, 10)+".gp"), []byte(src), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			gp := filepath.Join(t.TempDir(), "main.gp")
			if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			checkRunsNorm(t, emitter(gp), goResult(t, gp), samePanics)
		})
	}
}

// samePanics makes the run-time panics of the operators one message: when one expression has
// two that both fail, with no call between them, gocpp may report the other one than Go
// (order.go).
func samePanics(r result) result {
	if r.code == 2 {
		lines := strings.Split(r.stderr, "\n")
		for i, l := range lines {
			if strings.HasPrefix(l, "panic: runtime error: ") {
				lines[i] = "panic: runtime error: (one of the operators)"
			}
		}
		r.stderr = strings.Join(lines, "\n")
	}
	return r
}

func envInt(t *testing.T, name string, def int) int {
	s := os.Getenv(name)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("%s=%s: %v", name, s, err)
	}
	return v
}

// randomProgram returns the program of the seed.
func randomProgram(seed int64) string {
	g := &progGen{r: rand.New(rand.NewSource(seed)), fixed: make(map[string]bool)}
	return g.program()
}

// A progGen writes one random program.
type progGen struct {
	r       *rand.Rand
	b       strings.Builder
	indent  int
	tag     int             // the next tag of a call, so the output shows which call ran
	locals  []string        // the int variables in scope
	block0  int             // the first of locals that the current block declares
	fixed   map[string]bool // the loop counters, which the program doesn't assign
	depth   int             // of the statement nesting
	loops   int             // the loops around the current statement
	nvars   int
	helpers int // the helpers h0, k0, ... that the code may call (only earlier ones: no recursion)
}

const randomPrelude = `package main

import "fmt"

var g0, g1 = 1, 2
var gs = "abcde"
var gb = true
var g8 uint8 = 200

// e prints its tag and changes the package variables.
func e(tag, v int) int {
	fmt.Print("e", tag, " ")
	g0 += tag
	g1 = g1*3 + v
	gb = !gb
	g8 += uint8(tag)
	if tag%3 == 0 {
		gs += "x"
	}
	return v
}

func eb(tag int, v bool) bool {
	fmt.Print("b", tag, " ")
	g0 -= tag
	gb = v
	return v
}

func both(a, b int) int {
	return a*7 + b
}

`

func (g *progGen) program() string {
	g.b.WriteString(randomPrelude)
	// Helpers with several results and returns: each may call e, and only e.
	for i := range 3 {
		g.helpers = i
		g.printf("func h%d(p int) (int, int) {", i)
		g.indent++
		g.locals = []string{"p"}
		g.stmts(2)
		g.printf("return %s, %s", g.expr(3), g.expr(3))
		g.indent--
		g.printf("}")
		g.printf("")
		g.printf("func k%d(p int) int {", i)
		g.indent++
		g.locals = []string{"p"}
		g.stmts(1)
		g.printf("return %s", g.expr(3))
		g.indent--
		g.printf("}")
		g.printf("")
	}
	g.helpers = 3
	g.printf("func main() {")
	g.indent++
	g.locals = []string{"l0", "l1", "l2"}
	g.printf("l0, l1, l2 := %d, %d, %d", g.r.Intn(9)-2, g.r.Intn(9), g.r.Intn(5)+1)
	g.stmts(14)
	g.printf("fmt.Println(l0, l1, l2, g0, g1, gs, gb, g8)")
	g.indent--
	g.printf("}")
	return g.b.String()
}

func (g *progGen) printf(format string, args ...any) {
	for range g.indent {
		g.b.WriteByte('\t')
	}
	fmt.Fprintf(&g.b, format, args...)
	g.b.WriteByte('\n')
}

func (g *progGen) chance(p float64) bool { return g.r.Float64() < p }

func (g *progGen) nextTag() int {
	g.tag++
	return g.tag
}

// local returns a local int variable of the current scope.
func (g *progGen) local() string {
	return g.locals[g.r.Intn(len(g.locals))]
}

// newVar declares a new local int variable in the current scope. Sometimes it shadows a
// variable of an outer block, whose name the value of the declaration may read (v1 := v1 + 1,
// for v1 := range v1 & 3 { v1 := v1 * 2 }): the C++ keeps the Go names (funcNames).
func (g *progGen) newVar() string {
	var outer []string
	for _, v := range g.locals[:g.block0] {
		if !slices.Contains(g.locals[g.block0:], v) {
			outer = append(outer, v)
		}
	}
	var v string
	if len(outer) > 0 && g.chance(0.3) {
		v = outer[g.r.Intn(len(outer))]
	} else {
		g.nvars++
		v = "v" + strconv.Itoa(g.nvars)
	}
	g.locals = append(g.locals, v)
	return v
}

// block writes the statements of a block; its variables go out of scope at the end.
func (g *progGen) block(n int) {
	saved, block0 := len(g.locals), g.block0
	g.block0 = saved
	g.indent++
	g.depth++
	g.stmts(n)
	g.depth--
	g.indent--
	g.locals, g.block0 = g.locals[:saved], block0
}

func (g *progGen) stmts(n int) {
	for range n {
		g.stmt()
	}
}

// target returns a variable to assign: a local or a package variable.
func (g *progGen) target() string {
	var free []string
	for _, v := range g.locals {
		if !g.fixed[v] {
			free = append(free, v)
		}
	}
	if len(free) == 0 || g.chance(0.3) {
		return []string{"g0", "g1"}[g.r.Intn(2)]
	}
	return free[g.r.Intn(len(free))]
}

// helper returns the name of a helper with the prefix (h or k) that the code may call, or "".
func (g *progGen) helper(prefix string) string {
	if g.helpers == 0 {
		return ""
	}
	return prefix + strconv.Itoa(g.r.Intn(g.helpers))
}

func (g *progGen) stmt() {
	k := g.r.Intn(16)
	if g.depth >= 2 && k >= 8 {
		k = g.r.Intn(8) // no deeper nesting
	}
	switch k {
	case 0, 1:
		g.printf("fmt.Println(%s)", g.args(1+g.r.Intn(4)))
	case 2:
		g.printf("println(%s)", g.args(1+g.r.Intn(3)))
	case 3:
		g.printf("%s = %s", g.target(), g.expr(3))
	case 4:
		ops := []string{"+=", "-=", "*=", "/=", "%=", "^=", "|="}
		op := ops[g.r.Intn(len(ops))]
		y := g.expr(2)
		if op == "/=" || op == "%=" {
			y = g.divisor()
		}
		g.printf("%s %s %s", g.target(), op, y)
	case 5:
		a, b := g.target(), g.target()
		if a == b {
			g.printf("%s, _ = %s, %s", a, g.expr(2), g.expr(2))
		} else {
			g.printf("%s, %s = %s, %s", a, b, g.expr(2), g.expr(2))
		}
	case 6:
		x, y := g.expr(2), g.expr(2)
		a, b := g.newVar(), g.newVar()
		if g.chance(0.5) {
			g.printf("%s, %s := %s, %s", a, b, x, y)
		} else {
			g.printf("var %s, %s = %s, %s", a, b, x, y)
		}
		g.printf("fmt.Println(%s, %s)", a, b)
	case 7:
		h := g.helper("h")
		if h == "" {
			g.printf("fmt.Println(%s)", g.args(2))
			break
		}
		x := h + "(" + g.expr(2) + ")"
		a, b := g.newVar(), g.newVar()
		g.printf("%s, %s := %s", a, b, x)
		g.printf("fmt.Println(%s, %s, both(%s(%s)))", a, b, g.helper("h"), g.expr(1))
	case 8, 9:
		g.ifStmt()
	case 10, 11:
		g.forStmt()
	case 12:
		str := g.chance(0.4) // range over a string, else over an int
		var x string
		if str {
			x = g.rangeStr()
		} else {
			x = g.expr(2) + " & 3" // before i is in scope
		}
		i := g.newVar()
		g.fixed[i] = true
		if str {
			g.printf("for %s, r := range %s {", i, x)
		} else {
			g.printf("for %s := range %s {", i, x)
		}
		g.loops++
		g.indent++
		if str {
			g.printf("fmt.Println(%q, %s, r)", i, i)
		} else {
			g.printf("fmt.Println(%q, %s)", i, i)
		}
		g.indent--
		g.block(2)
		g.loops--
		g.printf("}")
		g.locals = g.locals[:len(g.locals)-1]
	case 13, 14:
		g.switchStmt()
	default:
		if g.loops > 0 {
			g.printf("if %s {", g.cond(2))
			g.indent++
			if g.chance(0.5) {
				g.printf("continue")
			} else {
				g.printf("break")
			}
			g.indent--
			g.printf("}")
		} else {
			g.printf("_ = %s", g.expr(3))
		}
	}
}

func (g *progGen) ifStmt() {
	saved := len(g.locals)
	defer func() { g.locals = g.locals[:saved] }() // the scope of the init's variable ends
	if g.chance(0.3) {
		x := g.expr(2)
		v := g.newVar()
		g.printf("if %s := %s; %s < %d || %s {", v, x, v, g.r.Intn(9), g.cond(2))
	} else {
		g.printf("if %s {", g.cond(3))
	}
	g.block(2)
	for range g.r.Intn(3) {
		g.printf("} else if %s {", g.cond(3))
		g.block(2)
	}
	if g.chance(0.5) {
		g.printf("} else {")
		g.block(2)
	}
	g.printf("}")
}

// forStmt writes a loop that ends: its counter goes up by one in each iteration, at most
// three times.
func (g *progGen) forStmt() {
	saved := len(g.locals)
	x := g.expr(2) // the value of a second variable of the init, outside its scope
	c := g.newVar()
	g.fixed[c] = true
	g.loops++
	switch g.r.Intn(3) {
	case 0:
		// A post statement with calls may need temporaries.
		post := c + "++"
		switch g.r.Intn(3) {
		case 0:
			post = fmt.Sprintf("%s, _ = %s+1, %s", c, c, g.expr(2))
		case 1:
			post = fmt.Sprintf("%s += 1 + 0*(%s)", c, g.expr(2))
		}
		init := c + " := 0"
		cond := g.cond(2)
		if g.chance(0.4) {
			d := g.newVar()
			init = fmt.Sprintf("%s, %s := 0, %s", c, d, x)
			cond = d + " > " + strconv.Itoa(g.r.Intn(9)-4) + " || " + cond
		}
		g.printf("for %s; %s < 3 && (%s); %s {", init, c, cond, post)
		g.block(2)
		g.printf("}")
		g.locals = g.locals[:saved] // the scope of c and d ends
	case 1:
		g.printf("%s := 0", c)
		g.printf("for %s < 3 && %s {", c, g.cond(2))
		g.indent++
		g.printf("%s++", c)
		g.indent--
		g.block(2)
		g.printf("}")
	default:
		g.printf("for %s := 0; %s < 3; %s++ {", c, c, c)
		g.block(2)
		g.printf("}")
		g.locals = g.locals[:saved]
	}
	g.loops--
}

func (g *progGen) switchStmt() {
	if g.chance(0.3) {
		g.printf("switch {")
		for range 1 + g.r.Intn(3) {
			g.printf("case %s:", g.condList())
			g.block(1)
		}
	} else {
		g.printf("switch %s {", g.expr(2))
		other := func() string { return g.nonConst(2) }
		consts := g.r.Intn(3)
		for i := range 1 + g.r.Intn(3) {
			if i < consts {
				g.printf("case %d:", i*2-1)
			} else {
				list := []string{other()}
				for range g.r.Intn(3) {
					list = append(list, other())
				}
				g.printf("case %s:", strings.Join(list, ", "))
			}
			g.block(1)
		}
	}
	if g.chance(0.5) {
		g.printf("default:")
		g.block(1)
	}
	g.printf("}")
}

func (g *progGen) condList() string {
	list := []string{g.cond(2)}
	for range g.r.Intn(3) {
		list = append(list, g.cond(2))
	}
	return strings.Join(list, ", ")
}

// args returns arguments for fmt.Println or println: ints, bools, bytes, strings.
func (g *progGen) args(n int) string {
	list := make([]string, n)
	for i := range list {
		switch g.r.Intn(9) {
		case 0:
			list[i] = g.cond(2)
		case 1:
			// int(x) first: an untyped constant shifted in x would get the type uint8.
			list[i] = []string{"uint8(int(", "int8(int("}[g.r.Intn(2)] + g.nonConst(2) + "))"
		case 2:
			list[i] = "gs[" + g.index() + "]"
		case 3:
			list[i] = []string{"gs", "gb", "g8", "len(gs)"}[g.r.Intn(4)]
		case 4:
			list[i] = g.strExpr(2)
		case 5:
			if k := g.helper("k"); k != "" {
				list[i] = k + "(" + g.expr(1) + ")"
			} else {
				list[i] = g.expr(2)
			}
		default:
			list[i] = g.expr(3)
		}
	}
	return strings.Join(list, ", ")
}

// index returns an index into gs: usually in range (gs has at least 5 bytes).
func (g *progGen) index() string {
	if g.chance(0.03) {
		return g.expr(2) + " & 15"
	}
	return g.expr(2) + " & 3"
}

// strExpr returns a string expression: substrings of gs (whose bounds are usually in
// range) and concatenations, whose operands may call e, which changes gs.
func (g *progGen) strExpr(depth int) string {
	if depth <= 0 || g.chance(0.3) {
		return "gs"
	}
	d := depth - 1
	switch g.r.Intn(5) {
	case 0:
		return "gs[" + g.bound() + ":]"
	case 1:
		return "gs[:" + g.bound() + "]"
	case 2:
		return "gs[" + g.bound() + ":" + g.bound() + "+1]"
	case 3:
		return "(" + g.strExpr(d) + " + " + g.strExpr(d) + ")"
	default:
		return g.strExpr(d) + " + " + g.strExpr(d) + " + \"-\""
	}
}

// rangeStr returns a short string to range over (gs grows, and a loop over all of it in a
// loop that calls e would print without end): a few bytes of gs and some UTF-8, also
// invalid.
func (g *progGen) rangeStr() string {
	switch g.r.Intn(3) {
	case 0:
		return "gs[:" + g.bound() + "] + \"\u00e9\\xff\""
	case 1:
		return "gs[" + g.bound() + ":4]"
	default:
		return "\"\\xe2\\x82\u4e16\" + gs[" + g.bound() + ":" + g.bound() + "+1]"
	}
}

// bound returns a bound of a slice expression: like index, but not a constant, because
// constant bounds out of order (gs[3:1]) don't compile.
func (g *progGen) bound() string {
	if g.chance(0.03) {
		return g.nonConst(2) + " & 15"
	}
	return g.nonConst(2) + " & 3"
}

// divisor returns a divisor: usually one that can't be 0.
func (g *progGen) divisor() string {
	if g.chance(0.02) {
		return g.nonConst(2)
	}
	return "(" + g.nonConst(2) + " | 1)"
}

// nonConst returns an int expression that is not a constant.
func (g *progGen) nonConst(depth int) string {
	for {
		if x := g.expr(depth); !isConstText(x) {
			return x
		}
	}
}

// isConstText reports whether the expression x has no variable and no call: the generator
// writes them all with a letter.
func isConstText(x string) bool {
	return !strings.ContainsAny(x, "abcdefghijklmnopqrstuvwxyz")
}

func (g *progGen) leaf() string {
	switch g.r.Intn(6) {
	case 0:
		return strconv.Itoa(g.r.Intn(10))
	case 1, 2:
		return g.local()
	case 3:
		return []string{"g0", "g1", "len(gs)", "int(g8)"}[g.r.Intn(4)]
	default:
		return fmt.Sprintf("e(%d, %s)", g.nextTag(), strconv.Itoa(g.r.Intn(7)-2))
	}
}

// expr returns an int expression.
func (g *progGen) expr(depth int) string {
	if depth <= 0 || g.chance(0.3) {
		return g.leaf()
	}
	d := depth - 1
	switch g.r.Intn(12) {
	case 0, 1:
		return "(" + g.expr(d) + " + " + g.expr(d) + ")"
	case 2:
		return "(" + g.expr(d) + " - " + g.expr(d) + ")"
	case 3:
		return "(" + g.expr(d) + " * " + g.expr(d) + ")"
	case 4:
		return "(" + g.expr(d) + " / " + g.divisor() + ")"
	case 5:
		return "(" + g.expr(d) + " % " + g.divisor() + ")"
	case 6:
		if g.chance(0.03) {
			return "(" + g.expr(d) + " >> (" + g.nonConst(1) + " - 2))" // may be negative
		}
		if g.chance(0.2) {
			// An untyped count that is not a constant: a uint, never negative (typeOf).
			return "(" + g.expr(d) + " >> (1<<(" + g.nonConst(1) + " & 127) - 2))"
		}
		return "(" + g.expr(d) + " << (uint(int(" + g.nonConst(1) + ")) & 7))"
	case 7:
		return fmt.Sprintf("e(%d, %s)", g.nextTag(), g.expr(d))
	case 8:
		if k := g.helper("k"); k != "" {
			return k + "(" + g.expr(d) + ")"
		}
		return g.leaf()
	case 9:
		if g.chance(0.3) {
			return "len(" + g.strExpr(d) + ")"
		}
		return "int(gs[" + g.index() + "])"
	case 10:
		return fmt.Sprintf("both(%s, %s)", g.expr(d), g.expr(d))
	default:
		return "-" + g.leaf()
	}
}

// cond returns a bool expression.
func (g *progGen) cond(depth int) string {
	if depth <= 0 || g.chance(0.2) {
		switch g.r.Intn(3) {
		case 0:
			return "gb"
		case 1:
			return fmt.Sprintf("eb(%d, %s)", g.nextTag(), g.compare(0, "<"))
		}
		return g.compare(0, "<")
	}
	d := depth - 1
	switch g.r.Intn(6) {
	case 0:
		return "(" + g.cond(d) + " && " + g.cond(d) + ")"
	case 1:
		return "(" + g.cond(d) + " || " + g.cond(d) + ")"
	case 2:
		return "!(" + g.cond(d) + ")"
	case 3:
		return fmt.Sprintf("eb(%d, %s)", g.nextTag(), g.cond(d))
	case 4:
		return g.compare(d, "==")
	}
	return g.compare(d, "<")
}

// compare returns x op y, where x is not a constant (so a case list has no duplicate
// constants).
func (g *progGen) compare(depth int, op string) string {
	return g.nonConst(depth) + " " + op + " " + g.expr(depth)
}
