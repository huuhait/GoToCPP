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
// concatenations and range over strings, Phase 03), pointers (step 3.3): reads and writes
// through pointers to package variables and to boxed locals (*pg, *pl, *lp, *pick(...)), which
// the calls change, and the per-iteration variables of loops whose address is taken, and slices
// (step 3.5): element reads and writes (also op=, ++ and parallel assignments) of a package
// slice that the calls change, reslice and append to (in place or into a new array), of a
// slice that shares its array, and of a local slice, appends, sub-slices and copy in any place,
// range over slices, and a pointer into a slice's array (&gsl[i]). Every append's effect on the
// arrays is the same whatever the growth (in place into a slice of len 4 and cap >= 5, or a new
// array from a full one), so the output never depends on the capacity, and arrays (step 3.6):
// element reads and writes of a package array that the calls change (also through a slice of
// it and a pointer into it), of a boxed local array that the calls change through a pointer,
// assignments of whole arrays (also with a store into the new array in the same assignment),
// range over arrays (a copy when the body could change the array, or through a pointer: no
// copy), &a[i] and a[lo:hi] that share the array, and == of arrays, and maps (step 3.7): reads,
// stores, op=, ++, v, ok and delete of a package map that the calls change (they store, delete
// and make a new map), of a local map that shares its table with a package map, and of a map of
// slices (m[k] = append(m[k], x)), map literals with constant and other keys that can be equal,
// and range over maps; no output depends on the order of a map's range (gocpp's isn't Go's: D8).
// It compares what each prints with real Go, on both compilers.
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
	helpers int      // the helpers h0, k0, ... that the code may call (only earlier ones: no recursion)
	ptrs    []string // the *int variables in scope that may be read and written through
	main    bool     // in main, whose local slice ls and local array la are in scope
}

const randomPrelude = `package main

import "fmt"

var g0, g1 = 1, 2
var gs = "abcde"
var gb = true
var g8 uint8 = 200

// pg points to g0 or g1, pl to a boxed local of main, pc to a loop's variable (or g1).
var pg = &g0
var pl, pc *int

// e prints its tag and changes the package variables, also through the pointers, and where pg
// points.
func e(tag, v int) int {
	fmt.Print("e", tag, " ")
	g0 += tag
	g1 = g1*3 + v
	gb = !gb
	g8 += uint8(tag)
	if tag%3 == 0 {
		gs += "x"
	}
	if tag%4 == 1 {
		if pg == &g0 {
			pg = &g1
		} else {
			pg = &g0
		}
	}
	if pl != nil {
		*pl += tag
	}
	reshape(tag)
	rearray(tag)
	remap(tag)
	return v
}

// gsl always has 5 elements, gal 2 of its array (or of an older one), and psl points into an
// array of gsl. e changes them: appends in place (gsl[:4] has room) and into a new array
// (gsl[:4:4] has none), so what the program prints doesn't depend on how a slice grows.
var gsl = []int{1, 2, 3, 4, 5}
var gal = gsl[1:3]
var psl = &gsl[2]

func reshape(tag int) {
	switch tag % 7 {
	case 2:
		gsl = append(gsl[:4], tag)
	case 3:
		gsl[tag&3] += tag
	case 4:
		gsl = append([]int(nil), gsl...)
	case 5:
		gal = gsl[tag&1 : 2+tag&1]
	case 6:
		gsl = append(gsl[:4:4], -tag)
	}
	if tag%6 == 1 {
		gal[tag&1] -= 1
	}
	if tag%8 == 6 {
		psl = &gsl[tag&3]
	}
}

// gar is a package array, gas a slice of its array (2 elements), and gpa points into it or into
// la, a boxed local array of main, which pla points to. e changes them all.
var gar = [4]int{10, 20, 30, 40}
var gas = gar[1:3]
var gpa = &gar[2]
var pla *[4]int

func rearray(tag int) {
	switch tag % 5 {
	case 1:
		gar[tag&3] += tag
	case 2:
		gas[tag&1] -= tag
	case 3:
		if pla != nil {
			pla[tag&3] *= 2
		}
	case 4:
		gar = [4]int{tag, gar[0], gar[1], gar[2]}
	}
	if tag%9 == 4 {
		gpa = &gar[tag&3]
	}
	if tag%7 == 3 {
		gas = gar[tag&1 : 2+tag&1]
	}
}

// gmp has keys in 0..3; e deletes from it, stores into it and makes a new one. gml shares its
// table with main's local map lm, so a call changes a local map. gms is a map of slices.
var gmp = map[int]int{0: 1, 2: 3}
var gml = map[int]int{1: 1}
var gms = map[int][]int{}

func remap(tag int) {
	switch tag % 5 {
	case 1:
		delete(gmp, tag&3)
	case 2:
		gmp[tag&3] += tag
	case 3:
		gml[tag&3] = tag
	case 4:
		if tag%3 == 0 {
			gmp = map[int]int{tag & 3: tag, 3: -tag}
		} else {
			gms[tag&3] = append(gms[tag&3], tag)
		}
	}
}

// sum prints its tag and returns the sum of s.
func sum(tag int, s []int) int {
	fmt.Print("s", tag, " ")
	t := 0
	for _, v := range s {
		t += v
	}
	return t
}

// pick prints its tag and returns p: a pointer from a call.
func pick(tag int, p *int) *int {
	fmt.Print("p", tag, " ")
	return p
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
		g.paramPtr()
		g.stmts(2)
		g.printf("return %s, %s", g.expr(3), g.expr(3))
		g.indent--
		g.printf("}")
		g.printf("")
		g.printf("func k%d(p int) int {", i)
		g.indent++
		g.locals = []string{"p"}
		g.paramPtr()
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
	// l1 and l2 are boxed: e changes l1 through pl, and lp points to l2. ls is a local slice
	// of 4 elements (also after the appends to ls[:3]); lsp points into an array of ls.
	g.printf("pl = &l1")
	g.printf("lp := &l2")
	g.printf("ls := []int{l0, 7, l1, 8}")
	g.printf("lsp := &ls[1]")
	g.printf("lb := []int{1, 2, 3}") // boxed: psb points to it
	g.printf("psb := &lb")
	// la is a boxed local array, which e changes through pla, lap points to it, and las is a
	// slice of it (2 elements).
	g.printf("la := [4]int{l0, 1, l2, 3}")
	g.printf("pla = &la")
	g.printf("lap := &la")
	g.printf("las := la[1:3]")
	g.ptrs = []string{"pg", "pl", "lp", "psl", "lsp", "gpa"}
	g.printf("lm := gml") // shares gml's table
	g.printf("var mv int")
	g.printf("var mok bool")
	g.main = true
	g.stmts(14)
	g.printf("fmt.Println(l0, l1, l2, g0, g1, gs, gb, g8, *pg, *lp)")
	g.printf("fmt.Println(gsl[0], gsl[1], gsl[2], gsl[3], gsl[4], gal[0], gal[1], len(gsl), *psl, ls[0], ls[1], ls[2], ls[3], *lsp)")
	g.printf("fmt.Println(len(lb), lb[0], lb[1], lb[2], len(*psb))")
	g.printf("fmt.Println(gar[0], gar[1], gar[2], gar[3], gas[0], gas[1], *gpa, la[0], la[1], la[2], la[3], las[0], las[1], lap[3], gar == la)")
	g.printf("fmt.Println(len(gmp), gmp[0], gmp[1], gmp[2], gmp[3], len(lm), lm[0], lm[1], lm[2], lm[3], mv, mok)")
	g.printf("for k := range 4 {")
	g.printf("\tfmt.Print(len(gms[k]), \":\")")
	g.printf("\tfor _, x := range gms[k] {")
	g.printf("\t\tfmt.Print(\" \", x)")
	g.printf("\t}")
	g.printf("\tfmt.Println()")
	g.printf("}")
	g.printf("if pc != nil {")
	g.printf("\tfmt.Println(*pc)")
	g.printf("}")
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

// paramPtr starts a helper: pg and pl, and sometimes a pointer to its parameter p, which boxes
// it.
func (g *progGen) paramPtr() {
	g.ptrs = []string{"pg", "pl", "psl", "gpa"}
	if g.chance(0.4) {
		g.printf("pp := &p")
		g.printf("*pp += %d", g.r.Intn(3))
		g.ptrs = append(g.ptrs, "pp")
	}
}

// ptr returns a pointer to read or write through: a variable, or sometimes a call of pick.
func (g *progGen) ptr() string {
	p := g.ptrs[g.r.Intn(len(g.ptrs))]
	if g.chance(0.2) {
		return fmt.Sprintf("pick(%d, %s)", g.nextTag(), p)
	}
	return p
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
	if g.chance(0.2) {
		return "*" + g.ptr()
	}
	if g.chance(0.2) {
		return g.elem()
	}
	if g.chance(0.15) {
		return g.mapElem()
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

// elem returns an element of a slice or an array to read or write: of gsl, gal, gar, gas, or
// (in main) ls, la, lap and las.
func (g *progGen) elem() string {
	switch k := g.r.Intn(9); {
	case k == 0:
		return "gal[" + g.expr(1) + " & 1]"
	case k == 1 && g.main:
		return "ls[" + g.expr(1) + " & 3]"
	case k == 2:
		return "gar[" + g.expr(1) + " & 3]"
	case k == 3:
		return "gas[" + g.expr(1) + " & 1]"
	case k == 4 && g.main:
		return "la[" + g.expr(1) + " & 3]"
	case k == 5 && g.main:
		return "lap[" + g.expr(1) + " & 3]"
	case k == 6 && g.main:
		return "las[" + g.expr(1) + " & 1]"
	}
	return "gsl[" + g.expr(1) + " & 3]"
}

// array returns an array of 4 ints that the calls can change: gar or (in main) la or *lap.
func (g *progGen) array() string {
	if g.main {
		return []string{"gar", "la", "*lap"}[g.r.Intn(3)]
	}
	return "gar"
}

// arrayStmt writes a statement with arrays: a range over an array (with a value: a copy of
// it, unless it is through a pointer), an assignment of a whole array (also with a store into
// the new array in the same assignment, which Go makes after the array's assignment), a new
// pointer into an array, a new slice of one, and a copy into one.
func (g *progGen) arrayStmt() {
	switch g.r.Intn(7) {
	case 0, 1:
		x := g.array()
		if g.main && g.chance(0.3) {
			x = "lap" // through the pointer: no copy, the body's changes are seen
		}
		i := g.newVar()
		g.fixed[i] = true
		if g.chance(0.3) {
			g.printf("for %s := range %s {", i, x)
		} else {
			g.printf("for %s, v := range %s {", i, x)
			g.indent++
			g.printf("fmt.Println(%q, v, %s)", i, g.expr(2))
			g.indent--
		}
		g.loops++
		g.indent++
		g.printf("fmt.Println(%q, %s)", i, i)
		g.indent--
		g.block(1)
		g.loops--
		g.printf("}")
		g.locals = g.locals[:len(g.locals)-1]
	case 2:
		a := g.array()
		if a == "*lap" {
			a = "la"
		}
		if g.chance(0.5) {
			g.printf("%s, %s[%s & 3] = %s, %s", a, a, g.expr(1), g.array(), g.expr(2))
		} else {
			g.printf("%s = [4]int{%s, %s, 3, %s}", a, g.expr(2), g.expr(1), g.expr(1))
		}
	case 3:
		a := g.array()
		if a == "*lap" {
			a = "lap"
		}
		g.printf("gpa = &%s[%s & 3]", a, g.expr(1))
	case 4:
		if g.main && g.chance(0.5) {
			g.printf("las = la[%s & 1 : 2+%s&1]", g.expr(1), g.expr(1))
		} else {
			g.printf("gas = gar[%s & 1 : 2+%s&1]", g.expr(1), g.expr(1))
		}
	case 5:
		g.printf("_ = copy(gar[%s & 3:], gas)", g.expr(1))
	default:
		g.printf("fmt.Println(%s == %s, %s)", g.array(), g.array(), g.expr(2))
	}
}

// mapElem returns an entry of a map to read or write: of gmp, or (in main) of lm, whose keys are
// in 0..3.
func (g *progGen) mapElem() string {
	if g.main && g.chance(0.3) {
		return "lm[" + g.expr(1) + " & 3]"
	}
	return "gmp[" + g.expr(1) + " & 3]"
}

// mapStmt writes a statement on maps: delete, v, ok, an append to a map of slices (with the same
// key on both sides, which takes the entry once), a new map from a literal whose
// keys can be equal (the constant entries are stored first), ++, clear, and range over a copy of
// gmp (a copy, so that the calls in the body can't change the map; and no break or continue,
// so the result doesn't depend on the order).
func (g *progGen) mapStmt() {
	switch g.r.Intn(8) {
	case 0:
		g.printf("delete(gmp, %s & 3)", g.expr(2))
	case 1:
		if g.main && g.chance(0.5) {
			g.printf("mv, mok = %s", g.mapElem())
			break
		}
		g.nvars++
		v, ok := "mv"+strconv.Itoa(g.nvars), "mok"+strconv.Itoa(g.nvars)
		g.printf("%s, %s := %s", v, ok, g.mapElem())
		g.printf("fmt.Println(%s, %s)", v, ok)
	case 2:
		g.nvars++
		k := "mk" + strconv.Itoa(g.nvars)
		g.printf("%s := %s & 3", k, g.expr(1))
		if g.chance(0.4) {
			// The same key that is not a name: gc evaluates it before the other arguments.
			g.printf("gms[(%s+1)&3] = append(gms[(%s+1)&3], %s)", k, k, g.expr(2))
			break
		}
		g.printf("gms[%s] = append(gms[%s], %s)", k, k, g.expr(2))
	case 3:
		g.printf("gms[2] = append(gms[2], %s, %s)", g.expr(2), g.expr(1))
	case 4:
		g.printf("gmp = map[int]int{%s & 3: %s, 3: %s, 1: 7}", g.nonConst(1), g.expr(2), g.expr(1))
	case 5:
		g.printf("%s++", g.mapElem())
	case 6:
		if g.chance(0.2) {
			g.printf("clear(gml)")
		} else {
			g.printf("gml[%s & 3] = len(gml)", g.expr(1))
		}
	default:
		g.nvars++
		cp, sum := "cp"+strconv.Itoa(g.nvars), "ms"+strconv.Itoa(g.nvars)
		g.printf("%s := map[int]int{}", cp)
		g.printf("for k, v := range gmp {")
		g.printf("\t%s[k] = v", cp)
		g.printf("}")
		g.printf("%s := 0", sum)
		g.printf("for k, v := range %s {", cp)
		g.indent++
		g.printf("%s += k*10 + v", sum)
		g.indent--
		loops := g.loops
		g.loops = 0
		g.block(1)
		g.loops = loops
		g.printf("}")
		g.printf("fmt.Println(%q, %s, len(%s))", sum, sum, cp)
	}
}

// sliceStmt writes a statement that changes a slice: an append (in place or into a new array,
// also with a store into the old array in the same assignment), a copy, a range over a slice,
// and the forms that move the slice into the call (s = append(s, x), s = s[a:]), which keep
// the lengths: a slice is cut by one element, then appended to in place.
func (g *progGen) sliceStmt() {
	if g.chance(0.3) {
		switch k := g.r.Intn(4); {
		case k == 0:
			g.printf("gsl = gsl[:4]")
			g.printf("gsl = append(gsl, %s)", g.expr(2))
		case k == 1 && g.main:
			g.printf("ls = ls[%s & 0:]", g.expr(1))
			g.printf("ls = ls[:3]")
			g.printf("ls = append(ls, %s)", g.expr(2))
		case k == 2 && g.main:
			g.printf("*psb = (*psb)[:2]")
			g.printf("*psb = append(*psb, %s)", g.expr(2))
		default:
			g.printf("gal = gal[:1]")
			g.printf("gal = append(gal, %s)", g.expr(2))
		}
		return
	}
	switch g.r.Intn(6) {
	case 0:
		g.printf("gsl = append(gsl[:4], %s)", g.expr(2))
	case 1:
		g.printf("gsl = append(gsl[:3:3], %s, %s)", g.expr(2), g.expr(2))
	case 2:
		// The store goes into the old array of gsl (Go evaluates gsl before the stores).
		g.printf("gsl, %s = append(gsl[:4:4], %s), %s", g.elem(), g.expr(1), g.expr(2))
	case 3:
		if g.main {
			g.printf("ls = append(ls[:3], %s)", g.expr(2))
			if g.chance(0.3) {
				g.printf("lsp = &ls[%s & 3]", g.expr(1))
			}
			break
		}
		fallthrough
	case 4:
		g.printf("_ = copy(gsl[%s & 3:], gal)", g.expr(1))
	default:
		x := g.sliceExpr(1) // before i is in scope
		i := g.newVar()
		g.fixed[i] = true
		g.printf("for %s, v := range %s {", i, x)
		g.loops++
		g.indent++
		g.printf("fmt.Println(%q, %s, v, %s)", i, i, g.expr(2))
		g.indent--
		g.block(1)
		g.loops--
		g.printf("}")
		g.locals = g.locals[:len(g.locals)-1]
	}
}

// sliceExpr returns a sub-slice of gsl (or gal), with bounds in range: gsl[a:b] with
// a <= 1 < 2 <= b <= 5.
func (g *progGen) sliceExpr(depth int) string {
	switch g.r.Intn(4) {
	case 0:
		return "gal"
	case 1:
		return "gsl[" + g.expr(depth) + " & 1:]"
	case 2:
		return "gsl[:2+" + g.expr(depth) + "&3]"
	}
	return "gsl[" + g.expr(depth) + " & 1 : 2+" + g.expr(depth) + "&3]"
}

func (g *progGen) stmt() {
	k := g.r.Intn(22)
	if k >= 18 {
		if k >= 20 {
			g.mapStmt()
		} else {
			g.arrayStmt()
		}
		return
	}
	if k >= 16 {
		g.sliceStmt()
		return
	}
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
		if g.chance(0.15) {
			// The pointer changes before the store through it: gc stores through the old one.
			g.printf("pg, *pg = &%s, %s", []string{"g0", "g1"}[g.r.Intn(2)], g.expr(2))
			break
		}
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
		g.keepAddr(i)
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
		g.keepAddr(c)
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
		g.keepAddr(c)
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
		case 6:
			// An element of a []int is addressable: read after the calls.
			list[i] = g.elem()
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
		if g.chance(0.4) {
			return "*" + g.ptr()
		}
		if g.chance(0.3) {
			return g.elem()
		}
		if g.chance(0.25) {
			return g.mapElem()
		}
		return []string{"g0", "g1", "len(gs)", "int(g8)", "len(gmp)"}[g.r.Intn(5)]
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
			// An untyped count that is not a constant: a uint, never negative (typeOf). (The
			// int(...) keeps an untyped constant shifted inside it, -6 >> n, an int.)
			return "(" + g.expr(d) + " >> (1<<(int(" + g.nonConst(1) + ") & 127) - 2))"
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
		switch g.r.Intn(4) {
		case 0:
			return fmt.Sprintf("sum(%d, %s)", g.nextTag(), g.sliceExpr(d))
		case 1:
			// In place into the array of gal (which has room after its 2 elements), or into a
			// new array (a full slice).
			if g.chance(0.5) {
				return fmt.Sprintf("len(append(gal, %s))", g.expr(d))
			}
			return fmt.Sprintf("len(append(gsl[%s & 1:3:3], %s))", g.expr(d), g.expr(d))
		case 2:
			if g.chance(0.3) {
				// A literal whose keys can be equal, indexed.
				return fmt.Sprintf("map[int]int{1: %s, %s & 3: %s}[%s & 3]", g.expr(d), g.nonConst(d), g.expr(d), g.expr(d))
			}
			return fmt.Sprintf("copy(gal, %s)", g.sliceExpr(d))
		}
		return fmt.Sprintf("both(%s, %s)", g.expr(d), g.expr(d))
	default:
		return "-" + g.leaf()
	}
}

// keepAddr sometimes keeps the address of the loop variable c in pc (in the loop's body), so
// the end of the program shows the variable of the iteration that set it last (Go 1.22: each
// iteration has its own).
func (g *progGen) keepAddr(c string) {
	if g.chance(0.3) {
		g.indent++
		g.printf("if %s {", g.cond(1))
		g.printf("\tpc = &%s", c)
		g.printf("}")
		g.indent--
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
