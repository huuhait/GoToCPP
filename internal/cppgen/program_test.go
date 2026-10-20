package cppgen

import (
	"bytes"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// The tests of EmitProgram (Phase 06 step 6.2): the files of a program of several packages.

// loadProgram loads and checks the module in the folder dir (the packages of ".") with the
// standard library std (stdlib.FS() if nil), and returns the packages that get C++ of their own
// in the order of their imports: the packages of the module, not fmt (the runtime has its
// functions) and not the other packages of std that are stubs.
func loadProgram(t *testing.T, dir string, std fs.FS, goStd func(string) bool) (*token.FileSet, []ProgramPackage) {
	t.Helper()
	if std == nil {
		std, goStd = stdlib.FS(), stdlib.IsStd
	}
	prog := load.Load(load.Config{Dir: dir, Stdlib: std, GoStd: goStd, GOOS: "linux", GOARCH: "amd64"}, ".")
	r := check.Check(prog, check.Config{Phase: check.LastPhase})
	var errs []string
	for _, p := range prog.Packages {
		if cp := r.Package(p); cp != nil && cp.Errors.HasErrors() {
			for _, d := range cp.Errors.Diagnostics() {
				errs = append(errs, d.Pos.String()+": "+d.Msg)
			}
		}
	}
	for _, d := range prog.Errors.Diagnostics() {
		errs = append(errs, d.Pos.String()+": "+d.Msg)
	}
	if len(errs) > 0 {
		t.Fatalf("the program has errors:\n%s", strings.Join(errs, "\n"))
	}
	// The packages of the program, like cmd/gocpp's load: what the main package imports, without
	// the packages that get no C++ (fmt, a stub) and what only they import (fmt imports io, which
	// the program has no C++ of when nothing else uses it).
	reach := make(map[*load.Package]bool)
	var visit func(p *load.Package)
	visit = func(p *load.Package) {
		if reach[p] {
			return
		}
		reach[p] = true
		if p.Std && (p.ImportPath == "fmt" || stub(p)) {
			return
		}
		for _, dep := range p.Imports {
			visit(dep)
		}
	}
	for _, p := range prog.Packages {
		if p.IsProgram() {
			visit(p)
		}
	}
	var pkgs []ProgramPackage
	for _, p := range prog.Packages {
		if !reach[p] || p.Std && (p.ImportPath == "fmt" || stub(p)) {
			continue
		}
		pkgs = append(pkgs, ProgramPackage{Pkg: r.Package(p)})
	}
	return prog.Fset, pkgs
}

// stub reports whether the standard package p is a declaration stub (a function without a body
// that is not native).
func stub(p *load.Package) bool {
	return p.IsStub()
}

// emitProgram returns the files of the module in dir written for the root folder root.
func emitProgram(t *testing.T, dir, root string) *ProgramFiles {
	t.Helper()
	fset, pkgs := loadProgram(t, dir, nil, nil)
	files, err := EmitProgram(fset, pkgs, root, Config{})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// snapshot writes the files one after the other, each after a line "//// path", with the
// folder dir written as $DIR.
func snapshot(files *ProgramFiles, dir string) string {
	var b strings.Builder
	for _, f := range files.Files {
		b.WriteString("//// " + f.Path + "\n")
		b.Write(f.Data)
	}
	return withDir(b.String(), dir)
}

// TestProgramLayout compares the files of a program of three packages with testdata/program/three.golden
// (go test -update rewrites it): the header of each package (types, descriptors declared extern,
// prototypes, extern variables, the exported constants as constexpr), the translation unit of each
// .gp file, pkg_desc.cpp and pkg_init.cpp, where main's has gp_init_all and main.
func TestProgramLayout(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "program", "three"))
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "gen")
	got := snapshot(emitProgram(t, dir, root), dir)
	got = strings.ReplaceAll(got, root, "$GEN")
	golden := filepath.Join("testdata", "program", "three.golden")
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (go test -update writes it)", err)
	}
	if got != string(want) {
		t.Errorf("the files differ from %s (go test -update rewrites it):\n%s", golden, got)
	}
}

// TestProgramDeterministic emits the program twice, and from a copy in another folder: the
// bytes are the same (but for the folder in the #line directives).
func TestProgramDeterministic(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "program", "three"))
	if err != nil {
		t.Fatal(err)
	}
	copyDir := filepath.Join(t.TempDir(), "copy")
	if err := os.CopyFS(copyDir, os.DirFS(dir)); err != nil {
		t.Fatal(err)
	}
	a := snapshot(emitProgram(t, dir, "/gen"), dir)
	b := snapshot(emitProgram(t, dir, "/gen"), dir)
	c := snapshot(emitProgram(t, copyDir, "/gen"), copyDir)
	if a != b || a != c {
		t.Error("the files of the same program differ between runs, or between folders")
	}
}

// TestProgramRuns builds the program with each compiler, one translation unit at a time (the
// headers are found through -iquote), links them with the runtime and runs it.
func TestProgramRuns(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "program", "three"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(dir, "output.txt"))
	if err != nil {
		t.Fatal(err)
	}
	checkProgramRuns(t, dir, nil, nil, string(want), Config{})
}

// TestProgramForwarders writes the forwarders of the promoted methods of every struct
// (Config.AllForwarders): mid.Box promotes base.Rect's Area across packages, declared in
// the struct of mid's header and defined in mid's pkg_desc.cpp, and the program still runs.
func TestProgramForwarders(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "program", "three"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(dir, "output.txt"))
	if err != nil {
		t.Fatal(err)
	}
	fset, pkgs := loadProgram(t, dir, nil, nil)
	files, err := EmitProgram(fset, pkgs, "", Config{AllForwarders: true})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot(files, dir)
	for _, w := range []string{"int64_t Area() const;", "int64_t Box::Area() const {", "//// gpkg/example.com/three/mid/pkg_desc.cpp"} {
		if !strings.Contains(got, w) {
			t.Errorf("the files don't have %q:\n%s", w, got)
		}
	}
	checkProgramRuns(t, dir, nil, nil, string(want), Config{AllForwarders: true})
}

// TestProgramForwarderNames: a struct that declares its own unexported method m and promotes the
// unexported m of an embedded type of another package has two members, which are different
// methods in Go (mid.m and base.m): the forwarder's name has the package of the method.
func TestProgramForwarderNames(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "program", "fwdnames"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(dir, "output.txt"))
	if err != nil {
		t.Fatal(err)
	}
	fset, pkgs := loadProgram(t, dir, nil, nil)
	files, err := EmitProgram(fset, pkgs, "", Config{AllForwarders: true})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot(files, dir)
	if !strings.Contains(got, "name__example_dcom__fwdnames__base") || !strings.Contains(got, "gp::string name() const;") {
		t.Errorf("the struct of mid doesn't have both members:\n%s", got)
	}
	checkProgramRuns(t, dir, nil, nil, string(want), Config{AllForwarders: true})
}

// TestProgramConstants: what a C++ user of a package sees of its exported constants. A C++ file
// includes pkg.hpp of package k and checks them with static_assert (the generated code never reads
// them: go/types folds every use, but the header is the API of cpp-lib users).
func TestProgramConstants(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "program", "consts"))
	if err != nil {
		t.Fatal(err)
	}
	check, err := os.ReadFile(filepath.Join(dir, "check.cpp.txt"))
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "gen")
	files := emitProgram(t, dir, root)
	for _, f := range files.Files {
		p := filepath.Join(root, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, f.Data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	src := filepath.Join(root, "check.cpp")
	if err := os.WriteFile(src, check, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range compilers(t) {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			if ok, out, err := c.syntaxOK(src, includeFlag(t), "-iquote", root); err != nil || !ok {
				t.Errorf("the constants of the header don't check: %v\n%s", err, out)
			}
		})
	}
}

// checkProgramRuns emits the module in dir, builds it with each compiler and checks its output.
func checkProgramRuns(t *testing.T, dir string, std fs.FS, goStd func(string) bool, want string, cfg Config) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "gen")
	fset, pkgs := loadProgram(t, dir, std, goStd)
	files, err := EmitProgram(fset, pkgs, root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	var units []string
	for _, f := range files.Files {
		p := filepath.Join(root, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, f.Data, 0o644); err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(f.Path, ".cpp") {
			units = append(units, p)
		}
	}
	for _, c := range compilers(t) {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			bin := c.buildUnits(t, root, units)
			if got := run(t, dir, bin); got.stdout != want || got.code != 0 || got.stderr != "" {
				t.Errorf("the program prints %+v, want %q", got, want)
			}
		})
	}
}

// buildUnits compiles the translation units (generated code, which includes the headers of the
// folder root by their paths in it) and links them with the runtime into a program.
func (c *cxx) buildUnits(t *testing.T, root string, units []string) string {
	t.Helper()
	objs := c.runtimeObjectsMode(t, false)
	flags := c.flags(includeFlag(t), "-iquote", root)
	flags = append(flags, c.pchArgs(t, false, flags)...)
	var wg sync.WaitGroup
	outs := make([][]byte, len(units))
	errs := make([]error, len(units))
	codes := make([]int, len(units))
	var linked []string
	for i, u := range units {
		obj := strings.TrimSuffix(u, ".cpp") + "-" + c.name + ".o"
		linked = append(linked, obj)
		wg.Go(func() {
			args := append(append([]string{}, flags...), "-c", u, "-o", obj)
			outs[i], _, codes[i], errs[i] = command(root, true, c.cc.Path, args...)
		})
	}
	wg.Wait()
	for i, u := range units {
		if errs[i] != nil || codes[i] != 0 || len(outs[i]) != 0 {
			t.Fatalf("%s -c %s: exit code %d %v\n%s", c.cc.Path, u, codes[i], errs[i], outs[i])
		}
	}
	bin := filepath.Join(root, "prog-"+c.name)
	args := append(append([]string{}, c.flags()...), linked...)
	args = append(args, objs...)
	args = append(args, "-o", bin)
	out, _, code, err := command(root, true, c.cc.Path, args...)
	if err != nil || code != 0 || len(out) != 0 {
		t.Fatalf("%s %s: exit code %d %v\n%s", c.cc.Path, strings.Join(args, " "), code, err, out)
	}
	return bin
}

// TestProgramStdPackage: a standard package written in .gp, with bodies, is built like any
// other package (the driver skips only the shims and the stubs): here a package "mathx" of a
// made-up standard library, with a type, a method, a constant and a variable.
func TestProgramStdPackage(t *testing.T) {
	std := fstest.MapFS{
		"mathx/mathx.gp": &fstest.MapFile{Data: []byte(`package mathx

const Two = 2

type Vec struct{ X, Y int }

func (v Vec) Dot(w Vec) int { return v.X*w.X + v.Y*w.Y }

var Calls int

func Double(n int) int { Calls++; return n * Two }
`)},
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gocpp.mod"), []byte("module example.com/usesstd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.gp"), []byte(`package main

import (
	"fmt"
	"mathx"
)

func main() {
	v := mathx.Vec{3, 4}
	fmt.Println(v.Dot(v), mathx.Double(21), mathx.Calls)
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	goStd := func(path string) bool { return path == "mathx" || stdlib.IsStd(path) }
	// The embedded standard library has no mathx: a union of it and the made-up package.
	checkProgramRuns(t, dir, unionFS{std, stdlib.FS()}, goStd, "25 42 1\n", Config{})
	// The namespace and the folder are those of the import path, like any package's.
	fset, pkgs := loadProgram(t, dir, unionFS{std, stdlib.FS()}, goStd)
	files, err := EmitProgram(fset, pkgs, "", Config{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range files.Files {
		got = append(got, f.Path)
	}
	if !strings.Contains(strings.Join(got, "\n"), "gpkg/mathx/pkg.hpp") || !bytes.Contains(files.Files[0].Data, []byte("namespace gpkg_mathx")) {
		t.Errorf("the files %v don't have the package mathx in its folder and its namespace", got)
	}
}

// TestProgramNativeFunction: a function of a standard package without a body and with
// //gocpp:native is the runtime's C++ function (ADR-027): the package is built, the calls (from the
// package itself and from main) name the C++ function, and nothing declares or defines the Go
// function. A bodyless function without the directive is a stub: the package is skipped, and a
// call of it is "not supported yet".
func TestProgramNativeFunction(t *testing.T) {
	std := fstest.MapFS{
		"natx/natx.gp": &fstest.MapFile{Data: []byte(`package natx

import "errors"

// Comparable is the runtime's gp::errors::comparable.
//
//gocpp:native gp::errors::comparable
func Comparable(err error) bool

func Twice(err error) int {
	n := 0
	if Comparable(err) {
		n += 2
	}
	return n
}

var Known = errors.New("known")
`)},
		"stubx/stubx.gp": &fstest.MapFile{Data: []byte("package stubx\n\nfunc F() int\n")},
	}
	goStd := func(path string) bool { return path == "natx" || path == "stubx" || stdlib.IsStd(path) }
	write := func(dir, src string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "gocpp.mod"), []byte("module example.com/usesnat\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "main.gp"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	write(dir, `package main

import (
	"errors"
	"fmt"

	"natx"
)

type listErr struct{ parts []string }

func (listErr) Error() string { return "list" }

func main() {
	fmt.Println(natx.Comparable(natx.Known), natx.Comparable(listErr{}), natx.Twice(errors.New("x")), natx.Twice(listErr{}))
}
`)
	checkProgramRuns(t, dir, unionFS{std, stdlib.FS()}, goStd, "true false 2 0\n", Config{})
	fset, pkgs := loadProgram(t, dir, unionFS{std, stdlib.FS()}, goStd)
	files, err := EmitProgram(fset, pkgs, "", Config{})
	if err != nil {
		t.Fatal(err)
	}
	var all strings.Builder
	for _, f := range files.Files {
		all.Write(f.Data)
	}
	text := all.String()
	if n := strings.Count(text, "::gp::errors::comparable("); n != 4 {
		t.Errorf("%d calls of ::gp::errors::comparable, want 4 (two in main, one in natx, one in errors)", n)
	}
	if strings.Contains(text, "Comparable(") {
		t.Errorf("the generated code names the native function (it has the runtime's C++ name only)")
	}

	// A stub is not built, and calling it is not supported.
	dir2 := t.TempDir()
	write(dir2, "package main\n\nimport \"stubx\"\n\nfunc main() { _ = stubx.F() }\n")
	fset, pkgs = loadProgram(t, dir2, unionFS{std, stdlib.FS()}, goStd)
	_, err = EmitProgram(fset, pkgs, "", Config{})
	if err == nil || !strings.Contains(err.Error(), "the call of stubx.F") {
		t.Errorf("calling a stub: error %v, want 'not supported yet: the call of stubx.F'", err)
	}
}

// unionFS is the files of a, then those of b.
type unionFS []fs.FS

func (u unionFS) Open(name string) (fs.File, error) {
	var err error
	for _, f := range u {
		var file fs.File
		if file, err = f.Open(name); err == nil {
			return file, nil
		}
	}
	return nil, err
}

// TestProgramBorrowHotPaths checks the final C++, not just the thread facts: implicit embedded
// fields must also prevent raw lends in direct calls, method-expression adapters and forwarders.
func TestProgramBorrowHotPaths(t *testing.T) {
	for _, lib := range []bool{false, true} {
		name, mutate, want := "threaded", "func replace(i *Inner) { h.Inner = i }", 3
		if lib {
			name, mutate, want = "library", "", 4 // foreign callers can replace either embedded field
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			for path, src := range map[string]string{
				"gocpp.mod": "module example.com/hotpaths\n\ngo 1.25\n",
				"main.gp": `package main

type T struct{ n int }
func (t *T) read() int { return t.n }
type Inner struct{ *T }
type Holder struct{ *Inner }
var h *Holder
func direct(local *Holder) int { return local.read() }
func expression(local *Holder) int { return (*Holder).read(local) }
func main() { go func() {}() }
` + mutate,
			} {
				if err := os.WriteFile(filepath.Join(dir, path), []byte(src), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			fset, pkgs := loadProgram(t, dir, nil, nil)
			files, err := EmitProgram(fset, pkgs, "", Config{Library: lib, AllForwarders: true})
			if err != nil {
				t.Fatal(err)
			}
			got := strings.ReplaceAll(snapshot(files, dir), "gpkg_main::", "") // T is shadowed by Inner.T
			if n := strings.Count(got, "T::read(gp::ptr<T>("); n != want {
				t.Errorf("%d owning receiver copies, want %d (direct, method expression, forwarders):\n%s", n, want, got)
			}
		})
	}
}

// TestProgramBorrowHotGuard checks copy/lend decisions without the runtime fixtures' mutexes.
// Each storage/mutation form has a hot case and controls removing either prerequisite, so a
// synchronization call or unrelated thread source cannot mask a lost defensive-copy guard.
func TestProgramBorrowHotGuard(t *testing.T) {
	cfg := "var cfg = &Config{k: 1}\n"
	cases := []struct {
		name        string
		declaration string
		store       string
		argument    string
		call        string
		prefix      string
		placement   string
	}{
		{"package pointer", cfg, "cfg = &Config{k: 2}", "cfg", "sum(cfg)", "sum", "function"},
		{"value field", "var hold = struct{ c *Config }{c: &Config{k: 1}}\n",
			"hold.c = &Config{k: 2}", "hold.c", "sum(hold.c)", "sum", "function"},
		// Constructing &Server{} would itself expose the fields and mask the cold control.
		{"heap field", "type Server struct{ cfg *Config }\nvar srv *Server\n",
			"srv.cfg = &Config{k: 2}", "srv->cfg", "sum(srv.cfg)", "sum", "function"},
		{"array element", "var cfgs = [2]*Config{nil, &Config{k: 1}}\n",
			"cfgs[1] = &Config{k: 2}", "cfgs[1]", "sum(cfgs[1])", "sum", "function"},
		{"pointer receiver", cfg, "cfg = &Config{k: 2}",
			"cfg", "cfg.total()", "Config::total", "function"},
		{"closure in main", cfg, "cfg = &Config{k: 2}", "cfg", "sum(cfg)", "sum", "closure-main"},
		{"closure in init", cfg, "cfg = &Config{k: 2}", "cfg", "sum(cfg)", "sum", "closure-init"},
		{"init store", cfg, "cfg = &Config{k: 2}", "cfg", "sum(cfg)", "sum", "init"},
		{"indirect init store", cfg, "cfg = &Config{k: 2}",
			"cfg", "sum(cfg)", "sum", "init-indirect"},
		{"init loop store", cfg, "cfg = &Config{k: 2}", "cfg", "sum(cfg)", "sum", "init-loop"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range []struct {
				name     string
				threaded bool
				mutates  bool
			}{
				{"hot replacement", true, true},
				{"no thread source", false, true},
				{"no replacement", true, false},
			} {
				t.Run(mode.name, func(t *testing.T) {
					store := ""
					if mode.mutates {
						store = tc.store + "\n"
					}
					start := ""
					if mode.threaded {
						start = "go worker()\n"
					}
					main := ""
					extra := ""
					switch tc.placement {
					case "function":
						extra = "func replace() {\n" + store + "}\n"
						main = start + main
					case "closure-main":
						main = "swap := func() {\n" + store + "}\n_ = swap\n" + start + main
					case "closure-init":
						extra = "var reload func()\nfunc init() { reload = func() {\n" +
							store + "} }\n"
						main = start + main
					case "init":
						extra = "func init() {\n" + start + store + "}\n"
					case "init-indirect":
						extra = "func start() {\n" + start + "}\nfunc init() {\nstart()\n" +
							store + "}\n"
					case "init-loop":
						extra = "func init() {\n" + start + "for i := 0; i < 2; i++ {\n" +
							store + "} }\n"
					}
					source := `package main
					type Config struct{ k int }
					func sum(c *Config) int { return c.k }
					func (c *Config) total() int { return c.k }
					func worker() {}
					` + tc.declaration + extra + "func sample() { _ = " + tc.call + " }\n" +
						"func main() {\n" + main + "}\n"
					directory := t.TempDir()
					path := filepath.Join(directory, "main.gp")
					if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
						t.Fatal(err)
					}
					files := emitProgram(t, directory, "")
					cpp := snapshot(files, directory)
					cpp = strings.ReplaceAll(cpp, "::gpkg_main::", "")
					cpp = strings.ReplaceAll(cpp, "gpkg_main::", "")
					argument := "gp::lend(" + tc.argument + ")"
					if mode.threaded && mode.mutates {
						argument = "gp::ptr<Config>(" + tc.argument + ")"
					}
					want := tc.prefix + "(" + argument + ")"
					if !strings.Contains(cpp, want) {
						t.Fatalf("missing %q:\n%s", want, cpp)
					}
				})
			}
		})
	}
}

// TestProgramLibraryBorrow: the exported functions and methods of a library keep owning pointers
// in their C++ signatures (ADR-086: C++ callers decide how long a pointer lives), the unexported
// ones borrow, and a program (not a library) borrows everywhere.
func TestProgramLibraryBorrow(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "program", "libborrow"))
	if err != nil {
		t.Fatal(err)
	}
	header := func(lib bool) string {
		root := filepath.Join(t.TempDir(), "gen")
		fset, pkgs := loadProgram(t, dir, nil, nil)
		files, err := EmitProgram(fset, pkgs, root, Config{Library: lib})
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files.Files {
			if strings.HasSuffix(f.Path, "libborrow/lib/pkg.hpp") {
				return string(f.Data)
			}
		}
		t.Fatal("no lib/pkg.hpp in the emitted files")
		return ""
	}
	lib := header(true)
	for _, want := range []string{"int64_t Use(gp::ptr<T> t)", "int64_t Get(gp::ptr<T> t)", "int64_t Drop(gp::ptr<T> t)", "int64_t get2(gp::bptr<T> t)", "int64_t unexported(gp::bptr<T> t)"} {
		if !strings.Contains(lib, want) {
			t.Errorf("the library header lacks %q:\n%s", want, lib)
		}
	}
	prog := header(false)
	for _, want := range []string{"int64_t Use(gp::bptr<T> t)", "int64_t Get(gp::bptr<T> t)"} {
		if !strings.Contains(prog, want) {
			t.Errorf("the program header lacks %q (the library test would prove nothing):\n%s", want, prog)
		}
	}
}
