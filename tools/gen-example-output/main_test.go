package main

import (
	"bytes"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// writeFiles creates the files (path: contents) under dir.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, data := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

const helloGo = "// phase: 02\n\npackage main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hi\") }\n"

func TestReadHeader(t *testing.T) {
	tests := []struct {
		src      string
		phase    int
		exitCode int
		err      string
	}{
		{helloGo, 2, 0, ""},
		{"// phase: 10\n// ExitCode: 3\n\npackage main\n", 10, 3, ""},
		{"// phase: 04\n// pkg-config: raylib\n\n// Doc.\n//\n//gocpp:pkg-config raylib\npackage main\n", 4, 0, ""},
		{"// phase: 14\n\n//go:build windows\n\n// ExitCode: 1\npackage main\n", 14, 1, ""},
		{"// phase: 02\npackage main\n\n// ExitCode: 3\n", 2, 0, ""}, // after the package clause: not header
		{"package main\n", 0, 0, `first line must be "// phase: NN"`},
		{"// phase: 2\npackage main\n", 0, 0, `first line must be`},
		{"// phase: 16\npackage main\n", 0, 0, "phase 16 does not exist"},
		{"", 0, 0, "empty"},
	}
	for _, tt := range tests {
		path := filepath.Join(t.TempDir(), "main.gp")
		writeFiles(t, filepath.Dir(path), map[string]string{"main.gp": tt.src})
		h, err := readHeader(path)
		switch {
		case tt.err != "":
			if err == nil || !strings.Contains(err.Error(), tt.err) {
				t.Errorf("%q: error %v, want one with %q", tt.src, err, tt.err)
			}
		case err != nil:
			t.Errorf("%q: %v", tt.src, err)
		case h.phase != tt.phase || h.exitCode != tt.exitCode:
			t.Errorf("%q: phase %d, exit code %d; want %d, %d", tt.src, h.phase, h.exitCode, tt.phase, tt.exitCode)
		}
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		kind  string
		err   string
	}{
		{"go", map[string]string{"main.gp": helloGo, "main_test.gp": "package main\n"}, kindGo, ""},
		{"header", map[string]string{"main.gp": "// phase: 04\npackage main\n\nimport \"<vector>\"\n"}, kindInterop, ""},
		{"local header", map[string]string{"main.gp": "// phase: 04\npackage main\n\nimport \"./x.h\"\n"}, kindInterop, ""},
		{"namespace", map[string]string{"main.gp": "// phase: 04\npackage main\n\nimport fs \"::std::filesystem\"\n"}, kindInterop, ""},
		{"c file", map[string]string{"main.gp": helloGo, "x.c": "int x;\n"}, kindInterop, ""},
		{"gocpp.mod", map[string]string{"main.gp": helloGo, "gocpp.mod": "module m\n"}, kindInterop, ""},
		{"twin", map[string]string{"main.gp": "// phase: 10\npackage main\n\nfunc main!() {}\n", "twin/go.mod": "module t\n"}, kindTwin, ""},
		{"twin without go.mod", map[string]string{"main.gp": helloGo, "twin/main.go": "package main\n"}, "", "twin/ has no go.mod"},
		{"gocpp syntax, no twin", map[string]string{"main.gp": "// phase: 10\npackage main\n\nfunc main!() {}\n"}, "", "needs one"},
	}
	for _, tt := range tests {
		dir := t.TempDir()
		writeFiles(t, dir, tt.files)
		kind, err := classify(dir)
		if tt.err != "" {
			if err == nil || !strings.Contains(err.Error(), tt.err) {
				t.Errorf("%s: error %v, want one with %q", tt.name, err, tt.err)
			}
			continue
		}
		if err != nil || kind != tt.kind {
			t.Errorf("%s: got %q, %v; want %q", tt.name, kind, err, tt.kind)
		}
	}
}

func TestIsHeaderImport(t *testing.T) {
	for path, want := range map[string]bool{
		"fmt": false, "example.com/m/geom": false, "<vector>": true, "<SDL2/SDL.h>": true,
		"./physics.h": true, "native/x.hpp": true, "::std::filesystem": true, "a.b/c": false,
	} {
		if got := isHeaderImport(path); got != want {
			t.Errorf("isHeaderImport(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestReadArgsAndEnv(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"args.txt":  "-n  -i (a|b)\\b x.txt\n",
		"bad.txt":   "one\ntwo\n",
		"env.txt":   "A=1\n\nB=x=y\r\n",
		"env2.txt":  "NOEQUALS\n",
		"empty.txt": "",
	})
	args, err := readArgs(filepath.Join(dir, "args.txt"))
	if want := []string{"-n", "-i", `(a|b)\b`, "x.txt"}; err != nil || !slices.Equal(args, want) {
		t.Errorf("readArgs = %q, %v; want %q", args, err, want)
	}
	if args, err := readArgs(filepath.Join(dir, "missing.txt")); err != nil || args != nil {
		t.Errorf("missing args.txt: %q, %v", args, err)
	}
	if args, err := readArgs(filepath.Join(dir, "empty.txt")); err != nil || len(args) != 0 {
		t.Errorf("empty args.txt: %q, %v", args, err)
	}
	if _, err := readArgs(filepath.Join(dir, "bad.txt")); err == nil {
		t.Error("args.txt with two lines: no error")
	}
	env, err := readEnv(filepath.Join(dir, "env.txt"))
	if want := []string{"A=1", "B=x=y"}; err != nil || !slices.Equal(env, want) {
		t.Errorf("readEnv = %q, %v; want %q", env, err, want)
	}
	if _, err := readEnv(filepath.Join(dir, "env2.txt")); err == nil {
		t.Error("env.txt line without =: no error")
	}
}

func TestGocppToGo(t *testing.T) {
	// gofmt-clean gocpp code must map to gofmt-clean Go of the same shape.
	src := `package main

import (
	"fmt"

	"<map>"
)

macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: %s: %s", #file(), #line(), #text(cond), msg()))
	}
}

#Derive(String)
type Level int

#Trace()
func load!(path string) int {
	m := std.map[std.string, int]{}
	n := strconv.Atoi!(path)
	x := node.type()
	#Assert(n > 0, "positive")
	switch v := any(n).(type) {
	case int:
		_ = v
	}
	return n + #Dbg(len(m)) + x
}
`
	got := gocppToGo([]byte(src))
	for _, want := range []string{"func Assert(cond bool, msg string)", "// ONTOP Derive(String)", "// ONTOP Trace()",
		"func loadǃ(path string)", "std.MAP[std.string, int]", "strconv.Atoiǃ(path)", "node.TYPE()",
		"ǂAssert(n > 0", "ǂfile()", ".(type)", "ǂDbg(len(m))"} {
		if !bytes.Contains(got, []byte(want)) {
			t.Errorf("mapped source has no %q:\n%s", want, got)
		}
	}
	formatted, err := format.Source(got)
	if err != nil {
		t.Fatalf("mapped source does not parse: %v\n%s", err, got)
	}
	if !bytes.Equal(formatted, got) {
		t.Errorf("mapped source is not gofmt-clean: %s", firstDiff(formatted, got))
	}
}

// cppTypes uses C++ names in types: a constant type argument (D4) and
// dotted names (D20), which the stock parser rejects there.
const cppTypes = `// phase: 04

package main

import (
	"<array>"
	"<bitset>"
	"<chrono>"
	"<filesystem>"
)

type timer struct {
	d   std.chrono.seconds
	buf std.array[byte, 256]
}

func f(p std.filesystem.path, xs std.array[std.array[int, 2], 3]) std.chrono.milliseconds {
	var bits std.bitset[64]
	_ = bits.count()
	return std.chrono.duration_cast[std.chrono.milliseconds](std.chrono.seconds(1))
}
`

func TestCheckFormat(t *testing.T) {
	tests := []struct {
		name, src, kind string
		err             string // "" for no error
	}{
		{"go, clean", helloGo, kindGo, ""},
		{"go, badly formatted", "// phase: 02\n\npackage main\nfunc main() {  }\n", kindGo, "not gofmt-clean"},
		{"go, broken", "// phase: 02\n\npackage main\n\nfunc main() {{{ oops\n", kindGo, "does not parse"},
		{"twin, badly formatted", "// phase: 10\n\npackage main\n\nfunc f!() {\n\tg!( )\n}\n", kindTwin, "not gofmt-clean"},
		{"twin, broken", "// phase: 10\n\npackage main\n\nfunc main!() {\n\tf!( )(\n}\n", kindTwin, "does not parse, also with"},
		{"interop, broken", "// phase: 04\n\npackage main\n\nimport \"<vector>\"\n\nfunc main() {{{ oops\n", kindInterop, "does not parse, also with"},
		{"interop, C++ names in types", cppTypes, kindInterop, ""},
		{"interop, C++ names in types, badly formatted", strings.Replace(cppTypes, "byte, 256", "byte,256", 1), kindInterop, "not gofmt-clean"},
		{"interop, not mapped yet", "// phase: 04\n\npackage main\n\nimport \"<vector>\"\n\nvar it std.vector[int].iterator\n", kindInterop, "extend gocppToGo"},
	}
	for _, tt := range tests {
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{"main.gp": tt.src})
		err := checkFormat(dir, tt.kind)
		if tt.err == "" && err != nil || tt.err != "" && (err == nil || !strings.Contains(err.Error(), tt.err)) {
			t.Errorf("%s: error %v, want %q", tt.name, err, tt.err)
		}
	}
}

func TestMapCppNames(t *testing.T) {
	tests := []struct{ in, want string }{
		{"var buf std.array[byte, 256]", "var buf std.array[byte, NNN]"},
		{"x := std.array[std.array[int, 2], 3]{}", "x := std.array[std.array[int, N], N]{}"},
		{"var d std.chrono.seconds", "var d std.chrono_seconds"},
		{"t := std.chrono.steady_clock.now()", "t := std.chrono_steady_clock_now()"},
		{"f(p std.filesystem.path) a.b", "f(p std.filesystem_path) a.b"},
		// Go code stays Go of the same shape; other numbers stay as they are.
		{"a[5], s[1:2], f(1, 2), []int{1, 2, 3}, [4]int{}, x[i+1]", "a[N], s[1:2], f(1, 2), []int{1, 2, 3}, [N]int{}, x[i+1]"},
		{"m[0x10], m[1.5], m[-1]", "m[0x10], m[1.5], m[-1]"},
		{"a . b . c, a.b .c", "a . b . c, a.b .c"},
		// Strings and comments don't change.
		{`s := "a.b.c[1, 2]" // x.y.z[3]`, `s := "a.b.c[1, 2]" // x.y.z[3]`},
	}
	for _, tt := range tests {
		if got := string(mapCppNames([]byte(tt.in))); got != tt.want {
			t.Errorf("mapCppNames(%q)\n  = %q\nwant %q", tt.in, got, tt.want)
		}
	}
}

func TestCheckTwinSites(t *testing.T) {
	gp := "// phase: 10\n\npackage main\n\n#Trace()\nfunc f() {\n\t#Assert(x > 0, \"msg\")\n}\n"
	twin := func(sites string) string {
		return "package main\n\ntype Site struct {\n\tFile string\n\tLine int\n\tArgs []string\n}\n\nvar (\n" + sites + ")\n"
	}
	tests := []struct {
		sites string
		err   string
	}{
		{"\ts1 = Site{File: \"main.gp\", Line: 5}\n\ts2 = Site{File: \"main.gp\", Line: 7, Args: []string{\"x > 0\", `\"msg\"`}}\n", ""},
		{"\ts1 = Site{File: \"main.gp\", Line: 6}\n", "has no macro use"},
		{"\ts1 = Site{File: \"main.gp\", Line: 7, Args: []string{\"x >= 0\"}}\n", `does not contain "x >= 0"`},
		{"\ts1 = Site{File: \"main.gp\", Line: 70}\n", "has no line 70"},
		{"\ts1 = Site{File: \"other.gp\", Line: 1}\n", "other.gp"},
		{"\ts1 = Site{File: \"main.go\", Line: 1000}\n", ""}, // not a .gp file: not a site
	}
	for _, tt := range tests {
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{"main.gp": gp, "twin/go.mod": "module t\n", "twin/main.go": twin(tt.sites)})
		err := checkTwinSites(dir)
		if tt.err == "" && err != nil || tt.err != "" && (err == nil || !strings.Contains(err.Error(), tt.err)) {
			t.Errorf("sites %q: error %v, want %q", tt.sites, err, tt.err)
		}
	}
}

func TestFirstDiff(t *testing.T) {
	for _, tt := range []struct{ want, got, diff string }{
		{"a\nb\n", "a\nc\n", `line 2: want "b\n", got "c\n"`},
		{"a\n", "a\nb\n", `line 2: unexpected "b\n"`},
		{"a\nb\n", "a\n", `line 2: missing "b\n"`},
		{"a\nb", "a\nb\n", `line 2: want "b", got "b\n"`},
	} {
		if got := firstDiff([]byte(tt.want), []byte(tt.got)); got != tt.diff {
			t.Errorf("firstDiff(%q, %q) = %s, want %s", tt.want, tt.got, got, tt.diff)
		}
	}
}

// TestHandle runs a whole small example of each kind.
func TestHandle(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go command")
	}
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"hello/main.gp":     "// phase: 02\n// ExitCode: 3\n\npackage main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() {\n\tfmt.Println(os.Args[1:], os.Getenv(\"GREETING\"))\n\tos.Exit(3)\n}\n",
		"hello/args.txt":    "a  b\n",
		"hello/env.txt":     "GREETING=hi\n",
		"echo/main.gp":      "// phase: 09\n\npackage main\n\nimport (\n\t\"io\"\n\t\"os\"\n)\n\nfunc main() { io.Copy(os.Stdout, os.Stdin) }\n",
		"echo/stdin.txt":    "from stdin\n",
		"echo/output.txt":   "stale\n",
		"cpp/main.gp":       "// phase: 04\n\npackage main\n\nimport \"<vector>\"\n",
		"cpp/output.txt":    "by hand\n",
		"nocpp/main.gp":     "// phase: 04\n\npackage main\n\nimport \"<vector>\"\n",
		"twin/main.gp":      "// phase: 10\n\npackage main\n\nfunc main!() {}\n",
		"twin/twin/go.mod":  "module t\n\ngo 1.25\n",
		"twin/twin/main.go": "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"twin\") }\n",
	})
	cfg := config{check: true, out: &bytes.Buffer{}}
	if _, err := handle(cfg, filepath.Join(root, "echo")); err == nil || !strings.Contains(err.Error(), "out of date") {
		t.Errorf("echo with -check: %v, want out of date", err)
	}
	if _, err := handle(cfg, filepath.Join(root, "hello")); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("hello with -check: %v, want missing", err)
	}
	if _, err := handle(cfg, filepath.Join(root, "nocpp")); err == nil {
		t.Error("interop example without output.txt: no error")
	}
	cfg.check = false
	for name, want := range map[string]string{"hello": "[a b] hi\n", "echo": "from stdin\n", "twin": "twin\n", "cpp": "by hand\n"} {
		if _, err := handle(cfg, filepath.Join(root, name)); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if got, _ := os.ReadFile(filepath.Join(root, name, "output.txt")); string(got) != want {
			t.Errorf("%s: output.txt is %q, want %q", name, got, want)
		}
	}
	cfg.check = true
	if err := run(cfg, root, []string{"hello", "echo", "twin", "cpp"}); err != nil {
		t.Errorf("check after writing: %v\n%s", err, cfg.out)
	}
	writeFiles(t, root, map[string]string{"hello/main.gp": strings.Replace(
		"// phase: 02\n// ExitCode: 3\n\npackage main\n\nfunc main() {}\n", "ExitCode: 3", "ExitCode: 4", 1)})
	if _, err := handle(cfg, filepath.Join(root, "hello")); err == nil || !strings.Contains(err.Error(), "exited with 0, want 4") {
		t.Errorf("wrong exit code: %v", err)
	}
}

// TestExamples checks that every output.txt in examples/ is up to date, the
// same check as "go run ./tools/gen-example-output -check".
func TestExamples(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs every example; skipped with -short")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go command")
	}
	var out bytes.Buffer
	if err := run(config{check: true, out: &out}, filepath.Join("..", "..", "examples"), nil); err != nil {
		t.Errorf("%v\n%s\nRun \"go run ./tools/gen-example-output\" in the repository root to update the outputs.", err, out.String())
	}
}
