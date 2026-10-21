package lower

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

func threadsOf(t *testing.T, src string, library bool) (*gen, *threadInfo) {
	t.Helper()
	gp := filepath.Join(t.TempDir(), "main.gp")
	if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	fset, pkg := checkFileIgnoring(t, gp, "missing function body") // the natives of the cases
	g := newGen(fset, pkg)
	g.opts.Library = library
	return g, g.threads()
}

// TestThreadGate: a program is threaded when its own code has a go statement or uses a thread
// source (threadSources); a program without either never needs the copies of hotLoad.
func TestThreadGate(t *testing.T) {
	const head = "package main\n\nimport (\n\t\"context\"\n\t\"runtime\"\n\t\"sync\"\n\t\"time\"\n)\n\nvar _ = context.Background\nvar _ runtime.MemStats\nvar _ sync.Mutex\nvar _ = time.Now\n\n"
	for _, tc := range []struct {
		name, body string
		library    bool
		want       bool
	}{
		{"none", "func main() {}\n", false, false},
		{"plain calls", "func f() int { return 1 }\nfunc main() { f() }\n", false, false},
		{"go statement", "func main() { go func() {}() }\n", false, true},
		{"go in an unused function", "func unused() { go func() {}() }\nfunc main() {}\n", false, true},
		{"waitgroup go", "func main() { var wg sync.WaitGroup; wg.Go(func() {}) }\n", false, true},
		{"afterfunc", "func main() { time.AfterFunc(time.Second, func() {}) }\n", false, true},
		{"finalizer", "func main() { x := new(int); runtime.SetFinalizer(x, func(*int) {}) }\n", false, true},
		{"context afterfunc", "func main() { context.AfterFunc(context.Background(), func() {}) }\n", false, true},
		{"func value", "var f func()\nfunc main() { f() }\n", false, true},
		{"interface call", "type I interface { Run() }\nfunc main() { var i I; i.Run() }\n", false, true},
		{"native callback", "type Callback func()\nfunc native(f Callback)\nfunc main() { native(nil) }\n", false, true},
		{"native interface callback", "func native(f any)\nfunc main() { native(nil) }\n", false, true},
		{"native variadic callback", "func native(f ...any)\nfunc main() { native(1) }\n", false, true},
		{"native nested callback", "type Box struct { f func() }\nfunc native(f *Box)\nfunc main() { native(nil) }\n", false, true},
		{"iterator", "var seq func(func(int) bool)\nfunc main() { for range seq {} }\n", false, true},
		{"a library", "func main() {}\n", true, true},
	} {
		_, th := threadsOf(t, head+tc.body, tc.library)
		if th.threaded != tc.want {
			t.Errorf("%s: threaded = %v, want %v", tc.name, th.threaded, tc.want)
		}
	}
}

// TestBorrowHotFixtureThreadFacts isolates every runtime fixture's startup mechanism. No
// formatting, mutex, timing or unused starter helper can supply a thread fact accidentally.
func TestBorrowHotFixtureThreadFacts(t *testing.T) {
	tests := []struct {
		name        string
		imports     string
		body        string
		initThreads bool
		hotGlobal   bool
	}{
		{"borrow_hotswap", "", "func main() { go worker() }", false, false},
		{"borrow_hot_field", "", "func main() { go worker() }", false, false},
		{"borrow_hot_heapfield", "", "func main() { go worker() }", false, false},
		{"borrow_hot_elem", "", "func main() { go worker() }", false, false},
		{"borrow_hot_recv", "", "func main() { go worker() }", false, false},
		{"borrow_hot_afterfunc", "import \"time\"",
			`func main() { _ = time.AfterFunc }`, false, false},
		{"borrow_hot_ctx_afterfunc", "import \"context\"",
			`func main() { _ = context.AfterFunc }`, false, false},
		{"borrow_hot_decl_go", "", `func start() bool {
			go worker()
			return true
		}
		var started = start()
		func main() { _ = started }`, true, false},
		{"borrow_hot_fmt_string", "", `type starter struct{}
		func (starter) String() string {
			go worker()
			return "started"
		}
		func main() {}`, false, false},
		{"borrow_hot_iface_go", "import \"sync\"", `type Goer interface{ Go(func()) }
		func main() {
			var group sync.WaitGroup
			var starter Goer = &group
			starter.Go(worker)
		}`, false, false},
		{"borrow_hot_once_do", "import \"sync\"", `var once sync.Once
		type sorter []int
		func (s sorter) Less(i, j int) bool {
			once.Do(func() { go worker() })
			return i < j
		}
		func main() {}`, false, false},
		{"borrow_hot_funcvalue_start", "", `var startFn = worker
		func main() { startFn() }`, false, false},
		{"borrow_hot_waitgroup_go", "import \"sync\"", `func main() {
			var group sync.WaitGroup
			group.Go(worker)
		}`, false, false},
		{"borrow_hot_waitgroup_go_lit", "import \"sync\"", `func main() {
			var group sync.WaitGroup
			group.Go(func() { worker() })
		}`, false, false},
		{"borrow_hot_closure_main", "", `func main() {
			swap := func() { cfg = nil }
			_ = swap
			go worker()
		}`, false, true},
		{"borrow_hot_closure_init", "", `var reload func()
		func init() { reload = func() { cfg = nil } }
		func main() { go worker() }`, false, true},
		{"borrow_hot_init_go", "", `func init() { go worker() }
		func main() {}`, true, false},
		{"borrow_hot_init_indirect", "", `func start() { go worker() }
		func init() {
			start()
			cfg = &Config{}
		}
		func main() {}`, true, true},
		{"borrow_hot_init_go_swap", "", `func init() {
			go worker()
			cfg = &Config{}
		}
		func main() {}`, true, true},
	}
	covered := make(map[string]bool, len(tests))
	for _, tt := range tests {
		covered[tt.name] = true
		t.Run(tt.name, func(t *testing.T) {
			source := "package main\n" + tt.imports + `
			type Config struct{ k int }
			var cfg *Config
			func worker() {}
			` + tt.body
			g, facts := threadsOf(t, source, false)
			if !facts.threaded || facts.initThreads != tt.initThreads {
				t.Fatalf("threaded/initThreads = %v/%v, want true/%v",
					facts.threaded, facts.initThreads, tt.initThreads)
			}
			cfg := g.pkg.Types.Scope().Lookup("cfg").(*types.Var)
			if facts.mutGlobals[cfg] != tt.hotGlobal {
				t.Fatalf("hot cfg = %v, want %v", facts.mutGlobals[cfg], tt.hotGlobal)
			}
		})
	}

	// Keep the runtime set and its isolated analysis coverage in one-to-one correspondence.
	paths, err := filepath.Glob(filepath.Join(repoRootDir(t), "tests", "run", "borrow_hot*.gp"))
	if err != nil || len(paths) != len(covered) {
		t.Fatalf("runtime fixture coverage: %d paths, %d cases, error %v",
			len(paths), len(covered), err)
	}
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".gp")
		if !covered[name] {
			t.Errorf("runtime fixture %s has no isolated thread-fact case", name)
		}
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(source), "// race: intentional") {
			t.Errorf("positive fixture %s must not contain an intentional race", name)
		}
	}
}

// TestMutatedFacts: what the program assigns (package variables, struct fields, element types)
// and the exemption of init functions. A name ending in _hot is recorded, _cold is not.
func TestMutatedFacts(t *testing.T) {
	const src = `package main

type T struct{ n int }
type S struct {
	p     *T
	arr   [2]*T
	hot_f *T
	cold_f *T
}
type W struct{ in S }

func (s *S) set(p *T) { s.hot_f = p }

var inInit_cold, closureInInit_hot, viaMain_hot *T
var field_hot, fieldNever_cold S
var addr_hot *T
var addrFieldS_hot S
var method_hot S
var arr_hot [2]*T
var sl_hot [2]*T
var whole_hot, wholeNever_cold S
var wrap_hot W
var slice_cold []*T
var mapv_cold map[string]*T
var rng_hot int
var inc_hot int

func init() {
	inInit_cold = &T{}
	f := func() { closureInInit_hot = &T{} }
	_ = f // declaring a closure is not a proof that its body runs only during init
}

func worker() {}

func main() {
	go worker()
	viaMain_hot = &T{}
	inc_hot++
	for rng_hot = range 3 {
	}
	field_hot.p = &T{}
	_ = &addr_hot
	_ = &addrFieldS_hot.p
	method_hot.set(&T{})
	arr_hot = [2]*T{}
	_ = sl_hot[:]
	whole_hot = S{}
	wrap_hot.in.arr[1] = &T{}
	var local S
	local.cold_f = nil
	_ = fieldNever_cold
	_ = wholeNever_cold
	_ = slice_cold
	_ = mapv_cold
}
`
	g, th := threadsOf(t, src, false)
	if !th.threaded {
		t.Fatal("the program has a go statement")
	}
	scope := g.pkg.Types.Scope()
	for _, name := range scope.Names() {
		v, ok := scope.Lookup(name).(*types.Var)
		if !ok {
			continue
		}
		want := strings.HasSuffix(name, "_hot")
		if got := g.mutatedGlobal(v); got != want {
			t.Errorf("mutatedGlobal(%s) = %v, want %v", name, got, want)
		}
	}
	st := scope.Lookup("S").Type().Underlying().(*types.Struct)
	if !th.mutFields[st.Field(0)] || !th.mutFields[st.Field(2)] {
		t.Errorf("fields p and hot_f of S must be recorded as assigned")
	}
	found := false
	for k := range th.mutElems {
		found = found || strings.HasSuffix(k, ".T") && strings.HasPrefix(k, "*")
	}
	if !found {
		t.Errorf("the element type *T is stored (wrap_hot.in.arr[1] = ...): %v", th.mutElems)
	}
}

// TestInitExemption: an init function that starts a thread (also through a function) takes the
// exemption of every init away; a program without such an init keeps it.
func TestInitExemption(t *testing.T) {
	cases := []struct {
		name, src string
		exempt    bool
	}{
		{"quiet init", "func init() { a = &T{} }\nfunc main() { go func() {}() }\n", true},
		{"init with go", "func init() { a = &T{}; go func() {}() }\nfunc main() {}\n", false},
		{"init calls a spawner", "func spawn() { go func() {}() }\nfunc init() { a = &T{}; spawn() }\nfunc main() {}\n", false},
		{"another init spawns", "func init() { a = &T{} }\nfunc init() { go func() {}() }\nfunc main() {}\n", false},
		{"initializer spawns", "var _ = func() int { go func() {}(); return 0 }()\nfunc init() { a = &T{} }\nfunc main() {}\n", false},
		{"init calls func value", "var f func()\nfunc init() { f(); a = &T{} }\nfunc main() {}\n", false},
		{"init calls interface", "type I interface { Run() }\nvar i I\nfunc init() { i.Run(); a = &T{} }\nfunc main() {}\n", false},
		{"init native callback", "func native(f func())\nfunc init() { native(nil); a = &T{} }\nfunc main() {}\n", false},
	}
	for _, tc := range cases {
		g, th := threadsOf(t, "package main\n\ntype T struct{ n int }\n\nvar a *T\n\n"+tc.src, false)
		a := g.pkg.Types.Scope().Lookup("a").(*types.Var)
		if !th.threaded {
			t.Errorf("%s: the program must be threaded", tc.name)
			continue
		}
		if got := !g.mutatedGlobal(a); got != tc.exempt {
			t.Errorf("%s: init assignment exempt = %v, want %v", tc.name, got, tc.exempt)
		}
	}
	// fmt has no lowered body here and ...any hides String/Error/Format callbacks. An init
	// using it cannot establish a thread-free proof merely because no named spawner is seen.
	gfmt, thfmt := threadsOf(t, "package main\n\nimport \"fmt\"\nvar a *int\nfunc init() { fmt.Println(1); a = new(int) }\nfunc main() {}\n", false)
	if !thfmt.threaded || !thfmt.initThreads || !gfmt.mutatedGlobal(gfmt.pkg.Types.Scope().Lookup("a").(*types.Var)) {
		t.Error("a callback-bearing fmt call must disable the init exemption")
	}
	// a library has no exemption (foreign threads can call in before main), and C++ may assign
	// every exported variable
	g, _ := threadsOf(t, "package main\n\ntype T struct{ n int }\n\nvar a *T\nvar Exp *T\n\nfunc init() { a = &T{} }\nfunc main() {}\n", true)
	if !g.mutatedGlobal(g.pkg.Types.Scope().Lookup("a").(*types.Var)) {
		t.Errorf("a library keeps no init exemption")
	}
	if !g.mutatedGlobal(g.pkg.Types.Scope().Lookup("Exp").(*types.Var)) {
		t.Errorf("an exported variable of a library may be assigned by foreign code")
	}
	g, _ = threadsOf(t, "package main\n\nvar Exp *int\n\nfunc main() { go func() {}() }\n", false)
	if g.mutatedGlobal(g.pkg.Types.Scope().Lookup("Exp").(*types.Var)) {
		t.Errorf("an exported variable that nothing assigns is not assigned in a program (closed world)")
	}
}

// TestMutatedGlobals checks actual argument/receiver paths, including fields hidden by
// promotion. These are compile-time assertions, so they do not rely on winning a data race.
func TestMutatedGlobals(t *testing.T) {
	const src = `package main

type T struct{ n int }
func (t *T) read() int { return t.n }
func use(t *T) {}
type Inner struct { p *T; *T }
type Holder struct { *Inner }
var changed, unchanged *T
var h = &Holder{}
func hotGlobal() { use(changed) }
func coldGlobal() { use(unchanged) }
func hotField() { local := &Holder{}; use(local.p) }
func hotReceiver() { local := &Holder{}; local.read() }

type A struct{ n int }
func useA(a *A) {}
func hotArray() { var a [2]*A; a = [2]*A{}; useA(a[0]) }
type B struct{ n int }
func useB(b *B) {}
func hotAppend(s []*B) { s = append(s, nil); useB(s[0]) }
type MapElem struct{ n int }
func useMap(c *MapElem) {}
func hotDelete(m map[int]*MapElem) { delete(m, 0); useMap(m[1]) }

type D0 struct{ p *T }
type D1 struct{ in D0 }
type D2 struct{ in D1 }
type D3 struct{ in D2 }
type D4 struct{ in D3 }
type D5 struct{ in D4 }
type D6 struct{ in D5 }
func hotDeep() { var d D6; d = D6{}; use(d.in.in.in.in.in.in.p) }
func main() { changed = nil; h.Inner = nil; go func() {}() }
`
	g, _ := threadsOf(t, src, false)
	for _, f := range g.pkg.Load.Files {
		for _, d := range f.AST.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || !strings.HasPrefix(fd.Name.Name, "hot") && !strings.HasPrefix(fd.Name.Name, "cold") {
				continue
			}
			t.Run(fd.Name.Name, func(t *testing.T) {
				fg := newFuncGen(g, g.newFuncNames(), nil)
				checked := false
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					c, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					var got bool
					switch fun := c.Fun.(type) {
					case *ast.Ident:
						if !strings.HasPrefix(fun.Name, "use") {
							return true
						}
						got = fg.hotLoad(c.Args[0])
					case *ast.SelectorExpr:
						got = fg.hotRecv(fun, pathOf(g.info.Selections[fun]))
					default:
						return true
					}
					checked = true
					if want := strings.HasPrefix(fd.Name.Name, "hot"); got != want {
						t.Errorf("hot load = %v, want %v", got, want)
					}
					return true
				})
				if !checked {
					t.Fatal("no argument or receiver checked")
				}
			})
		}
	}
}

func repoRootDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Skip("not in the repository")
	}
	return dir
}

// TestFmtNativeThreadSources audits the real native bindings and callback ABI through both
// parser paths. Retaining a callback is not a promise of synchronous invocation or purity.
func TestFmtNativeThreadSources(t *testing.T) {
	for _, mode := range []struct {
		name    string
		workers int
	}{
		{"sequential parser", 1},
		{"parallel parser", 4},
	} {
		t.Run(mode.name, func(t *testing.T) {
			previous := runtime.GOMAXPROCS(mode.workers)
			defer runtime.GOMAXPROCS(previous)
			directory := t.TempDir()
			source := "package main\nimport \"os\"\nvar _ = os.Stdout\nfunc main() {}\n"
			if err := os.WriteFile(filepath.Join(directory, "main.gp"), []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
			program := load.Load(load.Config{
				Dir: directory, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd,
				GOOS: "linux", GOARCH: "amd64",
			}, ".")
			if program.Errors.HasErrors() {
				t.Fatalf("load errors: %v", program.Errors.Diagnostics())
			}
			checked := check.Check(program, check.Config{Phase: check.LastPhase})
			osLoaded := program.Roots[0].Imports["os"]
			if osLoaded == nil {
				t.Fatal("os was not loaded")
			}
			pkg := checked.Package(osLoaded)
			if pkg == nil || pkg.Errors.HasErrors() {
				t.Fatalf("os was not checked successfully: %v", pkg)
			}

			// Construct the expected full Go signatures from the checked package's exact types.
			file := pkg.Types.Scope().Lookup("File").Type()
			innerFile := pkg.Types.Scope().Lookup("file").Type()
			atomic := checked.Package(osLoaded.Imports["sync/atomic"])
			if atomic == nil || atomic.Errors.HasErrors() {
				t.Fatal("sync/atomic was not checked successfully")
			}
			flag := types.NewPointer(atomic.Types.Scope().Lookup("Bool").Type())
			filePointer := types.NewPointer(file)
			variable := types.NewPointer(filePointer)
			original := types.NewPointer(innerFile)
			write := fmtNativeSignature(
				[]types.Type{filePointer, types.NewSlice(types.Typ[types.Uint8])},
				[]types.Type{types.Typ[types.Int], types.Universe.Lookup("error").Type()},
			)
			stdin := fmtNativeSignature(
				[]types.Type{filePointer},
				[]types.Type{types.Universe.Lookup("any").Type()},
			)
			closeNow := fmtNativeSignature(nil, nil)
			g := newGen(checked.Fset, pkg)
			for _, native := range []struct {
				name string
				cxx  string
				sig  *types.Signature
			}{
				{"setFmtStdout", "gp::fmt::set_os_stdout", fmtNativeSignature(
					[]types.Type{variable, original, flag, write}, nil,
				)},
				{"setFmtStdin", "gp::fmt::set_os_stdin", fmtNativeSignature(
					[]types.Type{variable, original, flag, stdin}, nil,
				)},
				{"fmtDeferClose", "gp::fmt::defer_close", fmtNativeSignature(
					[]types.Type{types.Typ[types.Bool], closeNow}, []types.Type{types.Typ[types.Bool]},
				)},
			} {
				t.Run(native.name, func(t *testing.T) {
					fn, ok := pkg.Types.Scope().Lookup(native.name).(*types.Func)
					if !ok || !types.Identical(fn.Signature(), native.sig) {
						t.Fatalf("native signature changed: %v, want %v", fn, native.sig)
					}
					if !callbackSignature(fn.Signature()) || !threadSources[fn.FullName()] ||
						synchronousNatives[fn.FullName()] {
						t.Fatalf("retained native callback lost its conservative classification: %v", fn)
					}
					if summary := g.borrowOf(fn); summary != nil && (summary.pure || summary.pureV) {
						t.Fatalf("native callback must not become pure: %+v", summary)
					}

					binding := ""
					for _, file := range osLoaded.Files {
						for _, declaration := range file.AST.Decls {
							fd, ok := declaration.(*ast.FuncDecl)
							if ok && fd.Name.Name == native.name {
								binding, _ = load.NativeFunc(fd)
							}
						}
					}
					if binding != native.cxx {
						t.Fatalf("native binding = %q, want %q", binding, native.cxx)
					}
				})
			}
		})
	}
}

func fmtNativeSignature(params, results []types.Type) *types.Signature {
	tuple := func(fields []types.Type) *types.Tuple {
		variables := make([]*types.Var, len(fields))
		for i, field := range fields {
			variables[i] = types.NewVar(token.NoPos, nil, "", field)
		}
		return types.NewTuple(variables...)
	}
	return types.NewSignatureType(nil, nil, nil, tuple(params), tuple(results), false)
}

// harmlessCallbackNatives are the native functions of the standard library that take a func (or a
// finalizer) and are NOT thread sources, with the reason.
var harmlessCallbackNatives = map[string]string{
	"iter.newcoro":           "a coroutine on its own thread, but in lock step with its caller: one runs at a time",
	"runtime.setPanicValues": "stores two callbacks the runtime calls on the panicking thread",
	"time.callOnce":          "runs f once, synchronously (the zone tables)",
	"time.startChan":         "starts the timer thread for a channel timer: f is the library's own send, no user code",
}

// TestThreadSources: every bodyless function of the standard library that takes a func (or is
// SetFinalizer) is a thread source (threadSources) or declared harmless above, and the C++
// files of the runtime that create threads are the ones that were reviewed. A new native
// that starts a thread has to be listed here and in threadSources, or the gate of hotLoad would
// call a program without goroutines "single threaded" while it is not.
func TestThreadSources(t *testing.T) {
	root := repoRootDir(t)
	fset := token.NewFileSet()
	found := map[string]bool{}
	err := filepath.WalkDir(filepath.Join(root, "stdlib"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".gp") || strings.HasSuffix(p, "_test.gp") {
			return err
		}
		f, perr := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if perr != nil {
			return nil // (a file with gocpp syntax: it has no natives of this kind)
		}
		rel, _ := filepath.Rel(filepath.Join(root, "stdlib"), filepath.Dir(p))
		pkg := filepath.ToSlash(rel)
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body != nil {
				continue
			}
			callback := fd.Name.Name == "SetFinalizer"
			for _, field := range fd.Type.Params.List {
				if _, isFunc := field.Type.(*ast.FuncType); isFunc {
					callback = true
				}
			}
			if !callback {
				continue
			}
			name := pkg + "." + fd.Name.Name
			if fd.Recv != nil {
				continue // (no native method takes a func today: add the case when one does)
			}
			found[name] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 {
		t.Fatal("no native with a callback found: the scan is broken")
	}
	for name := range found {
		if !threadSources[name] && harmlessCallbackNatives[name] == "" {
			t.Errorf("the native %s takes a func and is neither in threadSources nor in harmlessCallbackNatives (threads_test.go)", name)
		}
	}
	for name := range harmlessCallbackNatives {
		if !synchronousNatives[name] {
			t.Errorf("reviewed native %s is missing from synchronousNatives", name)
		}
	}
	for name := range synchronousNatives {
		if harmlessCallbackNatives[name] == "" {
			t.Errorf("synchronous native %s has no reviewed explanation", name)
		}
	}
	for name := range threadSources {
		if strings.HasPrefix(name, "(") {
			continue // methods are found by their go statement, not by the scan
		}
		if !found[name] && name != "time.AfterFunc" && name != "context.AfterFunc" {
			t.Errorf("threadSources has %s, which is not a native with a callback (the list is stale?)", name)
		}
	}

	// The C++ of the runtime that creates threads.
	want := []string{"runtime/src/os/linux/thread.cpp", "runtime/src/os/windows/thread.cpp", "runtime/src/timer.cpp"}
	var got []string
	for _, dir := range []string{"runtime/src", "runtime/include"} {
		filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			b, _ := os.ReadFile(p)
			s := string(b)
			for _, pat := range []string{"std::thread(", "std::jthread", "CreateThread(", "pthread_create(", "_beginthread"} {
				if strings.Contains(s, pat) {
					rel, _ := filepath.Rel(root, p)
					got = append(got, filepath.ToSlash(rel))
					return nil
				}
			}
			return nil
		})
	}
	sort.Strings(got)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("the runtime files that create threads are %v, reviewed: %v. A new one must be checked against threadSources and added here", got, want)
	}
}
