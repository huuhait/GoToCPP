package build_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/build/testlib"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// loadPkg writes the files into a new folder, loads the package there and
// returns it, with its FileSet. The package must load without errors.
func loadPkg(t *testing.T, files map[string]string) (*load.Package, *load.Program) {
	t.Helper()
	dir := filepath.Clean(t.TempDir())
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prog := load.Load(load.Config{Dir: dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}, ".")
	lp := prog.Roots[0]
	if lp.Errors.HasErrors() {
		t.Fatalf("load errors: %v", lp.Errors.Diagnostics())
	}
	return lp, prog
}

// errText returns the errors as "file:line:col: msg" lines, with the
// file's base name.
func errText(ds []diag.Diagnostic) string {
	var b strings.Builder
	for _, d := range ds {
		fmt.Fprintf(&b, "%s:%d:%d: %s\n", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Pos.Column, d.Msg)
	}
	return b.String()
}

// A fakePkgConfig answers pkg-config runs from a table, and records them.
type fakePkgConfig struct {
	mu    sync.Mutex
	runs  [][]string
	flags map[string][2]string // package → cflags, libs (split at spaces)
}

func (f *fakePkgConfig) run(_ context.Context, dir string, pkgs []string) (cflags, libs []string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runs = append(f.runs, slices.Clone(pkgs))
	for _, p := range pkgs {
		fl, ok := f.flags[p]
		if !ok {
			return nil, nil, fmt.Errorf("pkg-config --cflags -- %s: exit status 1\n\tPackage %s was not found", strings.Join(pkgs, " "), p)
		}
		cflags = append(cflags, strings.Fields(fl[0])...)
		libs = append(libs, strings.Fields(fl[1])...)
	}
	return cflags, libs, nil
}

// TestReadPackageFlags checks the order of the flags: source order, files
// by name, each kind of directive in its place; relative -I and -L paths
// become absolute; //gocpp:link adds -l.
func TestReadPackageFlags(t *testing.T) {
	lp, prog := loadPkg(t, map[string]string{
		"a.gp": `//gocpp:cflags -DA -Iinclude
//gocpp:link m
//gocpp:pkg-config p1 p2
//gocpp:cflags -DA2 '-DMSG="a b"'
package main

func main() {}
`,
		"b.gp": `//gocpp:ldflags -L lib -lq
//gocpp:pkg-config p1
//gocpp:link pthread
//gocpp:cflags -DB
package main
`,
	})
	pc := &fakePkgConfig{flags: map[string][2]string{
		"p1": {"-I/p1/include", "-L/p1/lib -lp1"},
		"p2": {"-DP2", "-lp2"},
	}}
	flags, errs, pending := build.ReadPackageFlags(context.Background(), prog.Fset, lp, pc.run)
	if len(errs) > 0 || pending {
		t.Fatalf("errors:\n%s pending %v", errText(errs), pending)
	}
	wantCompile := []string{"-DA", "-I" + filepath.Join(lp.Dir, "include"), "-I/p1/include", "-DP2", "-DA2", `-DMSG="a b"`, "-I/p1/include", "-DB"}
	wantLink := []string{"-lm", "-L/p1/lib", "-lp1", "-lp2", "-L", filepath.Join(lp.Dir, "lib"), "-lq", "-L/p1/lib", "-lp1", "-lpthread"}
	if !slices.Equal(flags.Compile, wantCompile) {
		t.Errorf("Compile =\n%q\nwant\n%q", flags.Compile, wantCompile)
	}
	if !slices.Equal(flags.Link, wantLink) {
		t.Errorf("Link =\n%q\nwant\n%q", flags.Link, wantLink)
	}
	if want := [][]string{{"p1", "p2"}, {"p1"}}; !slices.EqualFunc(pc.runs, want, slices.Equal) {
		t.Errorf("pkg-config runs %q, want one per line: %q", pc.runs, want)
	}

	// No directives: no flags, and pkg-config doesn't run.
	lp, prog = loadPkg(t, map[string]string{"a.gp": "package main\n\nfunc main() {}\n"})
	pc.runs = nil
	if flags, errs, pending := build.ReadPackageFlags(context.Background(), prog.Fset, lp, pc.run); flags.Compile != nil || flags.Link != nil || errs != nil || pending || pc.runs != nil {
		t.Errorf("without directives: %+v, %v, %v, runs %q; want nothing", flags, errs, pending, pc.runs)
	}
}

// TestReadPackageFlagsErrors checks the errors: at the directive, the
// whole line's flags dropped, the other lines kept; the allow-list for the
// directives and for the output of pkg-config; the overrides.
func TestReadPackageFlagsErrors(t *testing.T) {
	lp, prog := loadPkg(t, map[string]string{
		"a.gp": `//gocpp:cflags -DOK
//gocpp:cflags -DX -fplugin=./evil.so
//gocpp:ldflags -Wl,-plugin,x.so
//gocpp:ldflags -lok
//gocpp:link to_library
//gocpp:pkg-config evil
//gocpp:pkg-config evillibs
//gocpp:pkg-config gocpp-no-such-lib
//gocpp:cflags -I
//gocpp:cflags -B /tmp
package main

func main() {}
`,
	})
	pc := &fakePkgConfig{flags: map[string][2]string{
		"evil":     {"-I/evil/include -fplugin=/evil/x.so", "-levil"},
		"evillibs": {"-I/evil/include", "-Wl,-plugin=/evil/ld.so"},
	}}
	flags, errs, pending := build.ReadPackageFlags(context.Background(), prog.Fset, lp, pc.run)
	want := `a.gp:2:1: invalid flag in //gocpp:cflags: -fplugin=./evil.so
a.gp:3:1: invalid flag in //gocpp:ldflags: -Wl,-plugin,x.so
a.gp:5:1: invalid flag in //gocpp:link: -lto_library
a.gp:6:1: invalid flag in pkg-config --cflags: -fplugin=/evil/x.so
a.gp:7:1: invalid flag in pkg-config --libs: -Wl,-plugin=/evil/ld.so
a.gp:8:1: pkg-config --cflags -- gocpp-no-such-lib: exit status 1
	Package gocpp-no-such-lib was not found
a.gp:9:1: invalid flag in //gocpp:cflags: -I without argument
a.gp:10:1: invalid flag in //gocpp:cflags: -B (a package can't choose the system root or the toolchain: they are choices for the whole program; set them in GOCPP_CFLAGS or GOCPP_LDFLAGS; $GOCPP_CFLAGS_ALLOW can allow it)
`
	if got := errText(errs); got != want || pending {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
	if !slices.Equal(flags.Compile, []string{"-DOK"}) || !slices.Equal(flags.Link, []string{"-lok"}) {
		t.Errorf("flags %+v, want only those of the good lines", flags)
	}
	if errs[0].End.Column != len("//gocpp:cflags")+1 {
		t.Errorf("the error ends at %v, want after //gocpp:cflags", errs[0].End)
	}

	// GOCPP_CFLAGS_ALLOW and GOCPP_LDFLAGS_ALLOW allow more, the DISALLOW
	// variables refuse more.
	t.Setenv("GOCPP_CFLAGS_ALLOW", `-fplugin=.*`)
	t.Setenv("GOCPP_LDFLAGS_ALLOW", `-Wl,-plugin.*`)
	t.Setenv("GOCPP_LDFLAGS_DISALLOW", `-lok`)
	_, errs, _ = build.ReadPackageFlags(context.Background(), prog.Fset, lp, pc.run)
	want = `a.gp:4:1: invalid flag in //gocpp:ldflags: -lok
a.gp:5:1: invalid flag in //gocpp:link: -lto_library
a.gp:8:1: pkg-config --cflags -- gocpp-no-such-lib: exit status 1
	Package gocpp-no-such-lib was not found
a.gp:9:1: invalid flag in //gocpp:cflags: -I without argument
a.gp:10:1: invalid flag in //gocpp:cflags: -B (a package can't choose the system root or the toolchain: they are choices for the whole program; set them in GOCPP_CFLAGS or GOCPP_LDFLAGS; $GOCPP_CFLAGS_ALLOW can allow it)
`
	if got := errText(errs); got != want {
		t.Errorf("with the overrides: errors:\n%s\nwant:\n%s", got, want)
	}
	t.Setenv("GOCPP_LDFLAGS_DISALLOW", `(`)
	_, errs, _ = build.ReadPackageFlags(context.Background(), prog.Fset, lp, pc.run)
	if got := errText(errs); !strings.Contains(got, "a.gp:3:1: parsing $GOCPP_LDFLAGS_DISALLOW: error parsing regexp") {
		t.Errorf("a bad GOCPP_LDFLAGS_DISALLOW: errors:\n%s", got)
	}

	// A PkgConfigFunc that answers later: pending, no error.
	pending = false
	_, errs, pending = build.ReadPackageFlags(context.Background(), prog.Fset, lp, func(context.Context, string, []string) ([]string, []string, error) {
		return nil, nil, build.ErrPending
	})
	if !pending || strings.Contains(errText(errs), "pkg-config") {
		t.Errorf("pending %v, errors:\n%s\nwant pending and no pkg-config errors", pending, errText(errs))
	}
}

// TestPkgConfig runs the real pkg-config with gptestlib's .pc file, in a
// folder of PKG_CONFIG_PATH: its flags, a package that isn't there, a .pc
// file with a flag that the allow-list refuses, PKG_CONFIG, and no
// pkg-config at all.
func TestPkgConfig(t *testing.T) {
	testlib.NeedPkgConfig(t)
	prefix := filepath.Clean(t.TempDir())
	pcDir := testlib.Install(t, prefix, nil)
	evil := "Name: evil\nDescription: x\nVersion: 1\nCflags: -fplugin=/evil.so\nLibs:\n"
	if err := os.WriteFile(filepath.Join(pcDir, "gptestlib-evil.pc"), []byte(evil), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PKG_CONFIG_PATH", pcDir)
	t.Setenv("PKG_CONFIG", "")
	ctx := context.Background()
	// pkg-config (freedesktop) puts -D before -I, pkgconf doesn't.
	sorted := func(s []string) []string { return slices.Sorted(slices.Values(s)) }
	cflags, libs, err := build.PkgConfig(ctx, prefix, []string{"gptestlib"})
	wantC := []string{"-DGPTESTLIB_ON", "-I" + filepath.Join(prefix, "include")}
	wantL := []string{"-L" + filepath.Join(prefix, "lib"), "-lgptestlib"}
	if err != nil || !slices.Equal(sorted(cflags), wantC) || !slices.Equal(libs, wantL) {
		t.Errorf("PkgConfig = %q, %q, %v; want %q, %q", cflags, libs, err, wantC, wantL)
	}

	lp, prog := loadPkg(t, map[string]string{
		"a.gp": `//gocpp:pkg-config gptestlib
//gocpp:pkg-config gocpp-no-such-lib
//gocpp:pkg-config gptestlib-evil
package main

func main() {}
`,
	})
	flags, errs, _ := build.ReadPackageFlags(ctx, prog.Fset, lp, nil)
	if !slices.Equal(sorted(flags.Compile), wantC) || !slices.Equal(flags.Link, wantL) {
		t.Errorf("flags %+v, want those of gptestlib", flags)
	}
	got := errText(errs)
	rx := regexp.MustCompile(`^a\.gp:2:1: pkg-config --cflags -- gocpp-no-such-lib: exit status 1\n(\t.*\n)*?\t.*gocpp-no-such-lib.*\n(\t.*\n)*` +
		`a\.gp:3:1: invalid flag in pkg-config --cflags: -fplugin=/evil\.so\n$`)
	if !rx.MatchString(got) {
		t.Errorf("errors:\n%s\nwant to match:\n%s", got, rx)
	}

	// PKG_CONFIG names the program: its first word, like the go command.
	t.Setenv("PKG_CONFIG", filepath.Join(prefix, "no-such-pkg-config")+" --static")
	_, _, err = build.PkgConfig(ctx, prefix, []string{"gptestlib"})
	if err == nil || !strings.HasPrefix(err.Error(), "cannot run "+filepath.Join(prefix, "no-such-pkg-config")+": ") {
		t.Errorf("PKG_CONFIG of a program that isn't there: %v", err)
	}
	// The messages show the program that PKG_CONFIG names.
	wrapper := filepath.Join(prefix, "mypc")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec pkg-config \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PKG_CONFIG", wrapper)
	_, _, err = build.PkgConfig(ctx, prefix, []string{"gocpp-no-such-lib"})
	if err == nil || !strings.HasPrefix(err.Error(), wrapper+" --cflags -- gocpp-no-such-lib: exit status 1\n") {
		t.Errorf("PKG_CONFIG=%s: %v", wrapper, err)
	}
	var trace []string
	if _, _, err = build.TracePkgConfig(func(line string) { trace = append(trace, line) })(ctx, prefix, []string{"gptestlib"}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"cd " + prefix, wrapper + " --cflags -- gptestlib", "cd " + prefix, wrapper + " --libs -- gptestlib"}; !slices.Equal(trace, want) {
		t.Errorf("TracePkgConfig gave %q, want %q", trace, want)
	}
	t.Setenv("PKG_CONFIG", "'pkg-config")
	if _, _, err = build.PkgConfig(ctx, prefix, []string{"gptestlib"}); err == nil || err.Error() != "PKG_CONFIG: unterminated ' string" {
		t.Errorf("a bad PKG_CONFIG: %v", err)
	}
	t.Setenv("PKG_CONFIG", "")
	t.Setenv("PATH", t.TempDir())
	_, errs, _ = build.ReadPackageFlags(ctx, prog.Fset, lp, nil)
	if got, want := errText(errs[:1]), "a.gp:1:1: cannot run pkg-config: exec: \"pkg-config\": executable file not found in $PATH\n"; got != want {
		t.Errorf("without pkg-config: %s, want %s", got, want)
	}
}

// TestPkgConfigCanceled checks that a canceled context stops pkg-config.
func TestPkgConfigCanceled(t *testing.T) {
	testlib.NeedPkgConfig(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := build.PkgConfig(ctx, t.TempDir(), []string{"gptestlib"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("PkgConfig with a canceled context: %v, want context.Canceled", err)
	}
}

// TestReadEnvFlags checks GOCPP_CFLAGS, GOCPP_CXXFLAGS and GOCPP_LDFLAGS:
// split like CC, quotes keep spaces, not checked by the allow-list. The probes of the headers
// get no plugin flag (TestProbeFlagsDropPlugins).
func TestReadEnvFlags(t *testing.T) {
	t.Setenv("GOCPP_CFLAGS", `-DA '-DB=x y' -fplugin=mine.so`)
	t.Setenv("GOCPP_CXXFLAGS", "-stdlib=libc++")
	t.Setenv("GOCPP_LDFLAGS", "-fuse-ld=mold")
	env, err := build.ReadEnvFlags()
	if err != nil {
		t.Fatal(err)
	}
	want := build.EnvFlags{CFlags: []string{"-DA", "-DB=x y", "-fplugin=mine.so"}, CXXFlags: []string{"-stdlib=libc++"}, LDFlags: []string{"-fuse-ld=mold"}}
	if !slices.Equal(env.CFlags, want.CFlags) || !slices.Equal(env.CXXFlags, want.CXXFlags) || !slices.Equal(env.LDFlags, want.LDFlags) {
		t.Errorf("ReadEnvFlags = %+v, want %+v", env, want)
	}
	probe := build.ProbeFlags(env, build.PackageFlags{Compile: []string{"-DPKG"}, Link: []string{"-lpkg"}})
	if want := slices.Concat([]string{"-DA", "-DB=x y", "-stdlib=libc++", "-DPKG"}, build.LanguageFlags()); !slices.Equal(probe, want) {
		t.Errorf("ProbeFlags = %q, want %q", probe, want)
	}
	t.Setenv("GOCPP_CXXFLAGS", `"-DA`)
	if _, err := build.ReadEnvFlags(); err == nil || err.Error() != `GOCPP_CXXFLAGS: unterminated " string` {
		t.Errorf("an unterminated quote: %v", err)
	}
}

// languageSrc fails to compile, or prints something else than "0 0 0",
// when one of gocpp's language flags is off: C++20, exceptions, no fast
// math (NaN == NaN is false), no strict aliasing (at -O2, alias reads the
// int again after the float store) and no contraction (with FMA, a*b+c
// would be one fused operation, and contract gives 2^-54, not 0).
const languageSrc = `#include <gocpp/rt.hpp>
#include <cstdio>
#if __cplusplus != 202002L
#error "not C++20"
#endif
#ifdef __FAST_MATH__
#error "fast math"
#endif
#ifndef __cpp_exceptions
#error "no exceptions"
#endif
__attribute__((noinline)) int alias(int* i, float* f) { *i = 1; *f = 0; return *i; }
__attribute__((noinline)) double contract(double a, double b, double c) { return a * b + c; }
int shadow() {
    int n = 0;
    for (int i = 0; i < 2; i++) {
        int i2 = i;
        { int i = i2; n += i; } // -Wshadow warns: valid Go (i := i) gives this
    }
    return n;
}
int main() {
    if (shadow() != 1) return 1;
    volatile double z = 0;
    double n = z / z;
    int x;
    int r = alias(&x, reinterpret_cast<float*>(&x));
    double a = 1.0 + 0x1p-27;
    std::printf("%d %d %g\n", n == n, r, contract(a, a, -(1.0 + 0x1p-26)));
}
`

// TestLanguageFlagsWin checks that a package's flags can't turn off
// gocpp's language flags (build.LanguageFlags) in the compile of the
// generated C++, with g++ and clang++: -std=c++17, -x c, -fno-exceptions,
// -ffast-math, -fstrict-aliasing and -ffp-contract=fast (with -O2, which a
// package may choose) lose to them, and -Werror -Wshadow doesn't fail
// valid code. Where the CPU has FMA, -mfma lets the compilers fuse a*b+c,
// so that -ffp-contract=off shows. (Each of these flags alone fails the
// test when the language flags are left out.) The same flags in
// GOCPP_CXXFLAGS reach the runtime's compiles, with the language flags
// after them. A class object passed through a C function's ... is still
// an error with these flags (GeneratedFlags, runtime/reject_test.go).
func TestLanguageFlagsWin(t *testing.T) {
	bad := []string{"-std=c++17", "-x", "c", "-fno-exceptions", "-ffast-math", "-fstrict-aliasing", "-ffp-contract=fast", "-O2", "-Werror", "-Wshadow"}
	if cpu, err := os.ReadFile("/proc/cpuinfo"); err == nil && regexp.MustCompile(`(?m)^flags\s*:.* fma( |$)`).Match(cpu) {
		bad = append(bad, "-mfma")
	}
	for _, name := range []string{"g++", "clang++"} {
		t.Run(name, func(t *testing.T) {
			cxx, err := build.LookCXX(context.Background(), name)
			if err != nil {
				t.Skip(err)
			}
			work := t.TempDir()
			src := filepath.Join(work, "p.cpp")
			if err := os.WriteFile(src, []byte(languageSrc), 0o644); err != nil {
				t.Fatal(err)
			}
			var trace strings.Builder
			b := &build.Builder{CXX: cxx, GOOS: "linux", Work: work, Cache: t.TempDir(), Trace: &trace,
				Env: build.EnvFlags{CXXFlags: slices.Clone(bad)}, Package: build.PackageFlags{Compile: bad}}
			exe := filepath.Join(work, "p")
			if err := b.Program(context.Background(), src, exe); err != nil {
				var ce *build.CompileError
				if errors.As(err, &ce) {
					t.Fatalf("%v\n%s\n%s", ce, ce.Command, ce.Output)
				}
				t.Fatal(err)
			}
			if out, err := exec.Command(exe).Output(); err != nil || string(out) != "0 0 0\n" {
				t.Errorf("the program printed %q, %v; want \"0 0 0\\n\"", out, err)
			}
			// A runtime compile has the language flags after the
			// environment's, then -Wno-error and -x c++.
			after := strings.Join(slices.Concat(bad, build.LanguageFlags(), []string{"-Wno-error", "-x", "c++"}), " ") + " -I "
			if !regexp.MustCompile(`(?m)` + regexp.QuoteMeta(after) + `\S+/include -c \S+/src/\S+\.cpp `).MatchString(trace.String()) {
				t.Errorf("no runtime compile with %q in:\n%s", after, trace.String())
			}

			// A gp::ptr through ... is still an error.
			vsrc := filepath.Join(work, "v.cpp")
			varargs := "#include <gocpp/rt.hpp>\nextern \"C\" void var(int, ...);\nvoid f(gp::ptr<int64_t> p) { var(1, p); }\nint main() {}\n"
			if err := os.WriteFile(vsrc, []byte(varargs), 0o644); err != nil {
				t.Fatal(err)
			}
			var ce *build.CompileError
			if err := b.Program(context.Background(), vsrc, filepath.Join(work, "v")); !errors.As(err, &ce) || ce.Link ||
				!regexp.MustCompile(`v\.cpp:3:\d+: error: `).Match(ce.Output) {
				t.Errorf("a gp::ptr passed through ... with the flags %q: %v, want a compile error at v.cpp:3", bad, err)
			}

			// The probes get them last too.
			probe := build.ProbeFlags(build.EnvFlags{CXXFlags: []string{"-std=c++17"}}, build.PackageFlags{Compile: bad})
			if !slices.Equal(probe[len(probe)-len(build.LanguageFlags()):], build.LanguageFlags()) {
				t.Errorf("ProbeFlags = %q, want LanguageFlags at the end", probe)
			}
		})
	}
}
