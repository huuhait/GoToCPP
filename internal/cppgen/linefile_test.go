package cppgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/cppir"
)

// TestLineFileNames checks with each compiler that a #line file name that the printer wrote
// (cppir.CString) reads back as the same bytes: a Windows path, spaces, quotes and UTF-8.
func TestLineFileNames(t *testing.T) {
	names := []string{
		`C:\work\hello.gp`,
		`C:\Users\Me\My Project\x"y.gp`,
		"/home/người dùng/app/main.gp",
		"/tmp/what??=/a??/b.gp",
	}
	dir := t.TempDir()
	for _, c := range compilers(t) {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			for i, name := range names {
				// int main() { std::fputs(__FILE__, stdout); return __LINE__; }, all at line 42.
				pos := cppir.Pos{File: name, Line: 42}
				fputs := &cppir.Call{Fun: &cppir.Name{Name: "std::fputs"}, Args: []cppir.Expr{&cppir.Name{Name: "__FILE__"}, &cppir.Name{Name: "stdout"}}}
				f := &cppir.File{Decls: []cppir.Decl{
					&cppir.Include{Path: "cstdio", System: true},
					&cppir.Func{Pos: pos, Result: &cppir.TypeName{Name: "int"}, Name: "main", Body: &cppir.Block{
						Stmts: []cppir.Stmt{
							&cppir.ExprStmt{Pos: pos, X: fputs},
							&cppir.Return{Pos: pos, X: &cppir.Name{Name: "__LINE__"}},
						},
						End: pos,
					}},
				}}
				src := filepath.Join(dir, fmt.Sprintf("%s-%d.cpp", c.name, i))
				if err := os.WriteFile(src, cppir.Print(f, ""), 0o644); err != nil {
					t.Fatal(err)
				}
				bin := strings.TrimSuffix(src, ".cpp")
				out, _, code, err := command(dir, true, c.cc.Path, append(c.flags(), src, "-o", bin)...)
				if err != nil || code != 0 || len(out) != 0 {
					t.Fatalf("compile %s: exit code %d %v\n%s", src, code, err, out)
				}
				r := run(t, dir, bin)
				if r.stdout != name || r.code != 42 {
					t.Errorf("__FILE__ is %q and __LINE__ %d, want %q and 42", r.stdout, r.code, name)
				}
			}
		})
	}
}
