package cppgen

import (
	"bufio"
	"context"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/lower"
)

// The tests of the reserved names (internal/lower, names.go) with the real compilers.

// goNames returns the reserved names that a Go program can declare (identifiers that are not
// Go keywords), sorted.
func goNames() []string {
	var names []string
	for _, r := range lower.ReservedNames() {
		if token.IsIdentifier(r) {
			names = append(names, r)
		}
	}
	return names
}

// listFile returns the path of a list of internal/lower: cxx_words.txt or cxx_macros.txt.
func listFile(name string) string {
	return filepath.Join("..", "lower", name)
}

// readList returns the words of a list of internal/lower (listFile): one per line, without
// the empty lines and the comment lines (which start with #), as names.go reads them.
func readList(t *testing.T, name string) []string {
	t.Helper()
	data, err := os.ReadFile(listFile(name))
	if err != nil {
		t.Fatal(err)
	}
	var words []string
	for l := range strings.SplitSeq(string(data), "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			words = append(words, l)
		}
	}
	return words
}

// uses returns C++ that declares and uses the name name in three ways, each in its own
// namespace (with the suffix id): as a package variable, as a function that is called, and as
// a local variable. typ is the integer type to use.
func uses(name, id, typ string) string {
	return fmt.Sprintf("namespace gpkg_v%[2]s { %[3]s %[1]s = 0; %[3]s gp_after = 1; }\n"+
		"namespace gpkg_f%[2]s { %[3]s %[1]s(%[3]s x); %[3]s gp_use() { return %[1]s(1); } }\n"+
		"namespace gpkg_l%[2]s { %[3]s gp_f() { %[3]s %[1]s = 1; return %[1]s; } }\n", name, id, typ)
}

// TestWordsEscaped compiles a use of the escaped form of every reserved name that a Go program
// can declare, and of the name with one to three more _, with the runtime's header, with each
// compiler.
func TestWordsEscaped(t *testing.T) {
	var b strings.Builder
	b.WriteString("#include <gocpp/rt.hpp>\n")
	for i, r := range goNames() {
		for k := range 4 { // the name and the next names of its family
			b.WriteString(uses(lower.Escape(r+strings.Repeat("_", k)), fmt.Sprintf("%d_%d", i, k), "int64_t"))
		}
	}
	src := filepath.Join(t.TempDir(), "escaped.cpp")
	if err := os.WriteFile(src, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range compilers(t) {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			ok, out, err := c.syntaxOK(src, includeFlag(t))
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				t.Errorf("the escaped names don't compile:\n%s", firstLines(out, 30))
			}
		})
	}
}

// preprocessorOnly are words of cxx_words.txt that compile as names: g++ only warns about them.
var preprocessorOnly = map[string]bool{"__VA_ARGS__": true, "__VA_OPT__": true}

// TestWordsRaw checks that every word of internal/lower/cxx_words.txt that a Go program can declare really
// can't be a name: g++ or clang++ rejects one of its uses (each word is a keyword of at least
// one of them). It needs both compilers.
func TestWordsRaw(t *testing.T) {
	both := bothCompilers(t)
	if both == nil {
		t.Skip("needs both g++ and clang++")
	}
	dir := t.TempDir()
	var words []string
	for _, w := range readList(t, "cxx_words.txt") {
		if token.IsIdentifier(w) && !preprocessorOnly[w] {
			words = append(words, w)
		}
	}
	errs := make([]string, len(words))
	parallel(len(words), func(i int) {
		src := filepath.Join(dir, fmt.Sprintf("w%d.cpp", i))
		if err := os.WriteFile(src, []byte(uses(words[i], "", "long")), 0o644); err != nil {
			errs[i] = err.Error()
			return
		}
		for _, c := range both {
			ok, _, err := c.syntaxOK(src)
			if err != nil {
				errs[i] = err.Error()
				return
			}
			if !ok {
				return
			}
		}
		errs[i] = words[i] + ": both compilers accept it as a name; run internal/lower/mkwords.sh"
	})
	for _, e := range errs {
		if e != "" {
			t.Error(e)
		}
	}
}

// rawMacros are macros of the runtime's headers, and names that the generated code uses
// unqualified, that break a use of the raw name with every compiler. (Not every macro does:
// glibc's #define stdin stdin expands to itself, and a function-like macro such as assert only
// to a call.)
var rawMacros = []string{"EOF", "errno", "NULL", "EINVAL", "ENOENT", "EAGAIN", "SEEK_SET", "BUFSIZ",
	"INT64_C", "__x86_64__", "__GNUC__", "int64_t", "uint8_t", "uintptr_t"}

// TestMacrosRaw checks that the raw names of rawMacros don't compile with the runtime's header,
// with each compiler.
func TestMacrosRaw(t *testing.T) {
	dir := t.TempDir()
	for _, c := range compilers(t) {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			inc := includeFlag(t)
			reserved := make(map[string]bool)
			for _, r := range lower.ReservedNames() {
				reserved[r] = true
			}
			errs := make([]string, len(rawMacros))
			parallel(len(rawMacros), func(i int) {
				m := rawMacros[i]
				if !reserved[m] {
					errs[i] = m + " is not reserved"
					return
				}
				src := filepath.Join(dir, c.name+"-"+m+".cpp")
				typ := "int64_t"
				if strings.HasSuffix(m, "_t") {
					typ = m // an integer type: what breaks is a use of the type after the name
				}
				if err := os.WriteFile(src, []byte("#include <gocpp/rt.hpp>\n"+uses(m, "", typ)), 0o644); err != nil {
					errs[i] = err.Error()
					return
				}
				ok, _, err := c.syntaxOK(src, inc)
				switch {
				case err != nil:
					errs[i] = err.Error()
				case ok:
					errs[i] = m + ": the raw name compiles"
				}
			})
			for _, e := range errs {
				if e != "" {
					t.Error(e)
				}
			}
		})
	}
}

// macroModes are the flags (after the driver's) that can define more macros: the debug build,
// an optimized build (__OPTIMIZE__) and the sanitizers (-asan: __SANITIZE_ADDRESS__).
var macroModes = [][]string{nil, {"-O2"}, {"-fsanitize=address,undefined", "-fno-omit-frame-pointer"}}

// TestMacroList checks that internal/lower/cxx_macros.txt has every macro that the runtime's headers define,
// with each compiler, in each mode: the compiler lists them for a file that includes
// gocpp/rt.hpp (-dM -E). The macros that a Go program can't declare (Go keywords) are left out.
// When a runtime header, a compiler or the C library adds macros, the test fails; then
//
//	go test ./internal/cppgen -run TestMacroList -update
//
// adds them to the list (it keeps the names that it doesn't find, which another system can
// have: a list from MinGW in Phase 14, for example).
func TestMacroList(t *testing.T) {
	src := filepath.Join(t.TempDir(), "rt.cpp")
	if err := os.WriteFile(src, []byte("#include <gocpp/rt.hpp>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	found := make(map[string]bool)
	ran := false
	for _, c := range compilers(t) {
		if c.missing != "" {
			t.Logf("%s: %s", c.name, c.missing)
			continue
		}
		for _, mode := range macroModes {
			args := append(c.flags(includeFlag(t)), mode...)
			args = append(args, "-dM", "-E", src)
			out, _, code, err := command(filepath.Dir(src), false, c.cc.Path, args...)
			if err != nil || code != 0 {
				t.Fatalf("%s %s: exit code %d %v\n%s", c.cc.Path, strings.Join(args, " "), code, err, out)
			}
			for name := range defines(string(out)) {
				if token.IsIdentifier(name) {
					found[name] = true
				}
			}
			ran = true
		}
	}
	// The MinGW-w64 compiler of the Windows target (Phase 14): its C library headers define other
	// macros (DUMMYUNIONNAME, CLK_TCK, _alloca ...) that a Go name must not be either, as the
	// package syscall of Windows declares names like them. Only a compiler that is installed is
	// used (never the pinned toolchain's download); the list keeps what an absent one would add.
	t.Setenv("GOCPP_TOOLCHAIN", "local")
	if wcc, err := build.FindCXXFor(context.Background(), "windows", "amd64"); err != nil {
		t.Logf("windows/amd64: %v", err)
	} else {
		args := slices.Concat(build.Flags(wcc, false), []string{includeFlag(t), "-dM", "-E", src})
		out, _, code, err := command(filepath.Dir(src), false, wcc.Path, args...)
		if err != nil || code != 0 {
			t.Fatalf("%s %s: exit code %d %v\n%s", wcc.Path, strings.Join(args, " "), code, err, out)
		}
		for name := range defines(string(out)) {
			if token.IsIdentifier(name) {
				found[name] = true
			}
		}
		ran = true
	}
	// The dumps of compilers that are not installed here (testdata/mingw-macros: WinLibs' MinGW-w64 with
	// the posix thread model, whose headers define the pthread, signal and process macros that the
	// dev shell's mcf flavour does not).
	dumps, err := filepath.Glob(filepath.Join("testdata", "mingw-macros", "*.txt"))
	if err != nil || len(dumps) == 0 {
		t.Fatalf("no macro dumps in testdata/mingw-macros: %v", err)
	}
	for _, d := range dumps {
		text, err := os.ReadFile(d)
		if err != nil {
			t.Fatal(err)
		}
		for l := range strings.SplitSeq(string(text), "\n") {
			if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") && token.IsIdentifier(l) {
				found[l] = true
			}
		}
		ran = true
	}
	if !ran {
		t.Skip("no compiler")
	}
	listed := make(map[string]bool)
	for _, m := range readList(t, "cxx_macros.txt") {
		listed[m] = true
	}
	var missing []string
	for m := range found {
		if !listed[m] {
			missing = append(missing, m)
		}
	}
	slices.Sort(missing)
	if *update && len(missing) > 0 {
		for _, m := range missing {
			listed[m] = true
		}
		if err := writeMacroList(listed); err != nil {
			t.Fatal(err)
		}
		t.Logf("added %d macros to cxx_macros.txt", len(missing))
		return
	}
	if len(missing) > 0 {
		t.Errorf("cxx_macros.txt lacks %d macros of the runtime's headers (go test -run TestMacroList -update adds them): %s",
			len(missing), strings.Join(missing, " "))
	}
}

// defines returns the names of the macros in the output of -dM -E.
func defines(out string) map[string]bool {
	names := make(map[string]bool)
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		rest, ok := strings.CutPrefix(sc.Text(), "#define ")
		if !ok {
			continue
		}
		name, _, _ := strings.Cut(rest, " ")
		name, _, _ = strings.Cut(name, "(")
		names[name] = true
	}
	return names
}

// writeMacroList writes cxx_macros.txt with the names.
func writeMacroList(names map[string]bool) error {
	var b strings.Builder
	b.WriteString(`# The macros of the headers that gocpp/rt.hpp includes, with g++ and clang++ and the
# driver's flags (also -O2 and the sanitizers), with the MinGW-w64 compiler for windows/amd64 and with the dumps of
# testdata/mingw-macros (WinLibs' posix flavour), which a Go name must not be: names.go gives such a name a _ suffix. The compilers' predefined macros
# (__x86_64__, __GNUC__) are here too.
# TestMacroList checks the list; "go test ./internal/cppgen -run TestMacroList -update" adds
# new macros and keeps the old ones. One name per line.
`)
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	slices.Sort(sorted)
	for _, n := range sorted {
		b.WriteString(n + "\n")
	}
	return os.WriteFile(listFile("cxx_macros.txt"), []byte(b.String()), 0o644)
}

// firstLines returns the first n lines of s.
func firstLines(s string, n int) string {
	lines := strings.SplitAfter(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "")
}
