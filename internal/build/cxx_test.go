package build

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The tests use fake compilers: small shell scripts that print what a
// compiler prints for --version and -dM -E. The real compilers are tested
// by the namespace probe's tests (internal/interop), which follow the
// test harness's rules for g++ and clang++.

// The macro lines of the fake compilers' -dM -E.
var (
	clangMacros = []string{"#define __GNUC__ 4", "#define __clang__ 1", `#define __VERSION__ "Clang 99.0.0"`}
	gccMacros   = []string{"#define __GNUC__ 99", `#define __VERSION__ "99.1.0"`}
)

// fakeCXX writes a fake compiler into dir and returns its path. It prints
// version for --version and the lines macros for -dM, and fails for
// anything else. Each run adds a line to the file <path>.log. It uses only
// shell builtins, so it works with any PATH.
func fakeCXX(t *testing.T, dir, name, version string, macros ...string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake compilers are shell scripts")
	}
	var b strings.Builder
	b.WriteString("#!/bin/sh\necho \"$*\" >> \"$0.log\"\nfor a in \"$@\"; do\n\tcase \"$a\" in\n")
	if version != "" {
		b.WriteString("\t--version) printf '%s\\n' " + shellQuote(version) + "; exit 0;;\n")
	}
	b.WriteString("\t-dM)")
	for _, m := range macros {
		b.WriteString(" printf '%s\\n' " + shellQuote(m) + ";")
	}
	b.WriteString(" exit 0;;\n\tesac\ndone\necho \"fake: bad arguments\" >&2\nexit 3\n")
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(b.String()), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// shellQuote quotes s for sh.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// runsOf returns how often the fake compiler at path ran.
func runsOf(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path + ".log")
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(data), "\n")
}

// setEnv sets PATH to path and GOCPP_CXX and CXX to the given values ("" is
// the same as not set) for the rest of the test.
func setEnv(t *testing.T, path, gocppCXX, cxx string) {
	t.Setenv("PATH", path)
	t.Setenv("GOCPP_CXX", gocppCXX)
	t.Setenv("CXX", cxx)
}

func TestFindCXXOrder(t *testing.T) {
	dir := t.TempDir()
	clang := fakeCXX(t, dir, "clang++", "clang version 99.0.0", clangMacros...)
	gcc := fakeCXX(t, dir, "g++", "g++ (GCC) 99.1.0", gccMacros...)
	mine := fakeCXX(t, dir, "my-c++", "my c++ 1.0", gccMacros...)
	onlyGCC := t.TempDir()
	gccAlone := fakeCXX(t, onlyGCC, "g++", "g++ (GCC) 99.1.0", gccMacros...)

	for _, c := range []struct {
		name            string
		path            string // PATH
		gocppCXX, cxx   string
		want            string
		family          Family
		version         string
		fromEnvironment bool
	}{
		{"GOCPP_CXX wins", dir, "my-c++", "g++", mine, GCC, "my c++ 1.0", true},
		{"GOCPP_CXX path", dir, mine, "clang++", mine, GCC, "my c++ 1.0", true},
		{"then CXX", dir, "", "g++", gcc, GCC, "g++ (GCC) 99.1.0", true},
		{"CXX path", dir, "", clang, clang, Clang, "clang version 99.0.0", true},
		{"then clang++", dir, "", "", clang, Clang, "clang version 99.0.0", false},
		{"then g++", onlyGCC, "", "", gccAlone, GCC, "g++ (GCC) 99.1.0", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			setEnv(t, c.path, c.gocppCXX, c.cxx)
			got, err := FindCXX(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			want := CXX{Path: c.want, Family: c.family, Version: c.version}
			if got != want {
				t.Errorf("FindCXX() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestFindCXXMissing(t *testing.T) {
	dir := t.TempDir()
	fakeCXX(t, dir, "clang++", "clang version 99.0.0", clangMacros...)
	empty := t.TempDir()
	for _, c := range []struct {
		name          string
		path          string
		gocppCXX, cxx string
		want          []string // parts of the error message
	}{
		{"no compiler", empty, "", "", []string{"no C++ compiler found", "clang++ and g++", "GOCPP_CXX"}},
		// A compiler that was chosen is never replaced by clang++ from PATH.
		{"GOCPP_CXX missing", dir, "clang++-404", "", []string{"GOCPP_CXX: ", `"clang++-404" was not found`}},
		{"GOCPP_CXX path missing", dir, filepath.Join(empty, "c++"), "", []string{"GOCPP_CXX: ", "not found", filepath.Join(empty, "c++")}},
		{"CXX missing", dir, "", "g++-404", []string{"CXX: ", `"g++-404" was not found`}},
		{"CXX with arguments", dir, "", "ccache clang++", []string{"CXX: ", `"ccache clang++" was not found (it must be one program, without arguments, for now)`}},
	} {
		t.Run(c.name, func(t *testing.T) {
			setEnv(t, c.path, c.gocppCXX, c.cxx)
			got, err := FindCXX(context.Background())
			if err == nil {
				t.Fatalf("FindCXX() = %+v, want an error", got)
			}
			for _, w := range c.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("FindCXX() error = %q, want it to contain %q", err, w)
				}
			}
		})
	}
}

func TestFindCXXSkipsBrokenDefault(t *testing.T) {
	// Without GOCPP_CXX and CXX, a clang++ that doesn't work is skipped
	// for g++; when none works, the error says why for each.
	dir := t.TempDir()
	fakeCXX(t, dir, "clang++", "", clangMacros...) // fails --version
	gcc := fakeCXX(t, dir, "g++", "g++ (GCC) 99.1.0", gccMacros...)
	setEnv(t, dir, "", "")
	got, err := FindCXX(context.Background())
	if err != nil || got.Path != gcc {
		t.Errorf("FindCXX() = %+v, %v; want g++ at %s", got, err, gcc)
	}
	fakeCXX(t, dir, "g++", "", gccMacros...) // now g++ fails too (another size)
	got, err = FindCXX(context.Background())
	if err == nil {
		t.Fatalf("FindCXX() = %+v, want an error", got)
	}
	for _, w := range []string{"no working C++ compiler found", "GOCPP_CXX", filepath.Join(dir, "clang++") + ": --version failed", filepath.Join(dir, "g++") + ": --version failed"} {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("FindCXX() error = %q, want it to contain %q", err, w)
		}
	}
}

func TestLookCXXFamily(t *testing.T) {
	dir := t.TempDir()
	setEnv(t, dir, "", "")
	for _, c := range []struct {
		name, version string
		macros        []string
		family        Family
		err           string // part of the error message, if it must fail
	}{
		{name: "clang-like", version: "Ubuntu clang version 18.1.3 (1ubuntu1)", macros: clangMacros, family: Clang},
		// A compiler built on clang that doesn't say "clang" in its version line.
		{name: "icpx-like", version: "Intel(R) oneAPI DPC++/C++ Compiler 2024.0.0", macros: clangMacros, family: Clang},
		{name: "gcc-like", version: "x86_64-w64-mingw32-g++ (GCC) 13.2.0", macros: gccMacros, family: GCC},
		{name: "neither", version: "other c++ 1.0", macros: []string{"#define __OTHER__ 1"}, err: "is neither clang nor gcc"},
		{name: "no-version", macros: gccMacros, err: "--version failed: exit status 3: fake: bad arguments"},
		{name: "blank-version", version: " ", macros: gccMacros, err: "printed nothing for --version"},
	} {
		t.Run(c.name, func(t *testing.T) {
			path := fakeCXX(t, dir, c.name, c.version, c.macros...)
			got, err := LookCXX(context.Background(), c.name)
			if c.err != "" {
				if err == nil || !strings.Contains(err.Error(), c.err) {
					t.Fatalf("LookCXX(%q) = %+v, %v; want an error with %q", c.name, got, err, c.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := CXX{Path: path, Family: c.family, Version: c.version}
			if got != want {
				t.Errorf("LookCXX(%q) = %+v, want %+v", c.name, got, want)
			}
		})
	}
}

func TestLookCXXCache(t *testing.T) {
	dir := t.TempDir()
	setEnv(t, dir, "", "")
	path := fakeCXX(t, dir, "c++", "c++ 1.0", gccMacros...)
	ctx := context.Background()
	first, err := LookCXX(ctx, "c++")
	if err != nil {
		t.Fatal(err)
	}
	if n := runsOf(t, path); n != 2 {
		t.Errorf("the first LookCXX ran the compiler %d times, want 2 (--version and -dM -E)", n)
	}
	again, err := LookCXX(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if n := runsOf(t, path); n != 2 || again != first {
		t.Errorf("LookCXX again: %+v and %d runs, want %+v from the cache (2 runs)", again, n, first)
	}

	// A new compiler at the same path (another size) is looked at again.
	fakeCXX(t, dir, "c++", "c++ 2.0 (a longer version line)", clangMacros...)
	got, err := LookCXX(ctx, "c++")
	if err != nil {
		t.Fatal(err)
	}
	want := CXX{Path: path, Family: Clang, Version: "c++ 2.0 (a longer version line)"}
	if got != want {
		t.Errorf("LookCXX after the compiler changed = %+v, want %+v", got, want)
	}

	// A profile link that now points at another compiler is noticed too.
	other := fakeCXX(t, dir, "other", "other 1.0", gccMacros...)
	link := filepath.Join(dir, "link-c++")
	if err := os.Symlink(path, link); err != nil {
		t.Skip("no symbolic links:", err)
	}
	if got, err := LookCXX(ctx, link); err != nil || got.Version != want.Version {
		t.Fatalf("LookCXX(link) = %+v, %v; want version %q", got, err, want.Version)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, link); err != nil {
		t.Fatal(err)
	}
	if got, err := LookCXX(ctx, link); err != nil || got.Version != "other 1.0" || got.Path != link {
		t.Errorf("LookCXX(link) after the link changed = %+v, %v; want version \"other 1.0\" at %s", got, err, link)
	}
}

func TestLookCXXRelativePath(t *testing.T) {
	dir := t.TempDir()
	setEnv(t, dir, "", "")
	path := fakeCXX(t, dir, "c++", "c++ 1.0", gccMacros...)
	t.Chdir(dir)
	got, err := LookCXX(context.Background(), "./c++")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got.Path) {
		t.Errorf("LookCXX(\"./c++\").Path = %q, want an absolute path", got.Path)
	}
	fi1, err1 := os.Stat(got.Path)
	fi2, err2 := os.Stat(path)
	if err1 != nil || err2 != nil || !os.SameFile(fi1, fi2) {
		t.Errorf("LookCXX(\"./c++\").Path = %q, want %q", got.Path, path)
	}
}

func TestLookCXXCanceled(t *testing.T) {
	dir := t.TempDir()
	setEnv(t, dir, "", "")
	fakeCXX(t, dir, "c++", "c++ 1.0", gccMacros...)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := LookCXX(ctx, "c++"); err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("LookCXX with a canceled context = %+v, %v; want a context error", got, err)
	}
}

func TestLookCXXEnvironment(t *testing.T) {
	// The detection runs get CompilerEnv: LC_ALL=C, and none of the
	// variables that make a compiler write files of its own.
	if runtime.GOOS == "windows" {
		t.Skip("the fake compilers are shell scripts")
	}
	dir := t.TempDir()
	setEnv(t, dir, "", "")
	t.Setenv("LC_ALL", "de_DE.UTF-8")
	t.Setenv("DEPENDENCIES_OUTPUT", filepath.Join(dir, "deps.d"))
	t.Setenv("CC_PRINT_HEADERS", "1")
	script := `#!/bin/sh
case "$1" in
--version) echo "env LC_ALL=$LC_ALL DEPENDENCIES_OUTPUT=${DEPENDENCIES_OUTPUT-unset} CC_PRINT_HEADERS=${CC_PRINT_HEADERS-unset}";;
-dM) echo "#define __GNUC__ 99";;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "c++"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := LookCXX(context.Background(), "c++")
	want := "env LC_ALL=C DEPENDENCIES_OUTPUT=unset CC_PRINT_HEADERS=unset"
	if err != nil || got.Version != want {
		t.Errorf("LookCXX = %+v, %v; want the version line %q", got, err, want)
	}
}

func TestFamilyString(t *testing.T) {
	for f, want := range map[Family]string{Clang: "clang", GCC: "gcc", 0: "Family(0)"} {
		if got := f.String(); got != want {
			t.Errorf("Family(%d).String() = %q, want %q", int(f), got, want)
		}
	}
}
