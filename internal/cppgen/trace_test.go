package cppgen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestTraceVsGo compares the stack trace of an unrecovered panic with real Go's (Go 1.25's
// form), for a recursion of several depths: the same frames, function names, files and
// lines, and for a stack of more than 100 frames the inner 50, "...N frames elided..." with
// Go's count, and the outer 50 ending with main.main. normTrace leaves out what differs on
// purpose: the folders, the offsets, and what Go prints between the parentheses (argument
// words, or "..." for an inlined frame).
func TestTraceVsGo(t *testing.T) {
	goOnce.Do(func() { goCmd, _ = exec.LookPath("go") })
	if goCmd == "" {
		t.Skip("no go command")
	}
	for _, depth := range []int{3, 98, 99, 200} {
		t.Run(fmt.Sprint(depth), func(t *testing.T) {
			dir := t.TempDir()
			src := fmt.Sprintf("package main\n\nfunc r(n int) {\n\tif n == 0 {\n\t\tpanic(\"deep\")\n\t}\n\tr(n - 1)\n}\n\nfunc main() {\n\tr(%d)\n}\n", depth)
			gp := filepath.Join(dir, "main.gp")
			if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			want := goTrace(t, gp)
			work := t.TempDir()
			cpp := filepath.Join(work, "main.gp.cpp")
			if err := os.WriteFile(cpp, []byte(emitter(gp)(t, Config{OutputPath: cpp})), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, c := range compilers(t) {
				t.Run(c.name, func(t *testing.T) {
					if c.missing != "" {
						t.Skip(c.missing)
					}
					_, stderr, code, err := command(work, false, c.build(t, cpp))
					if err != nil {
						t.Fatal(err)
					}
					if got := normTrace(string(stderr)); code != 2 || got != want {
						t.Errorf("exit code %d, the trace (normalized):\n%s\nwant Go's:\n%s", code, got, want)
					}
				})
			}
		})
	}
}

// TestTraceClosuresVsGo compares the stack traces of panics in func literals with real Go's
// (step 3.11): the closures are named as Go names them (main.main.func1, main.main.func1.1,
// main.init.func1.1 for a literal in a package variable's initializer, main.init.0.func1 in an
// init function, main.T.M.func1 and main.(*T).M.func1 in methods), also through calls of func
// values, deferred closures and a closure called at once. Go's program is built without inlining (-gcflags=-l): Go's inliner copies a closure into
// the function it inlines it in and names the copy after that function (main.main.main.func1.func3),
// which gocpp doesn't do.
func TestTraceClosuresVsGo(t *testing.T) {
	goOnce.Do(func() { goCmd, _ = exec.LookPath("go") })
	if goCmd == "" {
		t.Skip("no go command")
	}
	progs := map[string]string{
		"nested": "package main\n\nfunc each(fs ...func()) {\n\tfor _, f := range fs {\n\t\tf()\n\t}\n}\n\nfunc main() {\n\tok := func() {}\n\t" +
			"outer := func(n int) {\n\t\teach(ok, func() {\n\t\t\tif n > 0 {\n\t\t\t\tpanic(\"deep\")\n\t\t\t}\n\t\t})\n\t}\n\touter(1)\n}\n",
		"pkgvar": "package main\n\nvar f = func() func(int) int {\n\treturn func(n int) int {\n\t\treturn 10 / n\n\t}\n}()\n\nvar g = func() {}\n\n" +
			"func main() {\n\tg()\n\tfunc() {\n\t\tprintln(f(0))\n\t}()\n}\n",
		"deferred": "package main\n\nfunc main() {\n\tdefer func() {\n\t\tpanic(\"in defer\")\n\t}()\n\tvar p *int\n\t_ = func() {}\n\tprintln(*p)\n}\n",
		"methods": "package main\n\ntype T struct{ n int }\n\nfunc (t T) Boom() {\n\tf := func() {\n\t\tpanic(t.n)\n\t}\n\tf()\n}\n\n" +
			"func (t *T) PBoom(xs ...int) {\n\tfunc() {\n\t\tpanic(len(xs))\n\t}()\n}\n\n" +
			"func main() {\n\tt := &T{3}\n\tdefer func() {\n\t\tt.PBoom(1, 2)\n\t}()\n\tt.Boom()\n}\n",
		"init": "package main\n\nvar x int\n\nfunc init() {\n\tfs := []func(){func() { x++ }, func() { panic(x) }}\n\tfor _, f := range fs {\n\t\tf()\n\t}\n}\n\nfunc main() {}\n",
	}
	for name, src := range progs {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			gp := filepath.Join(dir, "main.gp")
			if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			want := panicFrame.ReplaceAllString(goTraceFlags(t, gp, "-gcflags=-l"), "panic()\n")
			work := t.TempDir()
			cpp := filepath.Join(work, "main.gp.cpp")
			if err := os.WriteFile(cpp, []byte(emitter(gp)(t, Config{OutputPath: cpp})), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, c := range compilers(t) {
				t.Run(c.name, func(t *testing.T) {
					if c.missing != "" {
						t.Skip(c.missing)
					}
					_, stderr, code, err := command(work, false, c.build(t, cpp))
					if err != nil {
						t.Fatal(err)
					}
					if got := panicFrame.ReplaceAllString(normTrace(maskPC(result{stderr: string(stderr)}).stderr), "panic()\n"); code != 2 || got != want {
						t.Errorf("exit code %d, the trace (normalized):\n%s\nwant Go's:\n%s", code, got, want)
					}
				})
			}
		})
	}
}

// TestTraceInterfaceVsGo compares the trace of a panic through a call of an interface method with
// Go's (step 5.4): the wrapper method of the interface struct is no frame in Go, so the trace
// leaves it out (as the thunk's), and a call of a nil interface shows the caller only. Go's
// program is built without inlining (-gcflags=-l), which would devirtualize the calls.
func TestTraceInterfaceVsGo(t *testing.T) {
	goOnce.Do(func() { goCmd, _ = exec.LookPath("go") })
	if goCmd == "" {
		t.Skip("no go command")
	}
	progs := map[string]string{
		"value":   "package main\n\ntype I interface{ M(i int) int }\ntype T struct{ a []int }\n\nfunc (t T) M(i int) int {\n\treturn t.a[i]\n}\n\nfunc call(x I) int {\n\treturn x.M(5)\n}\n\nfunc main() {\n\tvar x any = T{[]int{1}}\n\tprintln(call(x.(I)))\n}\n",
		"pointer": "package main\n\ntype I interface{ M() }\ntype T struct{ n *int }\n\nfunc (t *T) M() {\n\tprintln(*t.n)\n}\n\nfunc main() {\n\tvar x I = &T{}\n\tx.M()\n}\n",
		"nil":     "package main\n\ntype I interface{ M() }\n\nfunc main() {\n\tvar x I\n\tx.M()\n}\n",
	}
	for name, src := range progs {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			gp := filepath.Join(dir, "main.gp")
			if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			want := goTraceFlags(t, gp, "-gcflags=-l")
			work := t.TempDir()
			cpp := filepath.Join(work, "main.gp.cpp")
			if err := os.WriteFile(cpp, []byte(emitter(gp)(t, Config{OutputPath: cpp})), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, c := range compilers(t) {
				t.Run(c.name, func(t *testing.T) {
					if c.missing != "" {
						t.Skip(c.missing)
					}
					_, stderr, code, err := command(work, false, c.build(t, cpp))
					if err != nil {
						t.Fatal(err)
					}
					if got := normTrace(maskPC(result{stderr: string(stderr)}).stderr); code != 2 || got != want {
						t.Errorf("exit code %d, the trace (normalized):\n%s\nwant Go's:\n%s", code, got, want)
					}
				})
			}
		})
	}
}

// panicFrame matches the runtime's panic frame of a normalized trace (a deferred call that
// panicked while a panic ran) and its place, which is in Go's runtime (panic:783) and gocpp's
// (?:0).
var panicFrame = regexp.MustCompile(`(?m)^panic\(\)\n\t.*\n`)

// goTrace builds the .gp file gp as a Go program with real Go, runs it, checks that it panics
// and returns its stderr, normalized.
func goTrace(t *testing.T, gp string) string {
	t.Helper()
	return goTraceFlags(t, gp)
}

// goTraceFlags is goTrace with more flags for go build.
func goTraceFlags(t *testing.T, gp string, flags ...string) string {
	t.Helper()
	dir := t.TempDir()
	src, err := os.ReadFile(gp)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "prog")
	args := append(append([]string{"build"}, flags...), "-o", bin, "main.go")
	out, _, code, err := goCommand(dir, args...)
	if err != nil || code != 0 {
		t.Fatalf("go build: exit code %d %v\n%s", code, err, out)
	}
	_, stderr, code, err := command(dir, false, bin)
	if err != nil || code != 2 {
		t.Fatalf("the Go program: exit code %d %v\n%s", code, err, stderr)
	}
	return normTrace(maskPC(result{stderr: string(stderr)}).stderr)
}

// normTrace rewrites the frames of a trace (after the goroutine line) so that Go's and
// gocpp's compare: "name(anything)" becomes "name()", and "\t/dir/main.go:9 +0x25" becomes
// "\tmain:9". The header and the "...N frames elided..." line stay as they are.
func normTrace(stderr string) string {
	i := strings.Index(stderr, goStackMark)
	if i < 0 {
		return stderr
	}
	var b strings.Builder
	b.WriteString(stderr[:i+len(goStackMark)])
	for _, line := range strings.Split(strings.TrimSuffix(stderr[i+len(goStackMark):], "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "\t"):
			loc := filepath.Base(strings.Fields(line)[0])
			file, num, _ := strings.Cut(loc, ":")
			file = strings.TrimSuffix(strings.TrimSuffix(file, ".gp"), ".go")
			fmt.Fprintf(&b, "\t%s:%s\n", file, num)
		case strings.HasPrefix(line, "..."):
			b.WriteString(line + "\n")
		default:
			if j := strings.LastIndex(line, "("); j >= 0 {
				line = line[:j] // the arguments (not the ( of main.(*T).M)
			}
			b.WriteString(line + "()\n")
		}
	}
	return b.String()
}

// TestTraceMethods compares the trace of a panic inside methods with Go's (step 3.10): a value
// method is main.T.M, a pointer method main.(*T).M, also for a named type that isn't a struct,
// and a method called through a deferred call shows at the line of its function's exit. Method
// values and expressions (step 3.10b) show the method itself: Go leaves out its wrappers
// (main.T.M-fm) as the trace leaves out gocpp's generic lambdas.
func TestTraceMethods(t *testing.T) {
	goOnce.Do(func() { goCmd, _ = exec.LookPath("go") })
	if goCmd == "" {
		t.Skip("no go command")
	}
	tests := []struct {
		name, src string
		frames    []string // in Go's trace
	}{
		{"calls", `package main

type T struct{ v int }

func (t T) Div(d int) int {
	return t.v / d
}

func (t *T) Half(d int) int {
	return T{t.v}.Div(d - 2)
}

type N int

func (n N) Run(t *T) int {
	return t.Half(int(n))
}

func (n *N) Defer(t *T) {
	defer n.Run(t)
}

func main() {
	n := N(2)
	n.Defer(&T{4})
}
`, []string{"main.T.Div()", "main.(*T).Half()", "main.N.Run()", "main.(*N).Defer()"}},
		{"values", `package main

type T struct{ v int }

func (t T) Div(d int) int {
	return t.v / d
}

func (t *T) Half(d int) int {
	return call(T{t.v}.Div, d-2)
}

func call(f func(int) int, d int) int {
	return f(d)
}

func main() {
	t := &T{4}
	h := t.Half
	e := (*T).Half
	println(e(t, 3))
	h(2)
}
`, []string{"main.T.Div()", "main.call()", "main.(*T).Half()", "main.main()"}},
		{"expressions", `package main

type T struct{ v int }

func (t T) Div(d int) int {
	return t.v / d
}

func main() {
	f := T.Div
	println(f(T{1}, 1))
	println(T.Div(T{1}, 0))
}
`, []string{"main.T.Div()", "main.main()"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			gp := filepath.Join(dir, "main.gp")
			if err := os.WriteFile(gp, []byte(tt.src), 0o644); err != nil {
				t.Fatal(err)
			}
			want := goTrace(t, gp)
			for _, name := range tt.frames {
				if !strings.Contains(want, name+"\n") {
					t.Fatalf("Go's trace has no %s:\n%s", name, want)
				}
			}
			work := t.TempDir()
			cpp := filepath.Join(work, "main.gp.cpp")
			if err := os.WriteFile(cpp, []byte(emitter(gp)(t, Config{OutputPath: cpp})), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, c := range compilers(t) {
				t.Run(c.name, func(t *testing.T) {
					if c.missing != "" {
						t.Skip(c.missing)
					}
					_, stderr, code, err := command(work, false, c.build(t, cpp))
					if err != nil {
						t.Fatal(err)
					}
					if got := normTrace(string(stderr)); code != 2 || got != want {
						t.Errorf("exit code %d, the trace (normalized):\n%s\nwant Go's:\n%s", code, got, want)
					}
				})
			}
		})
	}
}
