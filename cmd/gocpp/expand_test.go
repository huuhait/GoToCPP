package main

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/syntax/gpast"
	gpparser "github.com/huuhait/gocpp/internal/syntax/parser"
)

// The tests of gocpp expand (Phase 10 step 10.11): the output for each construct (golden files in
// testdata/expand: -update writes them), that a file with nothing to expand comes back byte for
// byte, that a package prints each file under its name, that errors print like gocpp check's, and that
// the output is the Go that the program stands for: the expansion of the programs of tests/run is run
// by real Go and prints what the program's // Output: block says.

// expandIn runs gocpp expand with the gates open in a copy of the files.
func expandIn(t *testing.T, files map[string]string, args ...string) result {
	t.Helper()
	inDir(t, files)
	t.Setenv(gatePhaseEnv, "15")
	return gocppRun("", false, append([]string{"expand"}, args...)...)
}

// fileTree reads the files of the folder dir into a map, with their names relative to dir.
func fileTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// TestExpandGolden: every .gp file of testdata/expand is a program, and every folder is a package
// (expanded as "."); NAME.golden is what gocpp expand prints for it.
func TestExpandGolden(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("testdata", "expand"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		name, files, args := e.Name(), map[string]string{}, []string{"."}
		switch {
		case e.IsDir():
			files = fileTree(t, filepath.Join(root, name))
		case strings.HasSuffix(name, ".gp"):
			name = strings.TrimSuffix(name, ".gp")
			data, err := os.ReadFile(filepath.Join(root, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			files["main.gp"] = string(data)
			args = []string{"main.gp"}
		default:
			continue
		}
		n++
		t.Run(name, func(t *testing.T) {
			if name == "mixed" {
				requireCXX(t)
			}
			r := expandIn(t, files, args...)
			if r.code != 0 || r.stderr != "" {
				t.Fatalf("gocpp expand %v = %d\n%s", args, r.code, r.stderr)
			}
			golden := filepath.Join(root, name+".golden")
			if *updateGolden {
				if err := os.WriteFile(golden, []byte(r.stdout), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (go test ./cmd/gocpp -run TestExpandGolden -update writes it)", err)
			}
			if r.stdout != string(want) {
				t.Errorf("gocpp expand differs from %s (-update rewrites it):\n%s", golden, lineDiff(string(want), r.stdout))
			}
		})
	}
	if n < 6 {
		t.Errorf("%d programs in testdata/expand, want at least 6", n)
	}
}

// requireCXX skips the test when there is no C++ compiler (a header import needs one).
func requireCXX(t *testing.T) {
	t.Helper()
	for _, cxx := range []string{"g++", "clang++"} {
		if _, err := exec.LookPath(cxx); err == nil {
			return
		}
	}
	t.Skip("no C++ compiler")
}

// lineDiff returns the first lines where want and got differ.
func lineDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := range max(len(w), len(g)) {
		var a, b string
		if i < len(w) {
			a = w[i]
		}
		if i < len(g) {
			b = g[i]
		}
		if a != b || i >= len(w) || i >= len(g) {
			return "line " + strconv.Itoa(i+1) + ":\n want: " + strconv.Quote(a) + "\n  got: " + strconv.Quote(b)
		}
	}
	return ""
}

// TestExpandEmbed: the bytes of an embedded file are shortened when the compiler's constant is printed, and -full
// prints them; a //go:embed variable is printed as written.
func TestExpandEmbed(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("testdata", "expand", "embed"))
	if err != nil {
		t.Fatal(err)
	}
	files := fileTree(t, root)
	r := expandIn(t, files, ".")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("exit %d\n%s", r.code, r.stderr)
	}
	big := strings.Repeat("0123456789", 40)
	for _, want := range []string{
		"//go:embed small.txt\nvar small string\n",
		`mine := "` + strings.Repeat("abcdefghij", 25) + `" // the user's own long literal stays`,
		`fmt.Println(mine, small, "hi\n", len("0123456789012345678901234567890123456789" /* 400 bytes in all */), "main.gp", 14)`,
	} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("no %q in the output:\n%s", want, r.stdout)
		}
	}
	if strings.Contains(r.stdout, big) {
		t.Errorf("the whole of big.txt is in the output")
	}
	inDir(t, files)
	full := gocppRun("", false, "expand", "-full", ".")
	if full.code != 0 || !strings.Contains(full.stdout, `len("`+big+`")`) {
		t.Errorf("-full: exit %d\n%s", full.code, full.stdout)
	}
}

// TestExpandPlain: a file with nothing to expand is printed exactly as it is, also its odd spacing,
// its comments and a file without a final newline.
func TestExpandPlain(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "expand", "plain.gp"))
	if err != nil {
		t.Fatal(err)
	}
	for name, src := range map[string]string{
		"plain":         string(data),
		"no newline":    strings.TrimRight(string(data), "\n"),
		"crlf":          strings.ReplaceAll(string(data), "\n", "\r\n"),
		"line":          "package main\n\n//line other.gp:100\nfunc main() {\n\tprintln(1)\n}\n\nvar x = 2\n",
		"empty":         "package main\n",
		"blank lines":   "\n\n// lead\n\n\npackage main\n\n\n\nimport \"fmt\"\n\n\nfunc main() { fmt.Println() }\n\n\n// tail\n\n\n",
		"only comments": "// first\n\npackage main // trailing\n\n// last\n",
		// The parser takes the carriage returns out of comments and raw strings, so the end of a token
		// is not where its text says.
		"crlf comment and raw string": "package main\r\n\r\nimport \"fmt\"\r\n\r\n/* line one\r\n   line two */\r\n\r\nvar s = `a\r\nb\r\n\r\n`\r\n\r\nfunc main() { fmt.Println(s) }\r\n",
		"crlf no final newline":       "package main\r\n\r\nvar s = `a\r\nb`",
		"bom":                         "\xef\xbb\xbfpackage main\n\nfunc main() {}\n",
		"mixed line ends":             "package main\r\n\nvar a = 1\r\nvar b = `x\ny`\n\r\nfunc main() {}\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			r := expandIn(t, map[string]string{"main.gp": src}, "main.gp")
			want := src
			if r.code != 0 || r.stderr != "" || r.stdout != want {
				t.Errorf("gocpp expand = %d, %q\n%s\nwant %q", r.code, r.stdout, r.stderr, want)
			}
		})
	}
}

// TestExpandOutputIsGo: what gocpp expand prints has no Phase 10 syntax left, parses as Go, and is the
// same when it is run again.
func TestExpandOutputIsGo(t *testing.T) {
	root := filepath.Join("testdata", "expand")
	for _, name := range []string{"must", "macros", "derive"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, name+".gp"))
			if err != nil {
				t.Fatal(err)
			}
			r := expandIn(t, map[string]string{"main.gp": string(data)}, "main.gp")
			if r.code != 0 {
				t.Fatalf("exit %d\n%s", r.code, r.stderr)
			}
			// The stock parser takes it: no macro, no lazy, no !.
			if _, err := parser.ParseFile(token.NewFileSet(), "expand.go", r.stdout, parser.ParseComments); err != nil {
				t.Errorf("the output does not parse as Go: %v\n%s", err, r.stdout)
			}
			f, err := gpparser.ParseFile(token.NewFileSet(), "expand.gp", r.stdout, gpparser.Phase10Syntax|gpparser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			if left := gpast.Leftovers(f); len(left) != 0 {
				t.Errorf("%d Phase 10 forms are left in the output", len(left))
			}
			if again := gocppRun("", false, "expand", "main.gp"); again.stdout != r.stdout {
				t.Errorf("the second run prints something else:\n%s", lineDiff(r.stdout, again.stdout))
			}
		})
	}
}

// TestExpandPackage: the files of a package are printed one after the other, each after a line
// "//// name", in the order of their names; a single file has no such line.
func TestExpandPackage(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("testdata", "expand", "pkg"))
	if err != nil {
		t.Fatal(err)
	}
	files := fileTree(t, root)
	r := expandIn(t, files, ".")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("exit %d\n%s", r.code, r.stderr)
	}
	if got := regexp.MustCompile(`(?m)^//// .*$`).FindAllString(r.stdout, -1); !slices.Equal(got, []string{"//// a.gp", "//// b.gp"}) {
		t.Errorf("headers = %q", got)
	}
	// A pattern for two packages: the files of each have their path.
	files["util/extra.gp"] = "package util\n\nfunc Extra() {}\n"
	r = expandIn(t, files, "./...")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.stderr)
	}
	if got := regexp.MustCompile(`(?m)^//// .*$`).FindAllString(r.stdout, -1); !slices.Equal(got, []string{"//// a.gp", "//// b.gp", "//// util/extra.gp", "//// util/util.gp"}) {
		t.Errorf("headers = %q", got)
	}
}

// TestExpandErrors: the errors of the check are printed like gocpp check's, with exit status 1 and
// nothing on standard output; the Phase 10 forms have no gate.
func TestExpandErrors(t *testing.T) {
	const bad = "package main\n\nfunc f!() int {\n\treturn 1\n}\n\nfunc main() { var s string = 1; _ = s }\n"
	r := expandIn(t, map[string]string{"main.gp": bad}, "main.gp")
	if r.code != 1 || r.stdout != "" || !strings.Contains(slashes(r.stderr), "./main.gp:7:30: cannot use 1") {
		t.Errorf("type error: %d %q\n%s", r.code, r.stdout, r.stderr)
	}
	c := expandIn(t, map[string]string{"main.gp": bad}, "main.gp")
	if code, _, errOut := gocpp("check", "main.gp"); code != c.code || errOut != c.stderr {
		t.Errorf("gocpp check printed something else:\n%s\nexpand:\n%s", errOut, c.stderr)
	}
	r = expandIn(t, map[string]string{"main.gp": "package main\n\nfunc main( {\n"}, "main.gp")
	if r.code != 1 || r.stdout != "" || !strings.Contains(slashes(r.stderr), "./main.gp:3:") {
		t.Errorf("syntax error: %d %q\n%s", r.code, r.stdout, r.stderr)
	}
	r = expandIn(t, map[string]string{}, "nothing.gp")
	if r.code != 1 || r.stdout != "" || r.stderr == "" {
		t.Errorf("missing file: %d %q\n%s", r.code, r.stdout, r.stderr)
	}
	// No gate: the Phase 10 forms expand at every phase (step 10.12, ADR-095), with no environment.
	inDir(t, map[string]string{"main.gp": "package main\n\nfunc main() { println(#line()) }\n"})
	for _, phase := range []string{"", "05"} {
		t.Setenv(gatePhaseEnv, phase)
		r = gocppRun("", false, "expand", "main.gp")
		if r.code != 0 || !strings.Contains(r.stdout, "println(3)") {
			t.Errorf("phase %q: %d %q\n%s", phase, r.code, r.stdout, r.stderr)
		}
	}
}

// slashes turns the backslashes of the paths in a message of gocpp (Windows) into slashes.
func slashes(s string) string { return strings.ReplaceAll(s, `\`, "/") }

// TestExpandUsage: -h prints the help, a bad flag is exit status 2, and the command is in gocpp help.
func TestExpandUsage(t *testing.T) {
	code, out, errOut := gocpp("expand", "-h")
	if code != 0 || out != "" || !strings.HasPrefix(errOut, "usage: gocpp expand ") || !strings.Contains(errOut, "-tags") {
		t.Errorf("expand -h = %d %q %q", code, out, errOut)
	}
	if code, _, errOut := gocpp("expand", "-nope"); code != 2 || !strings.Contains(errOut, "flag provided but not defined: -nope") {
		t.Errorf("expand -nope = %d %q", code, errOut)
	}
	_, help, _ := gocpp("help")
	if !regexp.MustCompile(`(?m)^\texpand   print code after macro, #Derive and ! rewriting$`).MatchString(help) {
		t.Errorf("gocpp help has no line for expand without a phase mark:\n%s", help)
	}
}

// expandRunFiles are the programs of tests/run whose expansion runs under real Go: the macros, the
// derive and the must programs that need no C++ and no standard package that Go lacks.
func expandRunFiles(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("..", "..", "tests", "run")
	var names []string
	for _, pattern := range []string{"umacro_*.gp", "derive_*.gp", "macro_*.gp", "must_*.gp"} {
		m, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, m...)
	}
	slices.Sort(names)
	return names
}

var (
	outputRx   = regexp.MustCompile(`(?s)\n// Output:(.*)$`)
	exitCodeRx = regexp.MustCompile(`(?m)^// ExitCode: (\d+)`)
)

// goStub is gocpp/must for real Go: Exit is what the runtime's gp::must_exit does.
const goStub = `package must

import (
	"fmt"
	"os"
	"path/filepath"
)

func Exit(err error) {
	if err != nil {
		name := filepath.Base(os.Args[0])
		fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
		os.Exit(1)
	}
}
`

// goErrors is a package for real Go that has what package errors has, and gocpp's own errors.Wrapf
// (stdlib/errors/wrapf.gp, divergence D17), for the programs that use it.
const goErrors = `package errors

import (
	"fmt"
	std "errors"
)

var ErrUnsupported = std.ErrUnsupported

func New(text string) error                  { return std.New(text) }
func Is(err, target error) bool              { return std.Is(err, target) }
func As(err error, target any) bool          { return std.As(err, target) }
func Unwrap(err error) error                 { return std.Unwrap(err) }
func Join(errs ...error) error               { return std.Join(errs...) }

type wrapfError struct {
	msg string
	err error
}

func (e *wrapfError) Error() string { return e.msg + ": " + e.err.Error() }
func (e *wrapfError) Unwrap() error { return e.err }

func Wrapf(errp *error, format string, args ...any) {
	if *errp == nil {
		return
	}
	*errp = &wrapfError{msg: fmt.Sprintf(format, args...), err: *errp}
}
`

var errorsImportRx = regexp.MustCompile(`(?m)^(import |\t)"errors"$`)

// TestExpandRunsAsGo: the expansion of a program of tests/run is Go that real Go runs, and it prints
// what the program's // Output: block says (which TestRunVsGo checks against the program's hand-written
// twin), with the same exit code. So the output of gocpp expand is not a picture of the compiler's work
// but the work itself.
func TestExpandRunsAsGo(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go command")
	}
	names := expandRunFiles(t)
	if len(names) < 40 {
		t.Fatalf("%d programs, want at least 40", len(names))
	}
	// A run of every program takes a few seconds: a sample of the must programs, all the others.
	step := 1
	if testing.Short() {
		step = 8
	}
	for i, path := range names {
		if i%step != 0 {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimSuffix(filepath.Base(path), ".gp")
		text := string(src)
		if !phase10Rx.MatchString(text) {
			continue
		}
		t.Run(name, func(t *testing.T) {
			if strings.Contains(text, "// divergence:") || regexp.MustCompile(`(?m)^\s*"(<|[^"]*\.h")`).MatchString(text) {
				t.Skip("not a program of real Go")
			}
			m := outputRx.FindStringSubmatch(text)
			if m == nil {
				t.Skip("no // Output:")
			}
			var want []string
			for _, l := range strings.Split(strings.TrimSpace(m[1]), "\n") {
				want = append(want, strings.TrimRight(strings.TrimPrefix(strings.TrimPrefix(l, "//"), " "), " "))
			}
			wantCode := 0
			if c := exitCodeRx.FindStringSubmatch(text); c != nil {
				wantCode, _ = strconv.Atoi(c[1])
			}
			r := expandIn(t, map[string]string{name + ".gp": text}, name+".gp")
			if r.code != 0 {
				t.Skipf("gocpp expand = %d (a program for a later gate or phase):\n%s", r.code, r.stderr)
			}
			mod := t.TempDir()
			prog := r.stdout
			if strings.Contains(prog, "errors.Wrapf(") {
				prog = errorsImportRx.ReplaceAllString(prog, `$1"gocpp/errors"`)
			}
			for file, data := range map[string]string{
				"go.mod":              "module gocpp\n\ngo 1.25\n",
				"must/must.go":        goStub,
				"errors/errors.go":    goErrors,
				"app/" + name + ".go": prog,
			} {
				p := filepath.Join(mod, filepath.FromSlash(file))
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			env := append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off", "GOTOOLCHAIN=local", "CGO_ENABLED=0", "GO111MODULE=on")
			exe := filepath.Join(mod, "app.exe")
			build := exec.Command(goBin, "build", "-o", exe, "./app")
			build.Dir, build.Env = mod, env
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("go does not build the expansion:\n%s\n%s", out, r.stdout)
			}
			cmd := exec.Command(exe)
			cmd.Dir, cmd.Env = mod, env
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			code := 0
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, l := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
				got = append(got, strings.TrimRight(l, " "))
			}
			if !slices.Equal(got, want) || code != wantCode && !(wantCode == 2 && code == 2) {
				t.Errorf("real Go runs the expansion of %s: exit %d (want %d), output:\n%s\nwant:\n%s\nexpansion:\n%s",
					name, code, wantCode, stdout.String(), strings.Join(want, "\n"), r.stdout)
			}
		})
	}
}

// TestExpandVerbatim: for the programs of tests/run that use no Phase 10 syntax, gocpp expand prints
// the file exactly as it is. This is the property that nothing but the expanded declarations changes.
func TestExpandVerbatim(t *testing.T) {
	dir := filepath.Join("..", "..", "tests", "run")
	paths, err := filepath.Glob(filepath.Join(dir, "*.gp"))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(paths)
	checked := 0
	for i, path := range paths {
		if i%7 != 0 && !testing.Short() { // a seventh of the programs is enough to see the property
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(src)
		if strings.Contains(text, "#") || strings.Contains(text, "!(") || regexp.MustCompile(`(?m)^\s*(\w+ )?"(<|[^"]*\.h")`).MatchString(text) {
			continue
		}
		f, err := gpparser.ParseFile(token.NewFileSet(), path, src, gpparser.Phase10Syntax|gpparser.SkipObjectResolution)
		if err != nil || len(gpast.Leftovers(f)) > 0 {
			continue
		}
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			r := expandIn(t, map[string]string{"main.gp": text}, "main.gp")
			if r.code != 0 {
				t.Skipf("does not check: %s", strings.SplitN(r.stderr, "\n", 3)[len(strings.SplitN(r.stderr, "\n", 3))-1])
			}
			if r.stdout != text {
				t.Errorf("gocpp expand changed %s:\n%s", name, lineDiff(text, r.stdout))
			}
		})
		checked++
	}
	if checked < 30 {
		t.Errorf("%d programs compared, want at least 30", checked)
	}
}

// expandSrc is the output of gocpp expand for one program.
func expandSrc(t *testing.T, src string) string {
	t.Helper()
	r := expandIn(t, map[string]string{"main.gp": src}, "main.gp")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("gocpp expand = %d\n%s", r.code, r.stderr)
	}
	return r.stdout
}

// TestExpandSplice: the text around an expanded declaration is the user's, byte for byte, and no comment is
// lost: a comment after a function whose signature gets a result, one on the line of its closing brace, the
// lines after it, the declarations that the check wrote and the import of gp_must.
func TestExpandSplice(t *testing.T) {
	tests := []struct{ name, src, want string }{
		{"comments after a must function",
			"package main\n\nimport \"os\"\n\nfunc f!() int {\n\tx := os.ReadFile(\"a\")\n\treturn len(x)\n}\n\n// section\n\nfunc g() {}\n\n// trailing\n",
			"package main\n\nimport \"os\"\n\nfunc f() (int, error) {\n\tx, must_e1 := os.ReadFile(\"a\")\n\tif must_e1 != nil {\n\t\treturn 0, must_e1\n\t}\n\treturn len(x), nil\n}\n\n// section\n\nfunc g() {}\n\n// trailing\n"},
		{"comments on the lines of the braces, and the user's blank lines",
			"package main\n\nimport \"os\"\n\nfunc f!() int { // open\n\tx := os.ReadFile(\"a\")\n\treturn len(x)\n} // close\n\nvar a = 1\nvar b = 2\nfunc g() {}\n\n\n\nvar c = 3\n",
			"package main\n\nimport \"os\"\n\nfunc f() (int, error) { // open\n\tx, must_e1 := os.ReadFile(\"a\")\n\tif must_e1 != nil {\n\t\treturn 0, must_e1\n\t}\n\treturn len(x), nil\n} // close\n\nvar a = 1\nvar b = 2\nfunc g() {}\n\n\n\nvar c = 3\n"},
		{"a comment after a macro declaration and a use",
			"package main\n\nmacro Ex(n int) int { return n } // c-decl\n\nfunc g() int { return #Ex(1) } // c-use\n\n// end\n",
			"package main\n\nfunc Ex(gp_file string, gp_line int, gp_func string, n int) int { return n } // c-decl\n\nfunc g() int { return Ex(\"main.gp\", 5, \"main.g\", 1) } // c-use\n\n// end\n"},
		{"main! without an import declaration",
			"package main\n\nfunc main!() { println(1) }\n",
			"package main\n\nimport gp_must \"gocpp/must\"\n\nfunc must_main() error { println(1); return nil }\n\nfunc main() { gp_must.Exit(must_main()) }\n"},
		{"main! with imports: the import follows the last import declaration",
			"package main\n\nimport \"fmt\"\nimport \"os\"\n\nvar _ = os.Args\n\nfunc main!() { fmt.Println(1) }\n",
			"package main\n\nimport \"fmt\"\nimport \"os\"\n\nimport gp_must \"gocpp/must\"\n\nvar _ = os.Args\n\nfunc must_main() error {\n\t{\n\t\t_, must_e1 := fmt.Println(1)\n\t\tif must_e1 != nil {\n\t\t\treturn must_e1\n\t\t}\n\t}\n\treturn nil\n}\n\nfunc main() { gp_must.Exit(must_main()) }\n"},
		{"a name that only the user's text has is no import",
			"package main\n\nmacro Ex() int { return 1 }\n\n// gp_must.Exit is not used here\nfunc g() int { return 2 }\n\nfunc h() int { return #Ex() }",
			"package main\n\nfunc Ex(gp_file string, gp_line int, gp_func string) int { return 1 }\n\n// gp_must.Exit is not used here\nfunc g() int { return 2 }\n\nfunc h() int { return Ex(\"main.gp\", 8, \"main.h\") }"},
		{"a derived method before a declaration that follows at once",
			"package main\n\n#Derive(Equal)\ntype P struct{ X int }\nfunc main() {}\n",
			"package main\n\ntype P struct{ X int }\n\nfunc (p P) Equal(other P) bool { return p == other }\n\nfunc main() {}\n"},
		{"a derived method at the end of a file with no line end",
			"package main\n\n#Derive(Equal)\ntype P struct{ X int }",
			"package main\n\ntype P struct{ X int }\n\nfunc (p P) Equal(other P) bool { return p == other }"},
		{"a user's long string at the end of a file stays",
			"package main\n\nvar long = #file() + \"" + strings.Repeat("0123456789", 30) + "\"",
			"package main\n\nvar long = \"main.gp\" + \"" + strings.Repeat("0123456789", 30) + "\""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := expandSrc(t, tc.src); got != tc.want {
				t.Errorf("gocpp expand:\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
	// The declarations that the check wrote are put in after the one before them, in the line ends of the file.
	t.Run("crlf", func(t *testing.T) {
		src := "package main\r\n\r\nimport \"os\"\r\n\r\n/* c1\r\n c2 */\r\nfunc f!() int {\r\n\tx := os.ReadFile(\"a\")\r\n\treturn len(x)\r\n}\r\n\r\nvar t = `x\r\ny`\r\n\r\nvar u = #file() + `p\r\n\r\n\r\n`\r\n\r\n#Derive(Equal)\r\ntype P struct{ X int }\r\n\r\nfunc main!() {}\r\n"
		got := expandSrc(t, src)
		if strings.Count(got, "\n") != strings.Count(got, "\r\n") {
			t.Errorf("a line end that is not CRLF in\n%q", got)
		}
		for _, want := range []string{"/* c1", "c2 */\r\nfunc f() (int, error) {", "}\r\n\r\nvar t = `x\r\ny`\r\n\r\nvar u = \"main.gp\" + `p\r\n\r\n\r\n`\r\n\r\ntype P", "type P struct{ X int }\r\n\r\nfunc (p P) Equal(other P) bool { return p == other }\r\n\r\nfunc must_main() error {"} {
			if !strings.Contains(got, want) {
				t.Errorf("no %q in\n%q", want, got)
			}
		}
	})
}

// TestExpandEmbedCut: the first bytes of an embedded file are cut between characters, and bytes that are not
// text are quoted.
func TestExpandEmbedCut(t *testing.T) {
	for name, data := range map[string]string{
		"é at the cut": "a" + strings.Repeat("é", 200),
		"binary":       strings.Repeat("\xff", 300),
	} {
		t.Run(name, func(t *testing.T) {
			files := map[string]string{"gocpp.mod": "module example.com/app\n\ngocpp 0.1\n", "d.bin": data,
				"main.gp": "package main\n\nfunc main() { println(len(#embed(\"d.bin\"))) }\n"}
			r := expandIn(t, files, ".")
			if r.code != 0 {
				t.Fatalf("exit %d\n%s", r.code, r.stderr)
			}
			want := `println(len("a` + strings.Repeat("é", 19) + `" /* 401 bytes in all */))`
			if name == "binary" {
				want = `println(len("` + strings.Repeat(`\xff`, 40) + `" /* 300 bytes in all */))`
			}
			if !strings.Contains(r.stdout, want) {
				t.Errorf("no %q in\n%s", want, r.stdout)
			}
		})
	}
}

// TestExpandFailures: a file that changes under gocpp is an internal error (exit status 2), and a failed write
// is exit status 1.
func TestExpandFailures(t *testing.T) {
	inDir(t, map[string]string{"main.gp": "package main\n\nfunc main() {}\n"})
	t.Setenv(gatePhaseEnv, "15")
	expandTestHook = func(f *load.File) { f.Src = []byte("package main\n") }
	defer func() { expandTestHook = nil }()
	r := gocppRun("", false, "expand", "main.gp")
	if r.code != 2 || r.stdout != "" || !strings.Contains(r.stderr, "changed while gocpp read it") {
		t.Errorf("a file that changed: %d %q\n%s", r.code, r.stdout, r.stderr)
	}
	expandTestHook = nil
	var errOut bytes.Buffer
	if code := run([]string{"expand", "main.gp"}, strings.NewReader(""), failingWriter{}, &errOut); code != 1 || !strings.Contains(errOut.String(), "gocpp expand: ") {
		t.Errorf("a failed write: %d %q", code, errOut.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

// commentsOf returns the comments of a program, one per element, without carriage returns and sorted.
func commentsOf(t *testing.T, src string) []string {
	t.Helper()
	f, err := gpparser.ParseFile(token.NewFileSet(), "x.gp", src, gpparser.Phase10Syntax|gpparser.ParseComments|gpparser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, g := range f.Comments {
		for _, c := range g.List {
			out = append(out, strings.Join(strings.Fields(strings.ReplaceAll(c.Text, "\r", "")), " "))
		}
	}
	slices.Sort(out)
	return out
}

// TestExpandKeepsComments: over the programs of tests/run, every comment of the program is in its expansion
// (a comment may move, it may not be lost), also the // Output: block at the end of a file. GOCPP_EXPAND_SWEEP=1
// takes every program, not every ninth.
func TestExpandKeepsComments(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "tests", "run", "*.gp"))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(paths)
	n := 0
	for i, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(src)
		phase10 := phase10Rx.MatchString(text) // the programs with the syntax of Phase 10 (their phase line names the latest gate they meet since step 10.12)
		if !phase10 && os.Getenv("GOCPP_EXPAND_SWEEP") != "1" && (i%9 != 0 || testing.Short()) {
			continue
		}
		if regexp.MustCompile(`(?m)^\s*(\w+ )?"(<|[^"]*\.h")`).MatchString(text) || strings.Contains(text, "// std-internal:") {
			continue
		}
		t.Run(filepath.Base(path), func(t *testing.T) {
			r := expandIn(t, map[string]string{"main.gp": text}, "main.gp")
			if r.code != 0 {
				t.Skip("does not check")
			}
			// No declaration is lost: the names of the source are in the expansion.
			have := declNames(t, r.stdout, false)
			for name := range declNames(t, text, true) {
				if !have[name] {
					t.Errorf("the declaration %s is lost", name)
				}
			}
			want, got := commentsOf(t, text), commentsOf(t, r.stdout)
			for _, c := range want {
				if i, ok := slices.BinarySearch(got, c); !ok {
					t.Errorf("the comment %q is lost", c)
				} else {
					got = slices.Delete(got, i, i+1)
				}
			}
		})
		n++
	}
	if n < 100 {
		t.Errorf("%d programs, want at least 100", n)
	}
}

// TestExpandStd: a package of the embedded standard library is printed from its copy in gocpp.
func TestExpandStd(t *testing.T) {
	inDir(t, nil)
	t.Setenv(gatePhaseEnv, "15")
	r := gocppRun("", false, "expand", "errors")
	if r.code != 0 || r.stderr != "" || !strings.Contains(r.stdout, "//// $GOCPP/stdlib/errors/wrapf.gp\n") || !strings.Contains(r.stdout, "func Wrapf(errp *error, format string, args ...any) {") {
		t.Errorf("gocpp expand errors = %d\n%s\n%.300s", r.code, r.stderr, r.stdout)
	}
}

// sharedPrefix is the start of the programs of TestExpandSharedLines, and what gocpp expand makes of it.
const (
	sharedPrefix = "package main\n\nimport \"os\"\n\nmacro Ex(a int) int { return a }\n\nvar _ = os.Args\n\n"
	sharedWant   = "package main\n\nimport \"os\"\n\nfunc Ex(gp_file string, gp_line int, gp_func string, a int) int { return a }\n\nvar _ = os.Args\n\n"
)

// TestExpandSharedLines: declarations that share a line with a declaration that is expanded are kept, in every
// order, and the text of the line around them stays.
func TestExpandSharedLines(t *testing.T) {
	const must = "{\n\tx, must_e1 := os.ReadFile(\"a\")\n\tif must_e1 != nil {\n\t\treturn 0, must_e1\n\t}\n\treturn len(x), nil\n}"
	tests := []struct{ name, src, want string }{
		{"changed, then unchanged", "var a = #Ex(1); var b = 2\n", "var a = Ex(\"main.gp\", 9, \"main.init\", 1); var b = 2\n"},
		{"unchanged, then changed", "var c = 3; var d = #Ex(4)\n", "var c = 3; var d = Ex(\"main.gp\", 9, \"main.init\", 4)\n"},
		{"two changed", "var e = #Ex(1); var f = #Ex(2) // end\n", "var e = Ex(\"main.gp\", 9, \"main.init\", 1); var f = Ex(\"main.gp\", 9, \"main.init\", 2) // end\n"},
		{"func, then func", "func h() int { return #Ex(5) }; func k() {}\n", "func h() int { return Ex(\"main.gp\", 9, \"main.h\", 5) }; func k() {}\n"},
		{"must func, then func", "func f1!() int { x := os.ReadFile(\"a\"); return len(x) }; func g1() {}\n", "func f1() (int, error) " + must + "; func g1() {}\n"},
		{"func, then must func", "func g2() {}; func f2!() int { x := os.ReadFile(\"a\"); return len(x) } // end\n", "func g2() {}; func f2() (int, error) " + must + " // end\n"},
		{"three", "var x = 1; func y!() { os.Remove(\"a\") }; var z = 2\n", "var x = 1; func y() error {\n\t{\n\t\tmust_e1 := os.Remove(\"a\")\n\t\tif must_e1 != nil {\n\t\t\treturn must_e1\n\t\t}\n\t}\n\treturn nil\n}; var z = 2\n"},
		{"a method of a derived type and the next declaration", "#Derive(Equal) /* after */\ntype P struct{ X int }; var n = 1\n", "/* after */\ntype P struct{ X int }; var n = 1\n\nfunc (p P) Equal(other P) bool { return p == other }\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got, want := expandSrc(t, sharedPrefix+tc.src), sharedWant+tc.want; got != want {
				t.Errorf("gocpp expand:\n%q\nwant:\n%q", got, want)
			}
		})
	}
}

// TestExpandDeriveLines: the comments on a line of #Derive stay, and a //line directive above it is no trouble.
func TestExpandDeriveLines(t *testing.T) {
	const methods = "\nfunc (t T) Equal(other T) bool { return t == other }\n"
	tests := []struct{ name, src, want string }{
		{"after", "#Derive(Equal) // note\ntype T struct{ X int }\n", "// note\ntype T struct{ X int }\n" + methods},
		{"before", "/* before */ #Derive(Equal)\ntype T struct{ X int }\n", "/* before */\ntype T struct{ X int }\n" + methods},
		{"block after", "#Derive(Equal) /* after */\ntype T struct{ X int }\n", "/* after */\ntype T struct{ X int }\n" + methods},
		{"both", "/* a */ #Derive(Equal) /* b */\ntype T struct{ X int }\n", "/* a */  /* b */\ntype T struct{ X int }\n" + methods},
		{"indented", "\t#Derive(Equal)\n\ttype T struct{ X int }\n", "\ttype T struct{ X int }\n" + methods},
		{"a line directive above", "//line a.go:10\n#Derive(Equal)\ntype T struct{ X int }\n", "//line a.go:10\ntype T struct{ X int }\n" + methods},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got, want := expandSrc(t, "package main\n\n"+tc.src), "package main\n\n"+tc.want; got != want {
				t.Errorf("gocpp expand:\n%q\nwant:\n%q", got, want)
			}
		})
	}
	// emit checks the derived code too: the directive gives it a place.
	inDir(t, map[string]string{"main.gp": "package main\n\n//line a.go:10\n#Derive(Equal)\ntype T struct{ X int }\n\nfunc main() {}\n"})
	t.Setenv(gatePhaseEnv, "15")
	if r := gocppRun("", false, "emit", "main.gp"); r.code != 0 || !strings.Contains(r.stdout, "Equal") {
		t.Errorf("gocpp emit = %d\n%s", r.code, r.stderr)
	}
}

// TestExpandMacroLines: the comments on the lines of macros above a func, and the func's own, go above it.
func TestExpandMacroLines(t *testing.T) {
	const decls = "macro Wr() { #body() }\nmacro Tr() func() { return nil }\n\n"
	got := expandSrc(t, "package main\n\n"+decls+"// doc\n#Wr() // note on wrap line\n#Tr() /* note on tr */\n// middle\nfunc a() int { return 1 }\n\n#Tr() // only\n// doc2\nfunc b() {}\n")
	for _, want := range []string{
		"// doc\n// note on wrap line\n/* note on tr */\n// middle\nfunc a() (gp_r0 int) {\n",
		"// only\n// doc2\nfunc b() {\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, "(//") || strings.Contains(got, "(/*") {
		t.Errorf("a comment is in a parameter list:\n%s", got)
	}
}

// TestExpandMixedLineEnds: a declaration is printed with the line ends of its own first line, and the
// declarations that the check wrote with those of the line before them.
func TestExpandMixedLineEnds(t *testing.T) {
	src := "package main\n\nimport \"os\"\n\nfunc a() {}\r\n\r\nfunc f!() int {\r\n\tx := os.ReadFile(\"a\")\r\n\treturn len(x)\r\n}\r\n\nfunc g!() int {\n\tx := os.ReadFile(\"a\")\n\treturn len(x)\n}\n"
	got := expandSrc(t, src)
	f, g, _ := strings.Cut(got, "func g() (int, error) {")
	if !strings.Contains(f, "func f() (int, error) {\r\n\tx, must_e1 := os.ReadFile(\"a\")\r\n") || strings.Count(f, "\n") != strings.Count(f, "\r\n")+5 {
		t.Errorf("f is not in CRLF:\n%q", f)
	}
	if strings.Contains(g, "\r") {
		t.Errorf("g has a CRLF:\n%q", g)
	}
	// The line ends of the declaration, not those of the line before it, and a multi-line comment above it.
	got = expandSrc(t, "package main\n\nimport \"os\"\n\nfunc a() {}\n\n/* doc\r\n   more */\r\nfunc f!() int {\r\n\tx := os.ReadFile(\"a\")\r\n\treturn len(x)\r\n}\r\n")
	if want := "func a() {}\n\n/* doc\r\n   more */\r\nfunc f() (int, error) {\r\n\tx, must_e1 := os.ReadFile(\"a\")\r\n"; !strings.Contains(got, want) {
		t.Errorf("no %q in\n%q", want, got)
	}
	// A file in CRLF whose last line has no line end.
	got = expandSrc(t, "package main\r\n\r\nimport \"os\"\r\n\r\nfunc f!() int {\r\n\tx := os.ReadFile(\"a\")\r\n\treturn len(x)\r\n}")
	if strings.Count(got, "\n") != strings.Count(got, "\r\n") || strings.HasSuffix(got, "\n") {
		t.Errorf("CRLF, no line end at the end:\n%q", got)
	}
	got = expandSrc(t, "package main\r\n\r\nfunc main!() {}")
	if strings.Count(got, "\n") != strings.Count(got, "\r\n") || strings.HasSuffix(got, "\n") {
		t.Errorf("CRLF, main!:\n%q", got)
	}
}

// TestApplyEdits: edits that overlap are an error, never text that is left out.
func TestApplyEdits(t *testing.T) {
	src := []byte("0123456789\n")
	if out, err := applyEdits(src, []edit{{start: 2, end: 4, text: "ab"}, {start: 6, end: 8, text: "cd"}, {start: 4, end: 4, text: "x"}}); err != nil || string(out) != "01abx45cd89\n" {
		t.Errorf("edits: %q, %v", out, err)
	}
	for name, edits := range map[string][]edit{
		"overlap":  {{start: 2, end: 6}, {start: 4, end: 8}},
		"inside":   {{start: 2, end: 8}, {start: 4, end: 6, text: "x"}},
		"backward": {{start: 6, end: 2}},
		"past end": {{start: 2, end: 40}},
	} {
		if out, err := applyEdits(src, edits); err == nil || out != nil {
			t.Errorf("%s: %q, %v; want an error", name, out, err)
		}
	}
}

// phase10Rx finds the syntax of Phase 10 in a program: a name with a ! right before ( or [, a macro use #Name, or a macro
// declaration. It looks at the text, so it may find one in a string or a comment: that only makes the sweep run a program more.
var phase10Rx = regexp.MustCompile(`(?m)\w![(\[]|#[A-Za-z_]|^macro \w`)

// declNames returns the names of the declarations of a program: functions (methods with their receiver),
// types, variables and constants, without the ! and the # of the macros and the must functions.
func declNames(t *testing.T, src string, phase10 bool) map[string]bool {
	t.Helper()
	mode := gpparser.ParseComments | gpparser.SkipObjectResolution
	if phase10 {
		mode |= gpparser.Phase10Syntax
	}
	f, err := gpparser.ParseFile(token.NewFileSet(), "x.gp", src, mode)
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool)
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			name := gpast.MacroBase(gpast.MustBase(d.Name.Name))
			if d.Recv != nil && len(d.Recv.List) > 0 {
				name = types.ExprString(d.Recv.List[0].Type) + "." + name
			}
			names[name] = true
		case *ast.GenDecl:
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					names[s.Name.Name] = true
				case *ast.ValueSpec:
					for _, n := range s.Names {
						if n.Name != "_" { // also the line of a macro on top of a declaration
							names[n.Name] = true
						}
					}
				}
			}
		}
	}
	return names
}
