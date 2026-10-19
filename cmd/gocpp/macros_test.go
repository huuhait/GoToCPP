package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The tests of the built-in macros (#file, #line, #func, #text, #embed) and of //go:embed through
// the gocpp commands (Phase 10 step 10.8): no phase gate (step 10.12), the errors of the directive (which
// sit on the directive's own line, so an error test file can't hold their comments), the program
// that a rebuild makes after an embedded file changed, a big file, the formatter, and the C++ of
// a program without macros staying the same.

// TestMacroNoGate: the built-in macros and //go:embed check at every phase, with no
// GOCPP_GATE_PHASE (step 10.12, ADR-095).
func TestMacroNoGate(t *testing.T) {
	macro := map[string]string{"main.go": "package main\n\nfunc main() { println(#line()) }\n"}
	embed := map[string]string{
		"main.go": "package main\n\nimport _ \"embed\"\n\n//go:embed a.txt\nvar a string\n\nfunc main() { println(a) }\n",
		"a.txt":   "x",
	}
	for _, phase := range []string{"", "05", "10"} {
		t.Run("macro "+phase, func(t *testing.T) {
			t.Setenv(gatePhaseEnv, phase)
			if r := checkIn(t, macro, "check", "main.go"); r.code != 0 {
				t.Errorf("exit %d:\n%s", r.code, show(r))
			}
		})
		t.Run("embed "+phase, func(t *testing.T) {
			t.Setenv(gatePhaseEnv, phase)
			if r := checkIn(t, embed, "check", "main.go"); r.code != 0 {
				t.Errorf("exit %d:\n%s", r.code, show(r))
			}
		})
	}
}

// TestGoEmbedDirectiveErrors: the errors of the //go:embed line, as the go command words them, at the
// place where it reports them (the pattern, or the start of the line).
func TestGoEmbedDirectiveErrors(t *testing.T) {
	const head = "package main\n\nimport _ \"embed\"\n\n"
	tests := []struct {
		name, src string
		want      string
	}{
		{"missing", "//go:embed nope.txt\nvar a string\n", "./main.go:5:12: pattern nope.txt: no matching files found"},
		{"no wildcard match", "//go:embed *.zip\nvar a string\n", "./main.go:5:12: pattern *.zip: no matching files found"},
		{"escapes", "//go:embed ../a.txt\nvar a string\n", "./main.go:5:12: pattern ../a.txt: invalid pattern syntax"},
		{"absolute", "//go:embed /etc/passwd\nvar a string\n", "./main.go:5:12: pattern /etc/passwd: invalid pattern syntax"},
		{"bad glob", "//go:embed [a\nvar a string\n", "./main.go:5:12: pattern [a: invalid pattern syntax"},
		{"no pattern", "//go:embed\nvar a string\n", "./main.go:5:1: usage: //go:embed pattern..."},
		{"two files", "//go:embed a.txt b.txt\nvar a string\n", "./main.go:6:5: invalid go:embed: multiple files for type string"},
		{"folder", "//go:embed d\nvar a []byte\n", "./main.go:6:5: invalid go:embed: multiple files for type []byte"},
		{"type", "//go:embed a.txt\nvar a int\n", "./main.go:6:5: go:embed cannot apply to var of type int"},
		{"initializer", "//go:embed a.txt\nvar a string = \"x\"\n", "./main.go:6:5: go:embed cannot apply to var with initializer"},
		{"several names", "//go:embed a.txt\nvar a, b string\n", "./main.go:6:5: go:embed cannot apply to multiple vars"},
		{"const", "//go:embed a.txt\nconst a = \"x\"\n", "./main.go:5:1: misplaced go:embed directive"},
		{"inside func", "func f() {\n\t//go:embed a.txt\n\tvar a string\n\t_ = a\n}\n", "./main.go:6:2: go:embed cannot apply to var inside func"},
		{"symlink", "//go:embed link.txt\nvar a string\n", "./main.go:5:12: pattern link.txt: cannot embed irregular file link.txt"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tree := map[string]string{
				"main.go": head + tc.src + "\nfunc main() {}\n",
				"a.txt":   "a", "b.txt": "b", "d/x.txt": "x", "d/y.txt": "y",
			}
			if tc.name == "symlink" {
				tree["link.txt"] = "symlink:a.txt"
			}
			r := checkIn(t, tree, "check", "main.go")
			if r.code != 1 || !strings.Contains(r.stderr, tc.want+"\n") {
				t.Errorf("exit %d, want 1 and %q:\n%s", r.code, tc.want, show(r))
			}
		})
	}
}

// TestEmbedRebuild: a changed embedded file changes the program at the next run (the C++ text holds
// the bytes, so the object cache can't give the old object), also for a file that a pattern added.
func TestEmbedRebuild(t *testing.T) {
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	t.Setenv("CXX", "")
	dir := inDir(t, map[string]string{
		"main.gp": "package main\n\nimport (\n\t_ \"embed\"\n\t\"fmt\"\n)\n\n//go:embed a.txt\nvar a string\n\nfunc main() { fmt.Println(a, #embed(\"b.txt\")) }\n",
		"a.txt":   "one",
		"b.txt":   "two",
	})
	run := func() string {
		r := gocppRun("", false, "run", "main.gp")
		if r.code != 0 {
			t.Fatalf("exit %d:\n%s", r.code, show(r))
		}
		return r.stdout
	}
	if got := run(); got != "one two\n" {
		t.Fatalf("first run: %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("ONE!"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := run(); got != "ONE! two\n" {
		t.Errorf("after changing the //go:embed file: %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("TWO!"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := run(); got != "ONE! TWO!\n" {
		t.Errorf("after changing the #embed file: %q", got)
	}
	if err := os.Remove(filepath.Join(dir, "b.txt")); err != nil {
		t.Fatal(err)
	}
	if r := gocppRun("", false, "run", "main.gp"); r.code != 1 || !strings.Contains(r.stderr, "main.gp:11:") || !strings.Contains(r.stderr, "no matching files found") {
		t.Errorf("after removing the #embed file: exit %d:\n%s", r.code, show(r))
	}
}

// TestEmbedBigFile: three megabytes (the C++ compiler takes it as a string literal) arrive intact, as
// a string and as a []byte.
func TestEmbedBigFile(t *testing.T) {
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	t.Setenv("CXX", "")
	data := make([]byte, 3<<20)
	rand.New(rand.NewSource(11)).Read(data)
	var h uint64
	for _, b := range data {
		h = h*31 + uint64(b)
	}
	inDir(t, map[string]string{
		"main.gp": "package main\n\nimport (\n\t_ \"embed\"\n\t\"fmt\"\n)\n\n//go:embed big.bin\nvar s string\n\n//go:embed big.bin\nvar b []byte\n\n" +
			"func main() {\n\tvar h uint64\n\tfor _, c := range b {\n\t\th = h*31 + uint64(c)\n\t}\n\tfmt.Println(len(s), len(b), s == string(b), h)\n}\n",
	})
	if err := os.WriteFile("big.bin", data, 0o644); err != nil {
		t.Fatal(err)
	}
	r := gocppRun("", false, "run", "main.gp")
	want := fmt.Sprintf("%d %d true %d\n", len(data), len(data), h)
	if r.code != 0 || r.stdout != want {
		t.Errorf("exit %d, want %q:\n%s", r.code, want, show(r))
	}
}

// TestEmbedTooBig: a file over the limit is an error, not an out-of-memory in the C++ compiler.
func TestEmbedTooBig(t *testing.T) {
	dir := inDir(t, map[string]string{
		"main.gp": "package main\n\nimport _ \"embed\"\n\n//go:embed big.bin\nvar s string\n\nfunc main() { println(len(s)) }\n",
	})
	f, err := os.Create(filepath.Join(dir, "big.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(17 << 20); err != nil {
		t.Fatal(err)
	}
	f.Close()
	r := gocppRun("", false, "check", "main.gp")
	if r.code != 1 || !strings.Contains(r.stderr, "main.gp:5:12: cannot embed big.bin: 17825792 bytes is more than the limit of 16 MiB for one file") {
		t.Errorf("exit %d:\n%s", r.code, show(r))
	}
}

// TestMacroEmitUnchanged: a program without macros gets the same C++ at Phase 10 as before it.
func TestMacroEmitUnchanged(t *testing.T) {
	src := "package main\n\nimport \"fmt\"\n\n// a // comment, and //go:embed in a string \"//go:embed x\"\nvar s = \"#line() #file()\"\n\nfunc main() {\n\tfor i := 0; i < 2; i++ {\n\t\tfmt.Println(s, i, `#text(x)`)\n\t}\n}\n"
	var outs []string
	for _, phase := range []string{"05", "10"} {
		t.Setenv(gatePhaseEnv, phase)
		r := checkIn(t, map[string]string{"main.go": src}, "emit", "main.go")
		if r.code != 0 {
			t.Fatalf("phase %s: exit %d:\n%s", phase, r.code, show(r))
		}
		outs = append(outs, r.stdout)
	}
	if outs[0] != outs[1] {
		t.Errorf("the C++ differs between Phase 05 and 10:\n%s\n----\n%s", outs[0], outs[1])
	}
}

// TestMacroEmitLines: the C++ of an expansion keeps the line of the use for the debugger and the
// compiler's messages, and holds the constant.
func TestMacroEmitLines(t *testing.T) {
	src := "package main\n\nfunc f() string { return #func() }\n\nfunc main() {\n\n\tprintln(#file(), #line(), f(), #text(a +\n\t\tb))\n}\n"
	r := checkIn(t, map[string]string{"main.go": src}, "emit", "main.go")
	if r.code != 0 {
		t.Fatalf("exit %d:\n%s", r.code, show(r))
	}
	for _, w := range []string{"\"main.go\"_gs", "\"main.f\"_gs", "\"a + b\"_gs", "#line 7 "} {
		if !strings.Contains(strings.ReplaceAll(r.stdout, "main.gp", "main.go"), w) {
			t.Errorf("no %q in\n%s", w, r.stdout)
		}
	}
}

// TestMacroFmt: gocpp fmt writes the built-in macros back as they are, and a second run changes nothing.
func TestMacroFmt(t *testing.T) {
	src := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(#file(),   #line(), #func(), #text(a+  b),\n\t\t#embed(\"a.txt\"))\n}\n"
	inDir(t, map[string]string{"main.gp": src, "a.txt": "x"})
	r := gocppRun("", false, "fmt", "-w", "main.gp")
	if r.code != 0 {
		t.Fatalf("fmt: exit %d:\n%s", r.code, show(r))
	}
	first, _ := os.ReadFile("main.gp")
	if !strings.Contains(string(first), "fmt.Println(#file(), #line(), #func(), #text(a+b),") {
		t.Errorf("formatted:\n%s", first)
	}
	if r := gocppRun("", false, "fmt", "-d", "main.gp"); r.code != 0 || r.stdout != "" {
		t.Errorf("second fmt: exit %d:\n%s", r.code, show(r))
	}
}

// TestEmbedStampSeesNewFiles: a file added to a folder that an embed pattern listed makes the next build
// "not up to date", also when its extension is none that the loader looks at (.dat); a removed file too.
func TestEmbedStampSeesNewFiles(t *testing.T) {
	for _, tc := range []struct{ name, pattern string }{{"glob", "*.dat"}, {"folder", "dir"}} {
		t.Run(tc.name, func(t *testing.T) {
			withOneCXX(t, func(t *testing.T, c testCXX) {
				t.Setenv("XDG_CACHE_HOME", t.TempDir())
				cacheOn(t)
				t.Setenv("GOCPP_NOUPTODATE", "")
				pre := "a.dat"
				if tc.name == "folder" {
					pre = "dir/a.dat"
				}
				dir := inDir(t, map[string]string{
					"main.gp": "package main\n\nimport (\n\t_ \"embed\"\n\t\"fmt\"\n)\n\n//go:embed " + tc.pattern + "\nvar s []byte\n\nfunc main() { fmt.Println(len(s)) }\n",
					pre:       "one",
				})
				build := func() result {
					r := gocppRun("", false, "build", "-x", "-o", "o", ".")
					if strings.Contains(r.stderr, "WORK=") {
						workDir(t, r.stderr)
					}
					return r
				}
				if r := build(); r.code != 0 {
					t.Fatalf("first build: %+v", r)
				}
				if r := build(); r.code != 0 || !isUpToDate(r.stderr) {
					t.Fatalf("second build is not up to date:\n%s", r.stderr)
				}
				extra := filepath.Join(dir, filepath.Dir(pre), "b.dat")
				if tc.name == "folder" {
					// A folder with a new file is still a folder of two files for a []byte: an error.
					if err := os.WriteFile(extra, []byte("two"), 0o644); err != nil {
						t.Fatal(err)
					}
					r := build()
					if r.code == 0 || isUpToDate(r.stderr) || !strings.Contains(r.stderr, "multiple files") {
						t.Errorf("after adding a file: exit %d:\n%s", r.code, r.stderr)
					}
					return
				}
				if err := os.WriteFile(extra, []byte("two"), 0o644); err != nil {
					t.Fatal(err)
				}
				r := build()
				if r.code == 0 || isUpToDate(r.stderr) || !strings.Contains(r.stderr, "multiple files") {
					t.Errorf("after adding a file: exit %d:\n%s", r.code, r.stderr)
				}
				os.Remove(extra)
				if r := build(); r.code != 0 {
					t.Errorf("after removing it again: %+v", r)
				}
			})
		})
	}
}
