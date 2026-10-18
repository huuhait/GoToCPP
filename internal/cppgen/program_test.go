package cppgen

import (
	"bytes"
	"go/ast"
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
	var pkgs []ProgramPackage
	for _, p := range prog.Packages {
		if p.Std && (p.ImportPath == "fmt" || stub(p)) {
			continue
		}
		pkgs = append(pkgs, ProgramPackage{Pkg: r.Package(p)})
	}
	return prog.Fset, pkgs
}

// stub reports whether the standard package p has functions without a body.
func stub(p *load.Package) bool {
	for _, f := range p.Files {
		for _, d := range f.AST.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body == nil {
				return true
			}
		}
	}
	return false
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
