package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The order mode of mustgen (-mode ord) writes programs that test Go's order of evaluation around `!`
// calls: the calls that come before a `!` call in the text (calls, method calls, receives, in every
// operand position: arguments, the function value of a call, the receiver, index and slice operands,
// composite literals, both sides of an assignment, conditions and loops) must run before it, and the
// ones after it must run after it, or not at all when it fails.
//
// The twin needs no hoisting of its own, which would only repeat the code under test. It is the same
// text with the `!` taken away: a `!` function there is a plain function that returns its value
// alone and panics with the error when it fails, and run recovers the panic and returns the error.
// Go evaluates the twin in its own order and stops where the panic starts, which is what `!` has to
// do, so the output of the twin (real Go) is the oracle. The calls log what they do and do not change
// variables, because the order of a variable read against a call is not defined by Go.
//
// ¡ stands for ! in the generator's text: the .gp program has `!`, the twin has nothing.

const ordPrelude = `var failAt = -1

type Pt struct {
	id int
	X  int
}

type Cell struct{ A, B int }

type Num int

func sum(id int, args []int) int {
	s := id * 3
	for _, a := range args {
		s += a
	}
	return s % 1000
}

func m8(x int) int { return (x%8 + 8) % 8 }

func b1(x int) int { return x & 1 }

func L(id int, args ...int) int {
	fmt.Println("L", id)
	return sum(id, args)
}

func S(id int) []int {
	fmt.Println("S", id)
	return []int{10, 11, 12, 13, 14, 15, 16, 17}
}

func FN(id int) []func(int) int {
	fmt.Println("FN", id)
	var fs []func(int) int
	for i := 0; i < 8; i++ {
		fs = append(fs, func(x int) int { return x + i })
	}
	return fs
}

func MP(id int) map[int]int {
	fmt.Println("MP", id)
	m := map[int]int{}
	for i := 0; i < 8; i++ {
		m[i] = i * i
	}
	return m
}

func PT(id int) *Pt {
	fmt.Println("PT", id)
	return &Pt{id: id, X: id + 1}
}

func (p *Pt) M(a int) int {
	fmt.Println("M", p.id)
	return p.id + a
}

func PI(id int) *int {
	fmt.Println("PI", id)
	v := id * 2
	return &v
}

func A(id int) any {
	fmt.Println("A", id)
	return id
}

func CH(id int) chan int {
	fmt.Println("CH", id)
	c := make(chan int, 1)
	c <- id + 100
	return c
}

func GEN[T int | int64](a T) T {
	fmt.Println("GEN", a)
	return a + 1
}

func Add2(a, b int) int { return a*10 + b }

func fail(id int) error {
	return fmt.Errorf("fail %d: %w", id, ErrBad)
}

var ErrBad = errors.New("bad")
`

const ordGP = `
func G(id int, args ...int) (int, error) {
	fmt.Println("G", id)
	if id == failAt {
		return 0, fail(id)
	}
	return sum(id, args), nil
}

func GB(id int, x int) (bool, error) {
	fmt.Println("GB", id)
	if id == failAt {
		return false, fail(id)
	}
	return (sum(id, nil)+x)%3 == 0, nil
}

func (p *Pt) GM(id int, a int) (int, error) {
	fmt.Println("GM", id, p.id)
	if id == failAt {
		return 0, fail(id)
	}
	return p.id + a + id, nil
}

func GG[T int | int64](id int, args ...T) (T, error) {
	fmt.Println("GG", id)
	if id == failAt {
		return 0, fail(id)
	}
	var s T
	for _, a := range args {
		s += a
	}
	return s + T(id), nil
}

func Two(id int, a int) (int, int, error) {
	fmt.Println("Two", id)
	if id == failAt {
		return 0, 0, fail(id)
	}
	return a, id, nil
}

func main() {
	for _, fa := range []int{FAILS} {
		failAt = fa
		res, err := run()
		fmt.Println("run", fa, res, err, errors.Is(err, ErrBad))
	}
}
`

const ordTwin = `
type mustFail struct{ err error }

func G(id int, args ...int) int {
	fmt.Println("G", id)
	if id == failAt {
		panic(mustFail{fail(id)})
	}
	return sum(id, args)
}

func GB(id int, x int) bool {
	fmt.Println("GB", id)
	if id == failAt {
		panic(mustFail{fail(id)})
	}
	return (sum(id, nil)+x)%3 == 0
}

func (p *Pt) GM(id int, a int) int {
	fmt.Println("GM", id, p.id)
	if id == failAt {
		panic(mustFail{fail(id)})
	}
	return p.id + a + id
}

func GG[T int | int64](id int, args ...T) T {
	fmt.Println("GG", id)
	if id == failAt {
		panic(mustFail{fail(id)})
	}
	var s T
	for _, a := range args {
		s += a
	}
	return s + T(id)
}

func Two(id int, a int) (int, int) {
	fmt.Println("Two", id)
	if id == failAt {
		panic(mustFail{fail(id)})
	}
	return a, id
}

func main() {
	for _, fa := range []int{FAILS} {
		failAt = fa
		res, err := run()
		fmt.Println("run", fa, res, err, errors.Is(err, ErrBad))
	}
}
`

type og struct {
	rng  *rand.Rand
	id   int
	bang []int
	loop int
}

func (g *og) newID() int { g.id++; return g.id }

func (g *og) bangID() int {
	id := 100 + len(g.bang)
	g.bang = append(g.bang, id)
	return id
}

func (g *og) args(d int, scope []string, max int) string {
	n := g.rng.Intn(max + 1)
	var parts []string
	for i := 0; i < n; i++ {
		parts = append(parts, g.expr(d, scope))
	}
	if len(parts) == 0 {
		return ""
	}
	return ", " + strings.Join(parts, ", ")
}

func (g *og) expr(d int, scope []string) string {
	if d <= 0 || g.rng.Intn(8) == 0 {
		if g.rng.Intn(3) == 0 {
			return fmt.Sprint(g.rng.Intn(10))
		}
		return scope[g.rng.Intn(len(scope))]
	}
	e := func() string { return g.expr(d-1, scope) }
	// idx is an operand that is often a call: the index of S(1)[idx], the function value fns[idx](...)
	idx := func() string {
		if g.rng.Intn(2) == 0 {
			return fmt.Sprintf("L(%d)", g.newID())
		}
		return e()
	}
	switch g.rng.Intn(30) {
	case 26, 27:
		return fmt.Sprintf("FN(%d)[m8(%s)](%s)", g.newID(), idx(), e())
	case 28, 29:
		return fmt.Sprintf("G¡(%d, %s, %s)", g.bangID(), e(), e())
	case 0:
		return fmt.Sprintf("L(%d%s)", g.newID(), g.args(d-1, scope, 3))
	case 1, 2:
		return fmt.Sprintf("G¡(%d%s)", g.bangID(), g.args(d-1, scope, 3))
	case 3:
		return fmt.Sprintf("S(%d)[m8(%s)]", g.newID(), idx())
	case 4:
		return fmt.Sprintf("FN(%d)[m8(%s)](%s)", g.newID(), idx(), e())
	case 5:
		return fmt.Sprintf("MP(%d)[m8(%s)]", g.newID(), idx())
	case 6:
		return fmt.Sprintf("PT(%d).M(%s)", g.newID(), e())
	case 7:
		return fmt.Sprintf("PT(%d).GM¡(%d, %s)", g.newID(), g.bangID(), e())
	case 8:
		return fmt.Sprintf("(*PI(%d) + %s)", g.newID(), e())
	case 9:
		return fmt.Sprintf("(A(%d).(int) - %s)", g.newID(), e())
	case 10:
		return fmt.Sprintf("(%s + %s)", e(), e())
	case 11:
		return fmt.Sprintf("(%s*3 - %s)", e(), e())
	case 12:
		return fmt.Sprintf("int(int64(%s))", e())
	case 13:
		return fmt.Sprintf("([]int{%s, %s, %s}[1])", e(), e(), e())
	case 14:
		return fmt.Sprintf("(Cell{A: %s, B: %s}.B)", e(), e())
	case 15:
		return fmt.Sprintf("(map[int]int{m8(%s): %s, m8(%s) + 8: %s}[3])", e(), e(), e(), e())
	case 16:
		return fmt.Sprintf("(len(S(%d)) + %s)", g.newID(), e())
	case 17:
		return fmt.Sprintf("(<-CH(%d) + %s)", g.newID(), e())
	case 18:
		return fmt.Sprintf("(func(a, b int) int { return a*2 + b }(%s, %s))", e(), e())
	case 19:
		return fmt.Sprintf("GEN[int](%s)", e())
	case 20:
		return fmt.Sprintf("GG¡[int](%d%s)", g.bangID(), g.args(d-1, scope, 2))
	case 21:
		return fmt.Sprintf("(PT(%d).M)(%s)", g.newID(), e())
	case 22:
		return fmt.Sprintf("append(S(%d), %s)[2]", g.newID(), e())
	case 23:
		return fmt.Sprintf("min(%s, %s)", e(), e())
	case 24:
		return fmt.Sprintf("int(Num(%s))", e())
	case 25:
		return fmt.Sprintf("Add2(Two¡(%d, %s))", g.bangID(), e())
	}
	return fmt.Sprintf("-(%s)", e())
}

func (g *og) cond(d int, scope []string) string {
	if d <= 0 || g.rng.Intn(4) == 0 {
		return fmt.Sprintf("%s < %s", g.expr(d, scope), g.expr(d, scope))
	}
	switch g.rng.Intn(6) {
	case 0:
		return fmt.Sprintf("(%s && %s)", g.cond(d-1, scope), g.cond(d-1, scope))
	case 1:
		return fmt.Sprintf("(%s || %s)", g.cond(d-1, scope), g.cond(d-1, scope))
	case 2:
		return fmt.Sprintf("!(%s)", g.cond(d-1, scope))
	case 3:
		return fmt.Sprintf("GB¡(%d, %s)", g.bangID(), g.expr(d-1, scope))
	case 4:
		return fmt.Sprintf("(L(%d) > %s)", g.newID(), g.expr(d-1, scope))
	}
	return fmt.Sprintf("(%s == %s)", g.expr(d-1, scope), g.expr(d-1, scope))
}

func (g *og) block(b *strings.Builder, ind int, depth int, scope []string) {
	n := 1 + g.rng.Intn(3)
	scope = append([]string(nil), scope...)
	for i := 0; i < n; i++ {
		scope = g.stmt(b, ind, depth, scope)
	}
}

func (g *og) stmt(b *strings.Builder, ind int, depth int, scope []string) []string {
	tab := strings.Repeat("\t", ind)
	line := func(f string, a ...any) { fmt.Fprintf(b, tab+f+"\n", a...) }
	vars := []string{"v0", "v1", "v2", "v3"}
	v := func() string { return vars[g.rng.Intn(4)] }
	ed := 2 + g.rng.Intn(2)
	r := g.rng.Intn(20)
	if depth <= 0 && r >= 9 {
		r = g.rng.Intn(9)
	}
	switch r {
	case 0, 1:
		line("%s = %s", v(), g.expr(ed, scope))
	case 2:
		line("%s += %s", v(), g.expr(ed, scope))
	case 3:
		k := g.rng.Intn(4)
		line("v%d, v%d = %s, %s", k, (k+1)%4, g.expr(ed, scope), g.expr(ed, scope))
	case 4:
		line("arr[m8(%s)] = %s", g.expr(ed, scope), g.expr(ed, scope))
	case 5:
		line("arr[m8(%s)] += %s", g.expr(ed, scope), g.expr(ed, scope))
	case 6:
		line("ch <- %s", g.expr(ed, scope))
	case 7:
		line("L(%d%s)", g.newID(), g.args(ed, scope, 3))
	case 8:
		name := fmt.Sprintf("w%d", g.newID())
		line("%s := %s", name, g.expr(ed, scope))
		line("_ = %s", name)
		return append(scope, name)
	case 9, 10, 11:
		n := 1 + g.rng.Intn(3)
		for i := 0; i < n; i++ {
			kw := "if"
			if i > 0 {
				kw = "} else if"
			}
			line("%s %s {", kw, g.cond(2, scope))
			g.block(b, ind+1, depth-1, scope)
		}
		if g.rng.Intn(2) == 0 {
			line("} else {")
			g.block(b, ind+1, depth-1, scope)
		}
		line("}")
	case 12, 13:
		line("switch %s {", g.expr(2, scope))
		for i := 0; i < 2; i++ {
			line("case m8(%s):", g.expr(2, scope))
			g.block(b, ind+1, depth-1, scope)
		}
		line("default:")
		g.block(b, ind+1, depth-1, scope)
		line("}")
	case 14, 15:
		iv := fmt.Sprintf("i%d", g.newID())
		inner := append(append([]string(nil), scope...), iv)
		c := fmt.Sprintf("%s < 3", iv)
		if g.rng.Intn(3) != 0 {
			c += " && " + g.cond(2, inner)
		}
		post := fmt.Sprintf("%s += 1 + b1(%s)", iv, g.expr(2, inner))
		line("for %s := b1(%s); %s; %s {", iv, g.expr(2, scope), c, post)
		g.loop++
		g.block(b, ind+1, depth-1, inner)
		g.loop--
		line("}")
	case 16:
		line("PT(%d).X = %s", g.newID(), g.expr(ed, scope))
	case 17:
		line("arr[m8(%s)]++", g.expr(ed, scope))
	case 18:
		if g.loop > 0 {
			line("if %s {", g.cond(2, scope))
			line("\tcontinue")
			line("}")
		} else {
			line("if %s {", g.cond(2, scope))
			line("\treturn %s, nil", g.expr(2, scope))
			line("}")
		}
	default:
		line("%s = %s", v(), g.expr(ed, scope))
	}
	return scope
}

// ordProgram returns the .gp and the twin of program i.
func ordProgram(seed int64, i int) (gp, twin string) {
	g := &og{rng: rand.New(rand.NewSource(seed*1000 + int64(i)))}
	var body strings.Builder
	scope := []string{"v0", "v1", "v2", "v3"}
	n := 6 + g.rng.Intn(4)
	for k := 0; k < n; k++ {
		scope = g.stmt(&body, 1, 2, scope)
	}
	fails := []string{"-1"}
	for _, id := range g.bang {
		fails = append(fails, fmt.Sprint(id))
	}
	head := "package main\n\nimport (\n\t\"errors\"\n\t\"fmt\"\n)\n\n" + ordPrelude
	run := func(sig string) string {
		return "\nfunc run" + sig + " {\n\tv0, v1, v2, v3 := 1, 2, 3, 4\n\tvar arr [8]int\n\tch := make(chan int, 64)\n\t_ = ch\n" +
			body.String() +
			"\treturn (v0*7+v1*5+v2*3+v3+arr[0]+arr[3]+len(ch)) % 100003, nil\n}\n"
	}
	mk := func(sig, tail string, bang string) string {
		t := strings.ReplaceAll(head+run(sig)+tail, "FAILS", strings.Join(fails, ", "))
		return strings.ReplaceAll(t, "¡", bang)
	}
	gp = mk("() (int, error)", ordGP, "!")
	twin = mk("() (res int, err error)", ordTwin, "")
	twin = strings.Replace(twin, "\nfunc run() (res int, err error) {\n", "\nfunc run() (res int, err error) {\n\tdefer func() {\n\t\tif r := recover(); r != nil {\n\t\t\tf, ok := r.(mustFail)\n\t\t\tif !ok {\n\t\t\t\tpanic(r)\n\t\t\t}\n\t\t\tres, err = 0, f.err\n\t\t}\n\t}()\n", 1)
	return gp, twin
}

func ordMain(n int, seed int64, dir string) {
	if err := os.MkdirAll(filepath.Join(dir, "testdata", "must_twin"), 0o755); err != nil {
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
	for i := 0; i < n; i++ {
		gp, twin := ordProgram(seed, i)
		name := fmt.Sprintf("must_ord_%02d", i)
		if err := os.WriteFile(filepath.Join(tmp, "main.go"), []byte(twin), 0o644); err != nil {
			fail(err)
		}
		cmd := exec.Command("go", "run", ".")
		cmd.Dir = tmp
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=")
		res, err := cmd.CombinedOutput()
		if err != nil {
			os.WriteFile(filepath.Join(tmp, "..", name+".failed.go"), []byte(twin), 0o644)
			fail(fmt.Errorf("%s: the twin does not run: %v\n%.3000s", name, err, res))
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
		hdr := fmt.Sprintf("// phase: 10\n\n// Generated by tests/mustgen -mode ord (seed %d, program %d): the order of evaluation around ! calls.\n// Its twin in testdata/must_twin/ is the same text without the !.\n", seed, i)
		if err := os.WriteFile(filepath.Join(dir, name+".gp"), []byte(hdr+gp+ob.String()), 0o644); err != nil {
			fail(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "testdata", "must_twin", name+".go"), []byte(twin), 0o644); err != nil {
			fail(err)
		}
	}
}
