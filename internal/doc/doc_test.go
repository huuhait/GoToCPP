package doc

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
)

// The golden tests: "gocpp doc" on the module testdata/gpmod (gocpp's own
// syntax: !, macros, #Derive, generics; C headers; build constraints;
// ambiguous paths; a required module), against the files in testdata/golden.
// go test -run TestGolden -update writes them again; read the diff.

var update = flag.Bool("update", false, "write the golden files again")

// fakeProbe stands in for the C++ compiler: the header <stdio.h> is
// testdata/gpmod/sys/stdio.h, and Kind knows a few names.
type fakeProbe struct{ sys string }

func (p fakeProbe) FindHeader(_ context.Context, name string) (string, error) {
	switch name {
	case "stdio.h":
		return filepath.Join(p.sys, "stdio.h"), nil
	case "broken.h":
		return "", errors.New("the compiler failed")
	}
	return "", nil
}

func (p fakeProbe) Kind(_ context.Context, includes []string, path string) (string, error) {
	if len(includes) == 0 {
		return "", errors.New("no includes")
	}
	switch path {
	case "std::vector":
		return "template", nil
	case "std::string":
		return "type", nil
	case "std::cout":
		return "value", nil
	case "std::nothing":
		return "not found", nil
	case "std::weird":
		return "unknown", nil
	case "std::fail":
		return "", errors.New("the include does not compile")
	}
	return "not found", nil
}

func modDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("testdata/gpmod")
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// runDoc runs the command in dir (relative to the module) and returns the
// text that the golden file holds: the output, and the error with its exit
// status.
func runDoc(t *testing.T, dir string, goos string, tags []string, args ...string) string {
	t.Helper()
	mod := modDir(t)
	var opts Options
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "-u":
			opts.Unexported = true
		case "-c":
			opts.MatchCase = true
		case "-all":
			opts.All = true
		case "-cmd":
			opts.Cmd = true
		case "-src":
			opts.Src = true
		case "-short":
			opts.Short = true
		case "-examples":
			opts.Examples = true
		default:
			t.Fatalf("unknown flag %s", args[0])
		}
		args = args[1:]
	}
	if goos == "" {
		goos = "linux"
	}
	var buf bytes.Buffer
	env := &Env{Dir: filepath.Join(mod, dir), GOOS: goos, GOARCH: "amd64", Tags: tags, Probe: fakeProbe{sys: filepath.Join(mod, "sys")}}
	err := Run(&buf, env, opts, args)
	out := buf.String()
	if err != nil {
		status := 1
		var ue *UsageError
		if errors.As(err, &ue) {
			status = 2
		}
		out += "--- error (exit " + string(rune('0'+status)) + "): " + err.Error() + "\n"
	}
	return moduleNamed(out, mod)
}

// belowModule matches what follows $MOD in a path: the rest of the path, up to a space, a quote or
// a bracket.
var belowModule = regexp.MustCompile(`\$MOD[^\s)'"]*`)

// moduleNamed writes the folder mod as $MOD in out, and the paths below it with '/', as the golden
// files have them: on Windows the folder is written with '\' (and in one message with '/').
func moduleNamed(out, mod string) string {
	if filepath.Separator == '/' {
		return strings.ReplaceAll(out, mod, "$MOD")
	}
	out = strings.ReplaceAll(out, mod, "$MOD")
	out = strings.ReplaceAll(out, filepath.ToSlash(mod), "$MOD")
	return belowModule.ReplaceAllStringFunc(out, filepath.ToSlash)
}

var goldenCases = []struct {
	name string
	dir  string   // folder to run in, relative to the module
	goos string   // "" is linux
	tags []string // build tags
	args []string
}{
	// Packages of gocpp's own syntax.
	{name: "macros-package", args: []string{"macros"}},
	{name: "macros-all", args: []string{"-all", "macros"}},
	{name: "macros-short", args: []string{"-short", "macros"}},
	{name: "macros-all-unexported", args: []string{"-u", "-all", "macros"}},
	{name: "macros-src-parse", args: []string{"-src", "macros.Parse"}},
	{name: "macros-sym-parse", args: []string{"macros.Parse"}},
	{name: "macros-sym-parse-bang", args: []string{"macros.Parse!"}},
	{name: "macros-sym-assert", args: []string{"macros.Assert"}},
	{name: "macros-sym-assert-lower", args: []string{"macros.assert"}},
	{name: "macros-sym-assert-hash", args: []string{"macros.#Assert"}},
	{name: "macros-sym-assert-matchcase", args: []string{"-c", "macros.assert"}},
	{name: "macros-src-assert", args: []string{"-src", "macros.Assert"}},
	{name: "macros-sym-locked", args: []string{"macros.Locked"}},
	{name: "macros-sym-trace", args: []string{"macros", "Trace"}},
	{name: "macros-sym-run-ontop", args: []string{"macros.Run"}},
	{name: "macros-src-run-ontop", args: []string{"-src", "macros.Run"}},
	{name: "derive-enum", args: []string{"macros.Color"}},
	{name: "derive-enum-string-method", args: []string{"macros.Color.String"}},
	{name: "derive-enum-const", args: []string{"macros.Red"}},
	{name: "derive-struct", args: []string{"macros.Point"}},
	{name: "derive-struct-equal-method", args: []string{"macros", "Point.Equal"}},
	{name: "derive-struct-lower-method", args: []string{"macros.point.clone"}},
	{name: "derive-json", args: []string{"macros.Config"}},
	{name: "derive-json-method", args: []string{"macros.Config.MarshalJSON"}},
	{name: "derive-json-field", args: []string{"macros.Config.Port"}},
	{name: "bang-method", args: []string{"macros.Config.Load"}},
	{name: "bang-method-with-bang", args: []string{"macros.Config.Load!"}},

	// Generics, embedded fields, interfaces, constants, deprecation.
	{name: "shapes-package", args: []string{"shapes"}},
	{name: "shapes-all", args: []string{"-all", "shapes"}},
	{name: "shapes-short", args: []string{"-short", "shapes"}},
	{name: "shapes-all-unexported", args: []string{"-u", "-all", "shapes"}},
	{name: "generic-type", args: []string{"shapes.Stack"}},
	{name: "generic-type-src", args: []string{"-src", "shapes.Stack"}},
	{name: "generic-method", args: []string{"shapes.Stack.Push"}},
	{name: "generic-func", args: []string{"shapes.Map"}},
	{name: "generic-func-constraint", args: []string{"shapes.Sum"}},
	{name: "generic-pair", args: []string{"shapes.Pair"}},
	{name: "generic-constructor", args: []string{"shapes.NewStack"}},
	{name: "constraint-interface", args: []string{"shapes.Number"}},
	{name: "interface-all-methods", args: []string{"shapes.Shape"}},
	{name: "interface-method", args: []string{"shapes.Shape.Area"}},
	{name: "embedded-fields", args: []string{"shapes.Circle"}},
	{name: "embedded-fields-unexported", args: []string{"-u", "shapes.Base"}},
	{name: "struct-field", args: []string{"shapes.Circle.Radius"}},
	{name: "struct-field-hidden-by-default", args: []string{"shapes", "Base.tag"}},
	{name: "struct-field-unexported", args: []string{"-u", "shapes.Base.tag"}},
	{name: "const-group-iota", args: []string{"shapes.Low"}},
	{name: "const-group-iota-lower", args: []string{"shapes", "mid"}},
	{name: "const-typed-standalone", args: []string{"shapes.MaxLevel"}},
	{name: "const-unexported", args: []string{"-u", "shapes.secret"}},
	{name: "const-unexported-hidden", args: []string{"shapes.secret"}},
	{name: "var-default", args: []string{"shapes.Default"}},
	{name: "deprecated-func", args: []string{"shapes.Old"}},
	{name: "deprecated-alias", args: []string{"shapes.Disc"}},
	{name: "unexported-func", args: []string{"-u", "shapes", "helper"}},
	{name: "symbol-in-cwd", dir: "shapes", args: []string{"Circle"}},
	{name: "method-in-cwd", dir: "shapes", args: []string{"Circle.Area"}},
	{name: "package-in-cwd", dir: "shapes", args: nil},
	{name: "package-in-cwd-short", dir: "shapes", args: []string{"-short"}},

	// Commands.
	{name: "cmd-hidden", args: []string{"cmdtool"}},
	{name: "cmd-shown", args: []string{"-cmd", "cmdtool"}},
	{name: "cmd-all", args: []string{"-cmd", "-all", "cmdtool"}},

	// Packages: partial paths, ambiguity, modules.
	{name: "partial-path-first-match", args: []string{"util"}},
	{name: "partial-path-falls-through", args: []string{"util.Second"}, dir: "."},
	{name: "partial-path-two-args", args: []string{"util", "Second"}},
	{name: "partial-path-first-wins", args: []string{"util", "Both"}},
	{name: "full-path-second", args: []string{"example.com/gp/b/util"}},
	{name: "path-suffix", args: []string{"b/util"}},
	{name: "dependency-package", args: []string{"example.com/dep/lib"}},
	{name: "dependency-partial", args: []string{"lib", "Clamp"}},
	{name: "dependency-generic", args: []string{"-all", "lib"}},
	{name: "relative-dir", args: []string{"./plat"}},
	{name: "std-via-flag-ordering", args: []string{"-short", "errors"}},

	// Build constraints.
	{name: "plat-linux", args: []string{"-all", "plat"}},
	{name: "plat-windows", goos: "windows", args: []string{"-all", "plat"}},
	{name: "plat-tag", tags: []string{"fancy"}, args: []string{"-short", "plat"}},
	{name: "plat-symbol-other-os", goos: "windows", args: []string{"plat", "OnlyLinux"}},

	// Errors.
	{name: "err-no-package", dir: "shapes", args: []string{"nosuchpkg"}},
	{name: "err-no-symbol", args: []string{"shapes", "Nope"}},
	{name: "err-no-symbol-one-arg", args: []string{"shapes.Nope"}},
	{name: "err-no-method", args: []string{"shapes", "Circle.Nope"}},
	{name: "err-no-symbol-multi-path", args: []string{"util", "Nope"}},
	{name: "err-too-many-args", args: []string{"a", "b", "c"}},
	{name: "err-too-many-periods", args: []string{"shapes", "a.b.c"}},
	{name: "err-matchcase", args: []string{"-c", "shapes", "circle"}},
	{name: "err-not-a-type", args: []string{"shapes", "Map.X"}},
	{name: "err-no-go-files", args: []string{"."}},

	// Examples (the flag is gocpp's).
	{name: "examples-package", args: []string{"-examples", "examples"}},
	{name: "examples-all", args: []string{"-examples", "-all", "examples"}},
	{name: "examples-func", args: []string{"-examples", "examples.Add"}},
	{name: "examples-method", args: []string{"-examples", "examples.Stack.Push"}},
	{name: "examples-off", args: []string{"-all", "examples"}},

	// Review round 1: a macro right after the package clause, examples in an
	// external test package, a field named like a header extension.
	{name: "derive-after-package", args: []string{"-all", "derivefirst"}},
	{name: "derive-after-package-method", args: []string{"derivefirst.Level.String"}},
	{name: "wrap-long-comment-package", args: []string{"longdoc"}},
	{name: "wrap-long-comment-func", args: []string{"longdoc.Wide"}},
	{name: "derive-on-type-group", args: []string{"-all", "derivegroup"}},
	{name: "examples-external-package", args: []string{"-examples", "-all", "extest"}},
	{name: "examples-external-list", args: []string{"-examples", "extest"}},
	{name: "examples-external-func", args: []string{"-examples", "-all", "extest.Add"}},
	{name: "examples-external-method", args: []string{"-examples", "extest.T.M"}},
	{name: "field-named-like-header", args: []string{"-u", "fields.Mixed.h"}},
	{name: "field-named-like-header-exported", args: []string{"fields.Mixed.H"}},
	{name: "header-doxygen-blocks", args: []string{"-all", "./physics/doxy.h"}},
	{name: "header-doxygen-sym", args: []string{"./physics/doxy.h", "add"}},
	{name: "header-doxygen-sym-last", args: []string{"./physics/doxy.h", "last"}},

	// Headers (the compiler is replaced by fakeProbe).
	{name: "header-local", args: []string{"./physics/physics.h"}},
	{name: "header-local-all", args: []string{"-all", "./physics/physics.h"}},
	{name: "header-local-short", args: []string{"-short", "./physics/physics.h"}},
	{name: "header-sym-function", args: []string{"./physics/physics.h", "vec_add"}},
	{name: "header-sym-struct", args: []string{"./physics/physics.h", "Body"}},
	{name: "header-sym-macro", args: []string{"./physics/physics.h", "PHYS_C"}},
	{name: "header-sym-case-sensitive", args: []string{"./physics/physics.h", "vec_ADD"}},
	{name: "header-sym-lower-folds", args: []string{"./physics/physics.h", "phys_c"}},
	{name: "header-sym-matchcase", args: []string{"-c", "./physics/physics.h", "phys_c"}},
	{name: "header-sym-missing", args: []string{"./physics/physics.h", "Nope"}},
	{name: "header-missing-file", args: []string{"./physics/nope.h"}},
	{name: "header-system", args: []string{"<stdio.h>"}},
	{name: "header-system-all", args: []string{"-all", "<stdio.h>"}},
	{name: "header-system-sym", args: []string{"<stdio.h>", "printf"}},
	{name: "header-system-not-found", args: []string{"<nosuch.h>"}},
	{name: "header-system-error", args: []string{"<broken.h>"}},
	{name: "header-cxx-template", args: []string{"<vector>", "std.vector"}},
	{name: "header-cxx-type", args: []string{"<string>", "std.string"}},
	{name: "header-cxx-value", args: []string{"<iostream>", "std.cout"}},
	{name: "header-cxx-short", args: []string{"-short", "<vector>", "std.vector"}},
	{name: "header-cxx-not-found", args: []string{"<vector>", "std.nothing"}},
	{name: "header-cxx-unknown", args: []string{"<vector>", "std.weird"}},
	{name: "header-cxx-failure", args: []string{"<vector>", "std.fail"}},
	{name: "namespace-std", args: []string{"std"}},
	{name: "namespace-std-short", args: []string{"-short", "std"}},
	{name: "namespace-nested", args: []string{"::std::chrono"}},
}

func TestGolden(t *testing.T) {
	for _, c := range goldenCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := c.dir
			if dir == "" {
				dir = "."
			}
			got := runDoc(t, dir, c.goos, c.tags, c.args...)
			file := filepath.Join("testdata", "golden", c.name+".txt")
			if *update {
				if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("%v (run with -update)", err)
			}
			if got != string(want) {
				t.Errorf("gocpp doc %s\n--- got\n%s--- want\n%s", strings.Join(c.args, " "), got, want)
			}
		})
	}
}

// TestGoldenFilesAreUsed fails for a golden file that no case reads, so the
// directory does not collect leftovers.
func TestGoldenFilesAreUsed(t *testing.T) {
	entries, err := os.ReadDir("testdata/golden")
	if err != nil {
		t.Skip("no golden files")
	}
	used := map[string]bool{}
	for _, c := range goldenCases {
		if used[c.name] {
			t.Errorf("two cases are named %s", c.name)
		}
		used[c.name] = true
	}
	for _, e := range entries {
		if !used[strings.TrimSuffix(e.Name(), ".txt")] {
			t.Errorf("testdata/golden/%s belongs to no case", e.Name())
		}
	}
}

// TestGoldenContent checks the things the golden files must show: they
// are read by a person, who should see that they are right (a golden file
// that is wrong from the start is easy to accept).
func TestGoldenContent(t *testing.T) {
	check := func(name string, must ...string) {
		t.Helper()
		got := runDoc(t, ".", "", nil, strings.Fields(name)...)
		for _, m := range must {
			if !strings.Contains(got, m) {
				t.Errorf("gocpp doc %s lacks %q:\n%s", name, m, got)
			}
		}
	}
	check("-all macros",
		"func Parse!(s string) int",
		"macro Assert(cond bool, msg lazy string)",
		"#Derive(String, Equal, Clone)",
		"func (p Point) Equal(other Point) bool\n    Equal is generated by #Derive(Equal).",
		"func (c Config) MarshalJSON() ([]byte, error)",
		"#Trace()\nfunc Run()")
	check("-all shapes",
		"type Pair[K comparable, V any] struct {",
		"func Map[T, U any](xs []T, f func(T) U) []U",
		"func (s *Stack[T]) Push(v T)",
		"Deprecated: use New.",
		"type Disc = Circle",
		"\t*macros.Point\n",
		"Low Level = iota + 1")
	check("-all plat", "OnlyLinux", "Always")
	check("./physics/physics.h vec_add", "Adds two vectors.", "double vec_add(double a, double b);")
	check("-cmd cmdtool", "package main", "func Helper()")
}

func TestGoldenPlatformsDiffer(t *testing.T) {
	lin := runDoc(t, ".", "linux", nil, "-short", "plat")
	win := runDoc(t, ".", "windows", nil, "-short", "plat")
	fancy := runDoc(t, ".", "linux", []string{"fancy"}, "-short", "plat")
	if strings.Contains(lin, "OnlyWindows") || !strings.Contains(lin, "OnlyLinux") {
		t.Errorf("linux:\n%s", lin)
	}
	if strings.Contains(win, "OnlyLinux") || !strings.Contains(win, "OnlyWindows") {
		t.Errorf("windows:\n%s", win)
	}
	if strings.Contains(lin, "Fancy") || !strings.Contains(fancy, "Fancy") {
		t.Errorf("tag fancy:\n%s\n%s", lin, fancy)
	}
}

func TestExitStatuses(t *testing.T) {
	for _, c := range []struct {
		args   []string
		status string
	}{
		{[]string{"a", "b", "c"}, "exit 2"},
		{[]string{"shapes", "a.b.c"}, "exit 2"},
		{[]string{"shapes", "Nope"}, "exit 1"},
		{[]string{"nosuch/pkg"}, "exit 1"},
		{[]string{"shapes"}, ""},
	} {
		got := runDoc(t, ".", "", nil, c.args...)
		if c.status == "" {
			if strings.Contains(got, "--- error") {
				t.Errorf("%v: unexpected error:\n%s", c.args, got)
			}
		} else if !strings.Contains(got, "--- error ("+c.status+")") {
			t.Errorf("%v: want %s:\n%s", c.args, c.status, got)
		}
	}
}

// TestDeterminism: the same question has the same answer, also at the same
// time (Render must not share state between runs).
func TestDeterminism(t *testing.T) {
	args := []string{"-all", "-u", "macros"}
	first := runDoc(t, ".", "", nil, args...)
	var wg sync.WaitGroup
	results := make([]string, 8)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = runDoc(t, ".", "", nil, args...)
		}()
	}
	wg.Wait()
	for i, r := range results {
		if r != first {
			t.Errorf("run %d printed something else", i)
		}
	}
}

// The standard library is documented from its embedded .gp files.
func TestStdlib(t *testing.T) {
	run := func(args ...string) (string, error) {
		var buf bytes.Buffer
		var opts Options
		for len(args) > 0 && strings.HasPrefix(args[0], "-") {
			switch args[0] {
			case "-all":
				opts.All = true
			case "-short":
				opts.Short = true
			case "-src":
				opts.Src = true
			}
			args = args[1:]
		}
		err := Run(&buf, &Env{Dir: t.TempDir(), GOOS: "linux", GOARCH: "amd64"}, opts, args)
		return buf.String(), err
	}
	for _, c := range []struct {
		args []string
		want []string
	}{
		{[]string{"errors"}, []string{`package errors // import "errors"`, "func New(text string) error"}},
		{[]string{"strings.Builder"}, []string{"type Builder struct {", "func (b *Builder) WriteString(s string) (int, error)"}},
		{[]string{"-short", "strings"}, []string{"func Contains(s, substr string) bool"}},
		{[]string{"slices.Sort"}, []string{"func Sort[S ~[]E, E cmp.Ordered](x S)"}},
		{[]string{"-src", "strings.Contains"}, []string{"// Contains reports whether substr is within s.\nfunc Contains(s, substr string) bool"}},
		{[]string{"fmt.Println"}, []string{"func Println(a ...any) (n int, err error)"}},
		{[]string{"maps"}, []string{`package maps // import "maps"`}},
	} {
		out, err := run(c.args...)
		if err != nil {
			t.Errorf("%v: %v", c.args, err)
			continue
		}
		for _, w := range c.want {
			if !strings.Contains(out, w) {
				t.Errorf("%v lacks %q:\n%s", c.args, w, out)
			}
		}
	}
}

// Env.Stdlib replaces the embedded standard library (an editor, the tests).
func TestCustomStdlib(t *testing.T) {
	fsys := fstest.MapFS{
		"mini/mini.gp": {Data: []byte("// Package mini is small.\npackage mini\n\n// Hello says hello.\nfunc Hello() string { return \"hi\" }\n")},
	}
	var buf bytes.Buffer
	err := Run(&buf, &Env{Dir: t.TempDir(), GOOS: "linux", GOARCH: "amd64", Stdlib: fsys, StdlibDir: "$STD"}, Options{Src: true}, []string{"mini.Hello"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "// Hello says hello.\nfunc Hello() string {") {
		t.Errorf("got\n%s", buf.String())
	}
	buf.Reset()
	err = Run(&buf, &Env{Dir: t.TempDir(), GOOS: "linux", GOARCH: "amd64", Stdlib: fsys, StdlibDir: "$STD"}, Options{}, []string{"mini", "Nope"})
	if err == nil || !strings.Contains(err.Error(), "no symbol Nope in package mini") {
		t.Errorf("error %v", err)
	}
}

func readPkg(t *testing.T, dir string) *Package {
	t.Helper()
	env := &Env{GOOS: "linux", GOARCH: "amd64"}
	pkg, err := env.ReadDir(os.DirFS(modDir(t)), dir, "example.com/gp/"+dir)
	if err != nil {
		t.Fatal(err)
	}
	return pkg
}

// Render is Run for a package that the caller read: the same text.
func TestRenderMatchesRun(t *testing.T) {
	pkg := readPkg(t, "shapes")
	for _, c := range []struct {
		symbol string
		opts   Options
		args   []string
	}{
		{"", Options{}, []string{"shapes"}},
		{"", Options{All: true}, []string{"-all", "shapes"}},
		{"Circle", Options{}, []string{"shapes.Circle"}},
		{"Circle.Area", Options{}, []string{"shapes.Circle.Area"}},
		{"Circle.Radius", Options{}, []string{"shapes.Circle.Radius"}},
		{"stack.push", Options{}, []string{"shapes.stack.push"}},
		{"Map", Options{Src: true}, []string{"-src", "shapes.Map"}},
		{"helper", Options{Unexported: true}, []string{"-u", "shapes.helper"}},
		{"low", Options{Short: true}, []string{"-short", "shapes.low"}},
	} {
		var buf bytes.Buffer
		if err := Render(&buf, pkg, c.symbol, c.opts); err != nil {
			t.Errorf("Render(%q): %v", c.symbol, err)
			continue
		}
		want := strings.ReplaceAll(runDoc(t, ".", "", nil, c.args...), "$MOD", modDir(t))
		if buf.String() != want {
			t.Errorf("Render(%q) differs from Run:\n--- Render\n%s--- Run\n%s", c.symbol, buf.String(), want)
		}
	}
}

func TestRenderErrors(t *testing.T) {
	pkg := readPkg(t, "shapes")
	for _, c := range []struct {
		symbol, want string
		opts         Options
	}{
		{"Nope", "no symbol Nope in package example.com/gp/shapes", Options{}},
		{"Circle.Nope", "no method or field Circle.Nope in package example.com/gp/shapes", Options{}},
		{"circle", "no symbol circle in package", Options{MatchCase: true}},
		{"helper", "no symbol helper in package", Options{}},
		{"a.b.c", "too many periods", Options{}},
	} {
		var buf bytes.Buffer
		err := Render(&buf, pkg, c.symbol, c.opts)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("Render(%q): error %v, want %q", c.symbol, err, c.want)
		}
		if buf.Len() != 0 {
			t.Errorf("Render(%q) wrote %q before the error", c.symbol, buf.String())
		}
	}
}

// Render works on text that is not in a file (an editor buffer), and the
// Package can be rendered again and again, in parallel.
func TestRenderBuffer(t *testing.T) {
	pkg := &Package{
		ImportPath: "example.com/buf",
		Name:       "buf",
		Dir:        "/virtual/buf",
		Files: []File{{Name: "a.gp", Src: []byte(`// Package buf is in a buffer.
package buf

// Get returns the value or fails.
func Get!(k string) int { return 0 }

#Derive(String)
type Kind int
`)}},
	}
	var wg sync.WaitGroup
	outs := make([]string, 6)
	for i := range outs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var buf bytes.Buffer
			if err := Render(&buf, pkg, "Get", Options{}); err != nil {
				t.Error(err)
			}
			outs[i] = buf.String()
		}()
	}
	wg.Wait()
	want := "package buf // import \"example.com/buf\"\n\nfunc Get!(k string) int\n    Get returns the value or fails.\n\n"
	for _, o := range outs {
		if o != want {
			t.Errorf("got %q, want %q", o, want)
		}
	}
	var buf bytes.Buffer
	if err := Render(&buf, pkg, "Kind.String", Options{}); err != nil || !strings.Contains(buf.String(), "func (k Kind) String() string") {
		t.Errorf("derived method: %v\n%s", err, buf.String())
	}
	// A syntax error is an error, not a crash.
	bad := &Package{ImportPath: "bad", Name: "bad", Dir: "/virtual/bad", Files: []File{{Name: "a.gp", Src: []byte("package bad\nfunc (")}}}
	if err := Render(&buf, bad, "", Options{}); err == nil {
		t.Error("a file with a syntax error rendered")
	}
	// No files at all.
	if err := Render(&buf, &Package{Name: "none", Dir: "/virtual/none"}, "", Options{}); err == nil {
		t.Error("a package without files rendered")
	}
}

// A comment is wrapped at 80 columns and written by go/doc/comment: lists,
// headings, code blocks and links.
func TestCommentText(t *testing.T) {
	pkg := &Package{ImportPath: "x", Name: "x", Dir: "/x", Files: []File{{Name: "x.gp", Src: []byte(`// Package x tests comments.
//
// # Heading
//
// A long paragraph that goes on and on to show that the text is wrapped by the printer of go/doc/comment at the same column as go doc does.
//
//   - first item
//   - second item
//
// Code:
//
//	x := 1
//
// See [Alpha] and https://example.com/doc.
package x

// Alpha is a function.
func Alpha() {}
`)}}}
	var buf bytes.Buffer
	if err := Render(&buf, pkg, "", Options{}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, w := range []string{
		"Heading\n\nA long paragraph that goes on and on to show that the text is wrapped by the\nprinter of go/doc/comment at the same column as go doc does.\n",
		"  - first item\n  - second item\n",
		"    x := 1\n",
		"See Alpha and https://example.com/doc.\n",
	} {
		if !strings.Contains(got, w) {
			t.Errorf("lacks %q:\n%s", w, got)
		}
	}
	for line := range strings.Lines(got) {
		if len(strings.TrimSuffix(line, "\n")) > 80 && !strings.Contains(line, "://") {
			t.Errorf("line over 80 columns: %q", line)
		}
	}
}

// Documentation of the macros of a package from another package is out of
// scope (no cross-package expansion): a type from another package is just
// printed as written.
func TestCrossPackageTypeIsAsWritten(t *testing.T) {
	got := runDoc(t, ".", "", nil, "shapes.Circle")
	if !strings.Contains(got, "*macros.Point") {
		t.Errorf("got\n%s", got)
	}
}

func TestOptionsDoNotLeakBetweenRuns(t *testing.T) {
	a := runDoc(t, ".", "", nil, "-u", "shapes")
	b := runDoc(t, ".", "", nil, "shapes")
	if a == b || !strings.Contains(a, "helper") || strings.Contains(b, "helper") {
		t.Errorf("-u: %q vs %q", a, b)
	}
}

var abs = regexp.MustCompile(`/home/|/tmp/|/nix/`)

// No output depends on the machine (a path of this machine would be in a
// golden file otherwise).
func TestGoldenFilesHavePortablePaths(t *testing.T) {
	entries, _ := os.ReadDir("testdata/golden")
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join("testdata/golden", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if abs.Match(data) {
			t.Errorf("%s has a path of this machine", e.Name())
		}
	}
}

// The real C++ compiler finds a system header and tells what a name is. It
// skips when there is none (GOCPP_REQUIRE_CXX makes that a failure).
func TestRealCompiler(t *testing.T) {
	env := &Env{Dir: t.TempDir(), GOOS: "linux", GOARCH: "amd64"}
	var buf bytes.Buffer
	err := Run(&buf, env, Options{All: true}, []string{"<stdio.h>"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "Header file: not found") {
		if os.Getenv("GOCPP_REQUIRE_CXX") != "" {
			t.Fatalf("no C++ compiler:\n%s", buf.String())
		}
		t.Skip("no C++ compiler")
	}
	if !regexp.MustCompile(`(?m)^Header file: /.*/stdio\.h$`).MatchString(buf.String()) {
		t.Errorf("no header file:\n%s", buf.String())
	}
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"<vector>", "std.vector"}, "std.vector: a class or alias template"},
		{[]string{"<string>", "std.string"}, "std.string: a type"},
		{[]string{"<iostream>", "std.cout"}, "std.cout: a value"},
		{[]string{"<vector>", "std.nosuchthing"}, "no symbol std.nosuchthing in header <vector>"},
	} {
		buf.Reset()
		err := Run(&buf, env, Options{}, c.args)
		got := buf.String()
		if err != nil {
			got = err.Error()
		}
		if !strings.Contains(got, c.want) {
			t.Errorf("%v: got %q, want %q", c.args, got, c.want)
		}
	}
}

func TestJoinStringsTruncates(t *testing.T) {
	if got := joinStrings([]string{"a", "b"}); got != "a, b" {
		t.Errorf("short list: %q", got)
	}
	long := strings.Fields(strings.Repeat("abcdefghij ", 12)) // 12 names of 10 letters
	got := joinStrings(long)
	if !strings.HasSuffix(got, ", ...") || len(got) > punchedCardWidth+len(", ...") {
		t.Errorf("long list: %q", got)
	}
	if n := strings.Count(got, "abcdefghij"); n != 6 {
		t.Errorf("kept %d names, want 6 (the width is 80 columns): %q", n, got)
	}
}

func TestDocumentationFilesAreSkipped(t *testing.T) {
	got := runDoc(t, ".", "", nil, "-all", "-u", "plat")
	if strings.Contains(got, "Hidden") || strings.Contains(got, "only documents") {
		t.Errorf("the file of package documentation is shown:\n%s", got)
	}
}
