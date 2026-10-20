package load

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// parseSources makes n sources of different sizes; some have syntax errors
// and one is empty.
func parseSources(n int) (names []string, srcs [][]byte) {
	for i := range n {
		var b strings.Builder
		fmt.Fprintf(&b, "package p\n\nimport \"fmt\"\n")
		for j := range 5 + i*7%23 {
			fmt.Fprintf(&b, "\nfunc F%d_%d(a int) int {\n\tx := a * %d\n\tfmt.Println(x)\n\treturn x\n}\n", i, j, j)
		}
		switch {
		case i%5 == 3:
			b.WriteString("\nfunc broken( {\n")
		case i%7 == 6:
			b.Reset()
		}
		names = append(names, fmt.Sprintf("/x/f%02d.gp", i))
		srcs = append(srcs, []byte(b.String()))
	}
	return names, srcs
}

// The parallel parse gives the files the bases that the sequence gives them,
// the same trees and the same errors.
func TestParseFilesParallel(t *testing.T) {
	for _, n := range []int{1, 2, 3, 10, 40} {
		for _, pre := range []int{0, 1, 500} {
			names, srcs := parseSources(n)
			seq := token.NewFileSet()
			par := token.NewFileSet()
			for _, fs := range []*token.FileSet{seq, par} {
				if pre > 0 {
					fs.AddFile("pre", -1, pre)
				}
			}
			var wantFiles []*ast.File
			var wantErrs []error
			for i := range srcs {
				f, err := parser.ParseFile(seq, names[i], srcs[i], parser.ParseComments)
				wantFiles, wantErrs = append(wantFiles, f), append(wantErrs, err)
			}
			gotFiles, gotErrs := parseFilesWith(par, names, srcs, parser.ParseComments, 8, 0)
			if seq.Base() != par.Base() {
				t.Errorf("n=%d pre=%d: base %d, want %d", n, pre, par.Base(), seq.Base())
			}
			for i := range srcs {
				w, g := wantFiles[i], gotFiles[i]
				if w.FileStart != g.FileStart || w.FileEnd != g.FileEnd {
					t.Errorf("n=%d pre=%d file %d: range %d-%d, want %d-%d", n, pre, i, g.FileStart, g.FileEnd, w.FileStart, w.FileEnd)
				}
				if fmt.Sprint(wantErrs[i]) != fmt.Sprint(gotErrs[i]) {
					t.Errorf("n=%d file %d: error %v, want %v", n, i, gotErrs[i], wantErrs[i])
				}
				if len(w.Decls) != len(g.Decls) || len(w.Comments) != len(g.Comments) {
					t.Errorf("n=%d file %d: %d decls, want %d", n, i, len(g.Decls), len(w.Decls))
				}
				for d := range w.Decls {
					if d < len(g.Decls) && (w.Decls[d].Pos() != g.Decls[d].Pos() || w.Decls[d].End() != g.Decls[d].End()) {
						t.Errorf("n=%d file %d decl %d: positions differ", n, i, d)
					}
				}
				// The position of every declaration reads the same in both sets.
				for d := range w.Decls {
					if d < len(g.Decls) && seq.Position(w.Decls[d].Pos()) != par.Position(g.Decls[d].Pos()) {
						t.Errorf("n=%d file %d decl %d: %v, want %v", n, i, d, par.Position(g.Decls[d].Pos()), seq.Position(w.Decls[d].Pos()))
					}
				}
				if seq.File(w.FileStart) == nil || par.File(g.FileStart) == nil || seq.File(w.FileStart).Name() != par.File(g.FileStart).Name() {
					t.Errorf("n=%d file %d: the file is not in the set", n, i)
				}
			}
			if !reflect.DeepEqual(wantErrs == nil, gotErrs == nil) {
				t.Error("errors")
			}
		}
	}
}
