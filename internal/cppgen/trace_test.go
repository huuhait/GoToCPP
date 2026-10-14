package cppgen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestTraceVsGo compares the stack trace of an unrecovered panic with real Go's (Go 1.25's
// form), for a recursion of several depths: the same frames, function names, files and
// lines, and for a stack of more than 100 frames the inner 50, "...N frames elided..." with
// Go's count, and the outer 50 ending with main.main. normTrace leaves out what differs on
// purpose: the folders, the offsets, and what Go prints between the parentheses (argument
// words, or "..." for an inlined frame).
func TestTraceVsGo(t *testing.T) {
	goOnce.Do(func() { goCmd, _ = exec.LookPath("go") })
	if goCmd == "" {
		t.Skip("no go command")
	}
	for _, depth := range []int{3, 98, 99, 200} {
		t.Run(fmt.Sprint(depth), func(t *testing.T) {
			dir := t.TempDir()
			src := fmt.Sprintf("package main\n\nfunc r(n int) {\n\tif n == 0 {\n\t\tpanic(\"deep\")\n\t}\n\tr(n - 1)\n}\n\nfunc main() {\n\tr(%d)\n}\n", depth)
			gp := filepath.Join(dir, "main.gp")
			if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			want := goTrace(t, gp)
			work := t.TempDir()
			cpp := filepath.Join(work, "main.gp.cpp")
			if err := os.WriteFile(cpp, []byte(emitter(gp)(t, Config{OutputPath: cpp})), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, c := range compilers(t) {
				t.Run(c.name, func(t *testing.T) {
					if c.missing != "" {
						t.Skip(c.missing)
					}
					_, stderr, code, err := command(work, false, c.build(t, cpp))
					if err != nil {
						t.Fatal(err)
					}
					if got := normTrace(string(stderr)); code != 2 || got != want {
						t.Errorf("exit code %d, the trace (normalized):\n%s\nwant Go's:\n%s", code, got, want)
					}
				})
			}
		})
	}
}

// goTrace builds the .gp file gp as a Go program with real Go, runs it, checks that it panics
// and returns its stderr, normalized.
func goTrace(t *testing.T, gp string) string {
	t.Helper()
	dir := t.TempDir()
	src, err := os.ReadFile(gp)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "prog")
	out, _, code, err := command(dir, true, goCmd, "build", "-o", bin, "main.go")
	if err != nil || code != 0 {
		t.Fatalf("go build: exit code %d %v\n%s", code, err, out)
	}
	_, stderr, code, err := command(dir, false, bin)
	if err != nil || code != 2 {
		t.Fatalf("the Go program: exit code %d %v\n%s", code, err, stderr)
	}
	return normTrace(string(stderr))
}

// normTrace rewrites the frames of a trace (after the goroutine line) so that Go's and
// gocpp's compare: "name(anything)" becomes "name()", and "\t/dir/main.go:9 +0x25" becomes
// "\tmain:9". The header and the "...N frames elided..." line stay as they are.
func normTrace(stderr string) string {
	i := strings.Index(stderr, goStackMark)
	if i < 0 {
		return stderr
	}
	var b strings.Builder
	b.WriteString(stderr[:i+len(goStackMark)])
	for _, line := range strings.Split(strings.TrimSuffix(stderr[i+len(goStackMark):], "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "\t"):
			loc := filepath.Base(strings.Fields(line)[0])
			file, num, _ := strings.Cut(loc, ":")
			file = strings.TrimSuffix(strings.TrimSuffix(file, ".gp"), ".go")
			fmt.Fprintf(&b, "\t%s:%s\n", file, num)
		case strings.HasPrefix(line, "..."):
			b.WriteString(line + "\n")
		default:
			if j := strings.Index(line, "("); j >= 0 {
				line = line[:j]
			}
			b.WriteString(line + "()\n")
		}
	}
	return b.String()
}
