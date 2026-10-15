package cppgen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestTraceInitVsGo compares the stack trace of a panic during the package's initialization
// with real Go's (step 3.16): a panic in a package variable's initializer is in main.init (also
// through a function it calls, and for a variable of another file), one in an init function is in
// main.init.N, numbered in file order and then in source order across the package's files (the
// deferred calls of that init function run first), and a func literal is main.init.funcN in a
// variable's initializer and main.init.N.funcM in an init function. Go builds without inlining
// (-gcflags=-l), as in TestTraceClosuresVsGo.
func TestTraceInitVsGo(t *testing.T) {
	goOnce.Do(func() { goCmd, _ = exec.LookPath("go") })
	if goCmd == "" {
		t.Skip("no go command")
	}
	progs := map[string]map[string]string{
		"var": {"main.gp": "package main\n\nvar s = []int{1, 2}\nvar i = 5\nvar x = get(i)\n\n" +
			"func get(n int) int {\n\treturn s[n]\n}\n\nfunc main() {\n\tprintln(x)\n}\n"},
		"func": {"main.gp": "package main\n\nvar n = 1\n\nfunc init() {\n\tprintln(\"first\")\n}\n\n" +
			"func init() {\n\tdefer println(\"deferred\")\n\tif n == 1 {\n\t\tpanic(\"boom in init\")\n\t}\n}\n\nfunc main() {}\n"},
		"closures": {"main.gp": "package main\n\nvar f = func(n int) int {\n\treturn 10 / n\n}\n\nvar g = func() int { return f(1) }()\n\n" +
			"func init() {\n\th := func() {\n\t\tprintln(f(g - 10))\n\t}\n\th()\n}\n\nfunc main() {}\n"},
		"files-init": {
			"a.gp": "package main\n\nfunc init() {\n\tprintln(\"a.gp\")\n}\n\nfunc main() {}\n",
			"b.gp": "package main\n\nvar m map[string]int\n\nfunc init() {\n\tprintln(\"b.gp 1\")\n}\n\n" +
				"func init() {\n\tm[\"x\"] = 1\n}\n",
		},
		"files-var": {
			"a.gp": "package main\n\nvar first = second + 1\n\nfunc main() {\n\tprintln(first)\n}\n",
			"b.gp": "package main\n\nvar p *int\n\nvar second = *p\n",
		},
	}
	for name, files := range progs {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			for f, src := range files {
				if err := os.WriteFile(filepath.Join(dir, f), []byte(src), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want := panicFrame.ReplaceAllString(goTraceDir(t, dir, "-gcflags=-l"), "panic()\n")
			work := t.TempDir()
			cpp := filepath.Join(work, "main.gp.cpp")
			if err := os.WriteFile(cpp, []byte(emitter(dir)(t, Config{OutputPath: cpp})), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, c := range compilers(t) {
				t.Run(c.name, func(t *testing.T) {
					if c.missing != "" {
						t.Skip(c.missing)
					}
					stdout, stderr, code, err := command(work, false, c.build(t, cpp))
					if err != nil {
						t.Fatal(err)
					}
					got := panicFrame.ReplaceAllString(normTrace(maskPC(result{stderr: string(stderr)}).stderr), "panic()\n")
					if code != 2 || got != want || len(stdout) != 0 {
						t.Errorf("exit code %d, stdout %q, the trace (normalized):\n%s\nwant Go's:\n%s", code, stdout, got, want)
					}
				})
			}
		})
	}
}

// goTraceDir builds the .gp files in dir as one Go package with real Go (with more flags for go
// build), runs it, checks that it panics and returns its stderr, normalized (normTrace).
func goTraceDir(t *testing.T, dir string, flags ...string) string {
	t.Helper()
	gps, err := filepath.Glob(filepath.Join(dir, "*.gp"))
	if err != nil || len(gps) == 0 {
		t.Fatalf("no .gp files in %s: %v", dir, err)
	}
	godir := t.TempDir()
	bin := filepath.Join(godir, "prog")
	args := append(append([]string{"build"}, flags...), "-o", bin)
	for _, gp := range gps {
		src, err := os.ReadFile(gp)
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimSuffix(filepath.Base(gp), ".gp") + ".go"
		if err := os.WriteFile(filepath.Join(godir, name), src, 0o644); err != nil {
			t.Fatal(err)
		}
		args = append(args, name)
	}
	out, _, code, err := command(godir, true, goCmd, args...)
	if err != nil || code != 0 {
		t.Fatalf("go build: exit code %d %v\n%s", code, err, out)
	}
	_, stderr, code, err := command(godir, false, bin)
	if err != nil || code != 2 {
		t.Fatalf("the Go program: exit code %d %v\n%s", code, err, stderr)
	}
	return normTrace(maskPC(result{stderr: string(stderr)}).stderr)
}
