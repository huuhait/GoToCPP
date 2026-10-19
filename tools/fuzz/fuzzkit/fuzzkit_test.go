package fuzzkit

import (
	"bytes"
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/cppgen"
)

const hello = "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hi\", 1+2)\n}\n"

// emitHello is the pipeline for a program that checks, lowers and emits.
func emitHello(t *testing.T) *Result {
	t.Helper()
	r := Run([]byte(hello), true)
	if r.EmitErr != nil || r.Files == nil || len(r.Internal) > 0 || !r.Checked || !r.Main {
		t.Fatalf("hello does not emit: checked=%v main=%v err=%v internal=%v errors=%v", r.Checked, r.Main, r.EmitErr, r.Internal, r.Errors)
	}
	return r
}

func TestPipelineClassifies(t *testing.T) {
	r := emitHello(t)
	if !r.Syntax {
		t.Error("hello has a syntax error")
	}
	// A type error: checked is false, no internal error, and nothing is emitted.
	r = Run([]byte("package main\n\nfunc main() { var x int = \"s\"; _ = x }\n"), true)
	if r.Checked || len(r.Errors) == 0 || r.Files != nil || len(r.Internal) > 0 || !r.Syntax {
		t.Errorf("type error: %+v", r)
	}
	// A syntax error.
	r = Run([]byte("package main\n\nfunc main( {\n"), true)
	if r.Syntax || r.Checked || r.Files != nil {
		t.Errorf("syntax error: %+v", r)
	}
	// Not a main package: checked, nothing emitted.
	r = Run([]byte("package p\n\nfunc F() {}\n"), true)
	if !r.Checked || r.Files != nil || r.Main {
		t.Errorf("library: checked=%v main=%v files=%d", r.Checked, r.Main, len(r.Files))
	}
}

func TestEmitDeterministic(t *testing.T) {
	a, b := emitHello(t), emitHello(t)
	if err := SameFiles(a.Files, b.Files); err != nil {
		t.Fatal(err)
	}
	if err := CheckLines([]byte(hello), a.Files); err != nil {
		t.Fatal(err)
	}
}

// The checks must report what they are for: each is run on a program that is wrong in that way.
func TestChecksReport(t *testing.T) {
	r := emitHello(t)
	cp := func() []cppgen.File {
		out := make([]cppgen.File, len(r.Files))
		for i, f := range r.Files {
			out[i] = cppgen.File{Path: f.Path, Data: bytes.Clone(f.Data)}
		}
		return out
	}
	// A difference between two emits.
	d := cp()
	d[0].Data = append(d[0].Data, "// x\n"...)
	if SameFiles(r.Files, d) == nil {
		t.Error("SameFiles does not see a changed file")
	}
	if SameFiles(r.Files, d[1:]) == nil {
		t.Error("SameFiles does not see a missing file")
	}
	// A #line past the end of the source, a #line of a file that is nowhere, a bad number.
	lineCases := map[string]string{
		"past the end":  "#line 999 \"" + File + "\"\n",
		"unknown file":  "#line 3 \"/nowhere.gp\"\n",
		"zero":          "#line 0 \"" + File + "\"\n",
		"not a number":  "#line x\n",
		"bad file name": "#line 1 \"a\\q\"\n",
	}
	for name, line := range lineCases {
		d := cp()
		d[0].Data = append(d[0].Data, line...)
		if err := CheckLines([]byte(hello), d); err == nil {
			t.Errorf("CheckLines does not see a #line %s", name)
		}
	}
	// What a good #line looks like is accepted.
	d = cp()
	d[0].Data = append(d[0].Data, "#line 3 \""+File+"\"\n#line 7\n"...)
	if err := CheckLines([]byte(hello), d); err != nil {
		t.Errorf("CheckLines refuses a good #line: %v", err)
	}
}

func TestDumpIgnoresFormatting(t *testing.T) {
	parse := func(src string) string {
		f, err := parser.ParseFile(token.NewFileSet(), "x.go", src, 0)
		if err != nil {
			t.Fatal(err)
		}
		return Dump(f)
	}
	a := parse("package p\n\nimport (\n\t\"b\"\n\t\"a\"\n)\n\nfunc f() int { return (1 + 2) } // c\n")
	b := parse("package p\nimport (\"a\"\n\"b\")\n// d\nfunc f() int {\n\treturn 1 + 2\n}\n")
	if a != b {
		t.Errorf("Dump differs for the same program:\n%s\n%s", a, b)
	}
	if parse("package p\nfunc f() { func()() }") != parse("package p\nfunc f() { func() }") {
		t.Error("Dump tells func()() from func() (the printer writes both as func())")
	}
	c := parse("package p\n\nimport (\n\t\"b\"\n\t\"a\"\n)\n\nfunc f() int { return 1 + 3 }\n")
	if a == c {
		t.Error("Dump does not see a changed literal")
	}
	d := parse("package p\n\nimport (\n\t\"b\"\n\t\"a\"\n)\n\nfunc g() int { return 1 + 2 }\n")
	if a == d {
		t.Error("Dump does not see a changed name")
	}
}

func TestSeeds(t *testing.T) {
	if n := len(GPSeeds()); n < 100 {
		t.Errorf("%d .gp seeds, want the files of tests, examples and stdlib", n)
	}
	progs := ProgramSeeds()
	if len(progs) < 50 {
		t.Errorf("%d program seeds", len(progs))
	}
	for _, p := range progs {
		if !strings.Contains(string(p), "package main") {
			t.Fatal("a program seed without package main")
		}
		if len(p) > maxSeed {
			t.Fatalf("a seed of %d bytes", len(p))
		}
	}
}

func TestSampled(t *testing.T) {
	t.Setenv("GOCPP_FUZZ_COMPILE_EVERY", "0")
	if Sampled([]byte("a")) {
		t.Error("sampled with the stage off")
	}
	t.Setenv("GOCPP_FUZZ_COMPILE_EVERY", "1")
	if !Sampled([]byte("a")) || !Sampled([]byte("b")) {
		t.Error("not sampled with N = 1")
	}
	t.Setenv("GOCPP_FUZZ_COMPILE_EVERY", "7")
	n := 0
	for i := range 700 {
		if Sampled([]byte{byte(i), byte(i >> 8), 'x'}) {
			n++
		}
	}
	if n < 50 || n > 150 {
		t.Errorf("%d of 700 sampled with N = 7", n)
	}
}

// The sampled stage must see C++ that does not compile: the emitter's output with a break in it
// (a broken emitter) fails with both compilers, and the output as it is passes.
func TestCompileStageSeesBrokenEmitter(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles C++")
	}
	cs, err := Compilers()
	if err != nil {
		if os.Getenv("GOCPP_REQUIRE_CXX") != "" {
			t.Fatal(err)
		}
		t.Skip(err)
	}
	t.Setenv("GOCPP_FUZZ_CACHE", t.TempDir())
	r := emitHello(t)
	if f, err := CompileAll(r); err != nil || f != nil {
		t.Fatalf("hello does not compile: %v %v", f, err)
	}
	broken := func(name string, edit func(cpp string) string) {
		files := make([]cppgen.File, len(r.Files))
		hit := false
		for i, f := range r.Files {
			files[i] = f
			if strings.HasPrefix(f.Path, r.MainPkg) && strings.HasSuffix(f.Path, ".cpp") {
				if e := edit(string(f.Data)); e != string(f.Data) {
					files[i] = cppgen.File{Path: f.Path, Data: []byte(e)}
					hit = true
				}
			}
		}
		if !hit {
			t.Fatalf("%s: the edit changed nothing", name)
		}
		bad := *r
		bad.Files = files
		for _, c := range cs {
			if fail := c.SyntaxOnly(files, r.MainPkg); fail == nil {
				t.Errorf("%s: %s accepts the broken C++", name, c.Name)
			}
		}
		if f, err := CompileAll(&bad); err != nil || f == nil {
			t.Errorf("%s: CompileAll does not report it (%v, %v)", name, f, err)
		}
	}
	broken("syntax error", func(s string) string { return s + "\nint broken = ;\n" })
	broken("unknown name", func(s string) string { return strings.Replace(s, "main()", "main(gp::NoSuchType x)", 1) })
	// A warning on by default (no -Wall) is a failure too: the driver compiles generated code without warnings.
	broken("warning", func(s string) string { return s + "\nint gen_missing_return() { }\n" })
}

func TestGuardPassesThrough(t *testing.T) {
	done := false
	Guard("test", func() { done = true })
	if !done {
		t.Fatal("Guard did not run the function")
	}
	// Guard does not leave a target armed: a pause after it is no hang.
	time.Sleep(10 * time.Millisecond)
}

// TestGuardHelper is the child of TestGuardEndsHang and TestGuardEndsMemoryBlowup: it hangs, or eats
// memory, inside Guard.
func TestGuardHelper(t *testing.T) {
	switch os.Getenv("GOCPP_GUARD_HELPER") {
	case "hang":
		Guard("helper", func() { time.Sleep(time.Minute) })
	case "mem":
		Guard("helper", func() {
			var keep [][]byte
			for range 100 {
				b := make([]byte, 16<<20)
				for i := range b {
					b[i] = 1
				}
				keep = append(keep, b)
				time.Sleep(50 * time.Millisecond)
			}
			_ = keep
		})
	}
}

func runGuardHelper(t *testing.T, mode string, env ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGuardHelper$")
	cmd.Env = append(os.Environ(), append([]string{"GOCPP_GUARD_HELPER=" + mode}, env...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestGuardEndsHang(t *testing.T) {
	out, err := runGuardHelper(t, "hang", "GOCPP_FUZZ_HANG=1")
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 3 || !strings.Contains(out, "HANG: helper") || !strings.Contains(out, "goroutine") {
		t.Fatalf("a hang inside Guard: %v\n%s", err, out)
	}
}

func TestGuardEndsMemoryBlowup(t *testing.T) {
	out, err := runGuardHelper(t, "mem", "GOCPP_FUZZ_MEM=64")
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 3 || !strings.Contains(out, "MEMORY BLOWUP") {
		t.Fatalf("memory inside Guard: %v\n%s", err, out)
	}
}

// TestPCHHelper is the child of TestSharedCacheProcesses: it compiles hello with every compiler.
func TestPCHHelper(t *testing.T) {
	if os.Getenv("GOCPP_PCH_HELPER") == "" {
		return
	}
	r := emitHello(t)
	if f, err := CompileAll(r); err != nil || f != nil {
		t.Fatalf("%v %v", f, err)
	}
}

// Several fuzz workers (processes) start together with an empty cache folder, as a campaign does:
// all must compile. (Once clang refused a .pch because a second worker rewrote its header.)
func TestSharedCacheProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles C++")
	}
	if _, err := Compilers(); err != nil {
		t.Skip(err)
	}
	cache := t.TempDir()
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPCHHelper$")
			cmd.Env = append(os.Environ(), "GOCPP_PCH_HELPER=1", "GOCPP_FUZZ_CACHE="+cache)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("worker %d: %v\n%s", i, err, out)
			}
		}()
	}
	wg.Wait()
}

// Dump leaves out what gofmt itself changes: the spelling of number literals, a repeated import
// spec, a ";" statement.
func TestDumpGofmtNormalizations(t *testing.T) {
	parse := func(src string) string {
		f, err := parser.ParseFile(token.NewFileSet(), "x.go", src, 0)
		if err != nil {
			t.Fatal(err)
		}
		return Dump(f)
	}
	pairs := [][2]string{
		{"package p\nvar x = 0XFF + 1E5\nvar y = 0123i\nvar z = 0X1P-2\n", "package p\nvar x = 0xFF + 1e5\nvar y = 123i\nvar z = 0x1p-2\n"},
		{"package p\nimport (\n\t\"a\"\n\t\"a\"\n)\n", "package p\nimport \"a\"\n"},
		{"package p\nfunc f() { ; }\n", "package p\nfunc f() {}\n"},
		{"package p\nfunc f() { x := 1; ; _ = x }\n", "package p\nfunc f() { x := 1; _ = x }\n"},
	}
	for _, p := range pairs {
		if parse(p[0]) != parse(p[1]) {
			t.Errorf("Dump differs:\n%s\n%s", p[0], p[1])
		}
	}
	if parse("package p\nvar x = 0xFF\n") == parse("package p\nvar x = 0xFE\n") {
		t.Error("Dump does not see a different number")
	}
	if parse("package p\nimport \"a\"\n") == parse("package p\nimport \"b\"\n") {
		t.Error("Dump does not see a different import")
	}
}

func TestCleanStale(t *testing.T) {
	tmp, cache := t.TempDir(), t.TempDir()
	mk := func(root, name string, age time.Duration) {
		d := filepath.Join(root, name)
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "f"), []byte("x"), 0o644)
		at := time.Now().Add(-age)
		os.Chtimes(d, at, at)
	}
	mk(tmp, "gocpp-fuzz-gen-old", 3*time.Hour)
	mk(tmp, "gocpp-fuzz-gen-new", time.Minute)
	mk(tmp, "other-old", 3*time.Hour)
	mk(cache, "rt-oldhash", 3*time.Hour)
	mk(cache, "rt-current", 3*time.Hour)
	mk(cache, "rt-newhash", time.Minute)
	cleanStale(tmp, cache, "rt-current", time.Now())
	exists := func(root, name string) bool {
		_, err := os.Stat(filepath.Join(root, name))
		return err == nil
	}
	for _, c := range []struct {
		root, name string
		want       bool
	}{
		{tmp, "gocpp-fuzz-gen-old", false}, {tmp, "gocpp-fuzz-gen-new", true}, {tmp, "other-old", true},
		{cache, "rt-oldhash", false}, {cache, "rt-current", true}, {cache, "rt-newhash", true},
	} {
		if exists(c.root, c.name) != c.want {
			t.Errorf("%s: exists %v, want %v", c.name, !c.want, c.want)
		}
	}
}
