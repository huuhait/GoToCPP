package build

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The tests of paths that a C++ compiler of a user can meet: a double quote (g++'s assembler
// markers) and letters outside ASCII (Nix's clang wrapper in the C locale).

// asmSource is a C++ file with an inline asm statement: g++ writes a line marker with the
// file's name for it.
const asmSource = "int nop() {\n\tasm volatile(\"nop\");\n\treturn 1;\n}\n"

// assertNoAssemblyLeft fails when the folder holds an assembly file of the two-step compile.
func assertNoAssemblyLeft(t *testing.T, dir string) {
	t.Helper()
	left, _ := filepath.Glob(filepath.Join(dir, "asm-*.s"))
	if len(left) != 0 {
		t.Errorf("assembly files left in %s: %q", dir, left)
	}
}

// underFolder returns the paths that are in dir (the system headers that g++ lists too are not
// the test's business).
func underFolder(paths []string, dir string) []string {
	return slices.DeleteFunc(slices.Clone(paths), func(p string) bool {
		return !strings.HasPrefix(p, dir+string(filepath.Separator))
	})
}

func TestQuoteInPathCompiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a double quote is not allowed in a Windows file name")
	}
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		dir := filepath.Join(t.TempDir(), `q"uote`, `dir é`)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		src := filepath.Join(dir, "asm.cpp")
		if err := os.WriteFile(src, []byte(asmSource), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, flags := range [][]string{{"-O0"}, {"-g"}, {"-O2"}, {"-fsanitize=address,undefined"}, {"-g", "-fsanitize=address,undefined"}} {
			t.Run(strings.Join(flags, " "), func(t *testing.T) {
				work := t.TempDir()
				obj := filepath.Join(work, "asm.o")
				b := &Builder{CXX: cxx, Work: work}
				args := slices.Concat([]string{"-std=c++20"}, flags, []string{"-c", src, "-o", obj})
				out, err := b.runOutput(context.Background(), cxx, "the test", false, args)
				if err != nil || len(out) != 0 {
					t.Fatalf("compile = %q, %v; want success and no output", out, err)
				}
				if fi, err := os.Stat(obj); err != nil || fi.Size() == 0 {
					t.Errorf("object file: %v, %v", fi, err)
				}
				assertNoAssemblyLeft(t, work)
			})
		}
	})
}

func TestQuoteInPathKeepsTheDependencyFile(t *testing.T) {
	// The dependency file of the two-step compile (assembly, then object) lists the source and
	// its headers, and so the object cache can use them.
	if runtime.GOOS == "windows" {
		t.Skip("a double quote is not allowed in a Windows file name")
	}
	cxx, err := LookCXX(context.Background(), "g++")
	if err != nil {
		t.Skip("no g++:", err)
	}
	dir := filepath.Join(t.TempDir(), `q"uote`)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "asm.cpp")
	header := filepath.Join(dir, "h.h")
	if err := os.WriteFile(header, []byte("#define H 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("#include \"h.h\"\n"+asmSource), 0o644); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	depfile := filepath.Join(work, "asm.d")
	b := &Builder{CXX: cxx, Work: work}
	args := []string{"-std=c++20", "-c", src, "-o", filepath.Join(work, "asm.o"), "-MD", "-MF", depfile}
	if out, err := b.runOutput(context.Background(), cxx, "the test", false, args); err != nil || len(out) != 0 {
		t.Fatalf("compile = %q, %v", out, err)
	}
	paths, ok := b.depPaths([]string{depfile}, src)
	paths = underFolder(paths, dir)
	if !ok || !slices.Equal(paths, []string{header}) {
		t.Errorf("depPaths = %q, %v; want %q", paths, ok, []string{header})
	}
}

func TestQuoteInPathReportsErrors(t *testing.T) {
	// A compile error in the first step is a CompileError with the command as the user wrote it.
	if runtime.GOOS == "windows" {
		t.Skip("a double quote is not allowed in a Windows file name")
	}
	cxx, err := LookCXX(context.Background(), "g++")
	if err != nil {
		t.Skip("no g++:", err)
	}
	dir := filepath.Join(t.TempDir(), `q"uote`)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "bad.cpp")
	if err := os.WriteFile(src, []byte("int f() { return undeclared_name; }\n"+asmSource), 0o644); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	b := &Builder{CXX: cxx, Work: work}
	_, err = b.runOutput(context.Background(), cxx, "the test", false, []string{"-c", src, "-o", filepath.Join(work, "bad.o")})
	ce, ok := err.(*CompileError)
	if !ok || !strings.Contains(string(ce.Output), "undeclared_name") || !strings.Contains(ce.Command, "-c ") ||
		strings.Contains(ce.Command, "-S ") {
		t.Fatalf("error = %#v, want a CompileError about undeclared_name with the -c command", err)
	}
	assertNoAssemblyLeft(t, work)
}

func TestAssemblerNeedsFix(t *testing.T) {
	gcc := CXX{Family: GCC}
	clang := CXX{Family: Clang}
	quote := `/p/q"uote/a.cpp`
	for _, c := range []struct {
		name string
		cxx  CXX
		args []string
		want bool
	}{
		{"g++ compile with a quote in the source", gcc, []string{"-g", "-c", quote, "-o", "/w/a.o"}, true},
		{"quote in an include folder", gcc, []string{"-I", `/p/q"uote/inc`, "-c", "/p/a.cpp", "-o", "/w/a.o"}, true},
		{"quote in a joined flag", gcc, []string{`-I/p/q"uote`, "-c", "/p/a.cpp", "-o", "/w/a.o"}, true},
		{"no quote", gcc, []string{"-c", "/p/a.cpp", "-o", "/w/a.o"}, false},
		{"clang escapes the name itself", clang, []string{"-c", quote, "-o", "/w/a.o"}, false},
		{"a link", gcc, []string{quote, "-o", "/w/prog"}, false},
		{"already assembly", gcc, []string{"-S", "-c", quote, "-o", "/w/a.s"}, false},
		{"preprocess only", gcc, []string{"-E", "-c", quote, "-o", "/w/a.i"}, false},
		{"precompiled header", gcc, []string{"-c", "-x", "c++-header", quote, "-o", "/w/a.pch"}, false},
		{"an LTO unit is compiled without LTO", gcc, []string{"-flto", "-c", quote, "-o", "/w/a.o"}, true},
		{"LTO with jobs", gcc, []string{"-flto=4", "-c", quote, "-o", "/w/a.o"}, true},
		{"no output file", gcc, []string{"-c", quote}, false},
		{"quote in a macro value", gcc, []string{`-DVER="1.0"`, "-c", "/p/a.cpp", "-o", "/w/a.o"}, false},
		{"quote in a macro, separate words", gcc, []string{"-D", `VER="1.0"`, "-c", "/p/a.cpp", "-o", "/w/a.o"}, false},
		{"quote in -U, -W and -f flags", gcc, []string{`-Wa,--x="y"`, `-fplugin-arg-x-y="z"`, "-c", "/p/a.cpp", "-o", "/w/a.o"}, false},
		{"quote in a forced include", gcc, []string{"-include", `/p/q"uote/h.h`, "-c", "/p/a.cpp", "-o", "/w/a.o"}, true},
		{"quote in -isystem, separate", gcc, []string{"-isystem", `/p/q"uote`, "-c", "/p/a.cpp", "-o", "/w/a.o"}, true},
		{"quote in -idirafter, joined", gcc, []string{`-idirafter/p/q"uote`, "-c", "/p/a.cpp", "-o", "/w/a.o"}, true},
		{"quote in --sysroot=", gcc, []string{`--sysroot=/p/q"uote`, "-c", "/p/a.cpp", "-o", "/w/a.o"}, true},
		{"quote in the output path only", gcc, []string{"-c", "/p/a.cpp", "-o", `/w/q"uote/a.o`}, false},
		{"quote in the dependency file path only", gcc, []string{"-c", "/p/a.cpp", "-o", "/w/a.o", "-MF", `/w/q"uote/a.d`}, false},
	} {
		if got := assemblerNeedsFix(c.cxx, c.args); got != c.want {
			t.Errorf("%s: assemblerNeedsFix = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestEscapeLineMarkers(t *testing.T) {
	for _, c := range []struct{ name, in, want string }{
		{"quote", "x\n# 70 \"/p/q\"uote/a.cpp\" 1\nnop\n# 0 \"\" 2\n", "x\n# 70 \"/p/q\\\"uote/a.cpp\" 1\nnop\n# 0 \"\" 2\n"},
		{"two quotes", "# 5 \"/a\"b\"c/f.cpp\" 1\n", "# 5 \"/a\\\"b\\\"c/f.cpp\" 1\n"},
		{"backslash", "# 5 \"/a\\b/f.cpp\" 1\n", "# 5 \"/a\\\\b/f.cpp\" 1\n"},
		{"nothing to escape", "# 5 \"/a/f.cpp\" 1\n\t.string \"q\"uote\"\n", "# 5 \"/a/f.cpp\" 1\n\t.string \"q\"uote\"\n"},
		{"other lines stay", "\t.file \"a\\\"b.cpp\"\n", "\t.file \"a\\\"b.cpp\"\n"},
	} {
		if got := string(escapeLineMarkers([]byte(c.in))); got != c.want {
			t.Errorf("%s: escapeLineMarkers(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestNonASCIIPathCompiles(t *testing.T) {
	// Nix's clang wrapper wrote a non-ASCII path as bash's $'\320...' in the C locale (the
	// locale of CompilerEnv); every compile in such a folder failed.
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		root := filepath.Join(t.TempDir(), "проект ёж", "Zoë é")
		inc := filepath.Join(root, "inc ü")
		for _, d := range []string{root, inc} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(inc, "h.h"), []byte("#define VALUE 7\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		src := filepath.Join(root, "a.cpp")
		if err := os.WriteFile(src, []byte("#include <h.h>\nint v = VALUE;\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		work := t.TempDir()
		obj := filepath.Join(work, "a.o")
		b := &Builder{CXX: cxx, Work: work}
		args := []string{"-std=c++20", "-g", "-I", inc, "-c", src, "-o", obj, "-MD", "-MF", filepath.Join(work, "a.d")}
		if out, err := b.runOutput(context.Background(), cxx, "the test", false, args); err != nil || len(out) != 0 {
			t.Fatalf("compile = %q, %v; want success and no output", out, err)
		}
		if fi, err := os.Stat(obj); err != nil || fi.Size() == 0 {
			t.Errorf("object file: %v, %v", fi, err)
		}
		paths, ok := b.depPaths([]string{filepath.Join(work, "a.d")}, src)
		paths = underFolder(paths, root)
		if want := []string{filepath.Join(inc, "h.h")}; !ok || !slices.Equal(paths, want) {
			t.Errorf("depPaths = %q, %v; want %q", paths, ok, want)
		}
	})
}

func TestCompilerEnvNixResponseFile(t *testing.T) {
	envValue := func() (string, int) {
		value, count := "", 0
		for _, kv := range CompilerEnv() {
			if v, ok := strings.CutPrefix(kv, "NIX_CC_USE_RESPONSE_FILE="); ok {
				value, count = v, count+1
			}
		}
		return value, count
	}
	t.Run("unset becomes 0", func(t *testing.T) {
		t.Setenv("NIX_CC_USE_RESPONSE_FILE", "") // restored when the test ends
		os.Unsetenv("NIX_CC_USE_RESPONSE_FILE")
		if v, n := envValue(); v != "0" || n != 1 {
			t.Errorf("NIX_CC_USE_RESPONSE_FILE = %q (%d entries), want one entry 0", v, n)
		}
	})
	t.Run("the user's value stays", func(t *testing.T) {
		t.Setenv("NIX_CC_USE_RESPONSE_FILE", "1")
		if v, n := envValue(); v != "1" || n != 1 {
			t.Errorf("NIX_CC_USE_RESPONSE_FILE = %q (%d entries), want one entry 1", v, n)
		}
	})
}

// A loggingCompiler is a g++ in front of the real one that writes every call's arguments, one line
// per call, to a log file.
func loggingCompiler(t *testing.T, real CXX) (cxx CXX, calls func() []string) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + log + "'\nexec '" + real.Path + "' \"$@\"\n"
	path := filepath.Join(dir, "logging-g++")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cxx = real
	cxx.Path = path
	return cxx, func() []string {
		data, _ := os.ReadFile(log)
		return strings.Split(strings.TrimSpace(string(data)), "\n")
	}
}

func lookGCC(t *testing.T) CXX {
	t.Helper()
	cxx, err := LookCXX(context.Background(), "g++")
	if err != nil {
		t.Skip("no g++:", err)
	}
	return cxx
}

func writeAsmSource(t *testing.T, dir, name, text string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, name)
	if err := os.WriteFile(src, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return src
}

func TestQuoteInMacroStaysOneStep(t *testing.T) {
	// A quote in -DVER="1.0" says nothing about a path: the compile is one call, in a folder with
	// a quote or without (without, it must not take the two-step route).
	if runtime.GOOS == "windows" {
		t.Skip("the logging compiler is a shell script")
	}
	cxx, calls := loggingCompiler(t, lookGCC(t))
	plain := writeAsmSource(t, t.TempDir(), "a.cpp", asmSource)
	work := t.TempDir()
	b := &Builder{CXX: cxx, Work: work}
	for _, define := range [][]string{{`-DVER="1.0"`}, {"-D", `VER="1.0"`}} {
		args := slices.Concat(define, []string{"-c", plain, "-o", filepath.Join(work, "a.o")})
		if out, err := b.runOutput(context.Background(), cxx, "the test", false, args); err != nil || len(out) != 0 {
			t.Fatalf("%q: %q, %v", define, out, err)
		}
	}
	if got := calls(); len(got) != 2 {
		t.Errorf("calls with a quote in a macro and no quote in a path: %q, want 2 (one each)", got)
	}
}

func TestQuoteInPathKeepsAssemblerFlags(t *testing.T) {
	// The assembler gets the flags the one-step compile gives it: -Wa,... and -Xassembler words
	// (this source needs the symbol that -Wa,--defsym defines) and the target flags (-m...).
	if runtime.GOOS == "windows" {
		t.Skip("a double quote is not allowed in a Windows file name")
	}
	cxx, calls := loggingCompiler(t, lookGCC(t))
	src := writeAsmSource(t, filepath.Join(t.TempDir(), `q"uote`), "need.cpp",
		"void f() {\n\tasm volatile(\".ifndef GOCPP_NEEDED\\n.error \\\"missing\\\"\\n.endif\");\n}\n")
	work := t.TempDir()
	b := &Builder{CXX: cxx, Work: work}
	args := []string{
		"-std=c++20", "-O0", "-march=x86-64", "-Wa,--defsym,GOCPP_NEEDED=1", "-Xassembler", "--noexecstack",
		"-c", src, "-o", filepath.Join(work, "need.o"),
	}
	if runtime.GOARCH != "amd64" {
		args = slices.DeleteFunc(args, func(a string) bool { return a == "-march=x86-64" })
	}
	if out, err := b.runOutput(context.Background(), cxx, "the test", false, args); err != nil || len(out) != 0 {
		t.Fatalf("compile = %q, %v", out, err)
	}
	got := calls()
	if len(got) != 2 {
		t.Fatalf("calls = %q, want the compile to assembly and the assembly", got)
	}
	want := []string{"-Wa,--defsym,GOCPP_NEEDED=1", "-Xassembler --noexecstack"}
	if runtime.GOARCH == "amd64" {
		want = append(want, "-march=x86-64")
	}
	for _, flag := range want {
		if !strings.Contains(got[1], flag) {
			t.Errorf("the assembling call %q lacks %q", got[1], flag)
		}
	}
	if strings.Contains(got[1], "-std=c++20") || strings.Contains(got[1], "-O0") {
		t.Errorf("the assembling call %q has the compile's flags", got[1])
	}
}

func TestQuoteInPathWithLTO(t *testing.T) {
	// A unit with a quote in its path is compiled without LTO (the marker would be written at the
	// link, out of reach), and the LTO link takes the plain object.
	if runtime.GOOS == "windows" {
		t.Skip("a double quote is not allowed in a Windows file name")
	}
	cxx, calls := loggingCompiler(t, lookGCC(t))
	dir := filepath.Join(t.TempDir(), `q"uote`)
	src := writeAsmSource(t, dir, "m.cpp", asmSource+"int main() { return nop() - 1; }\n")
	work := t.TempDir()
	b := &Builder{CXX: cxx, Work: work}
	obj, exe := filepath.Join(work, "m.o"), filepath.Join(work, "m")
	compile := []string{"-std=c++20", "-O2", "-flto", "-flto=4", "-ffat-lto-objects", "-c", src, "-o", obj}
	if out, err := b.runOutput(context.Background(), cxx, "the test", false, compile); err != nil || len(out) != 0 {
		t.Fatalf("compile = %q, %v", out, err)
	}
	for _, call := range calls() {
		if strings.Contains(call, "-flto") || strings.Contains(call, "lto-objects") {
			t.Errorf("the call %q has an LTO flag", call)
		}
	}
	if out, err := b.runOutput(context.Background(), cxx, "the test", true, []string{"-O2", "-flto", obj, "-o", exe}); err != nil || len(out) != 0 {
		t.Fatalf("link = %q, %v", out, err)
	}
	if err := exec.Command(exe).Run(); err != nil {
		t.Errorf("the program: %v", err)
	}
}

func TestNofortifyFlags(t *testing.T) {
	quoted := `/c/ca"ch/rt`
	plain := "/c/cache/rt"
	for _, c := range []struct {
		name string
		cxx  CXX
		dir  string
		want []string
	}{
		{"gcc, plain", CXX{Family: GCC}, plain, []string{"-include", plain + "/include/gocpp/nofortify.h"}},
		{"clang, plain", CXX{Family: Clang}, plain, []string{"-include", plain + "/include/gocpp/nofortify.h"}},
		{"gcc, quote: gcc escapes it", CXX{Family: GCC}, quoted, []string{"-include", quoted + "/include/gocpp/nofortify.h"}},
		{"clang, quote: by name through -iquote", CXX{Family: Clang}, quoted,
			[]string{"-iquote", quoted + "/include", "-include", "gocpp/nofortify.h"}},
	} {
		if runtime.GOOS == "windows" {
			t.Skip("paths of this test are slash-separated")
		}
		if got := nofortifyFlags(c.cxx, c.dir); !slices.Equal(got, c.want) {
			t.Errorf("%s: nofortifyFlags = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestCompilerEnvEmptyNixResponseFileIsUnset(t *testing.T) {
	t.Setenv("NIX_CC_USE_RESPONSE_FILE", "")
	last := ""
	for _, kv := range CompilerEnv() {
		if strings.HasPrefix(kv, "NIX_CC_USE_RESPONSE_FILE=") {
			last = kv
		}
	}
	if last != "NIX_CC_USE_RESPONSE_FILE=0" {
		t.Errorf("last NIX_CC_USE_RESPONSE_FILE entry = %q, want =0 (an empty value is not a choice)", last)
	}
}
