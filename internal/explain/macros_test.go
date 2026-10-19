package explain

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestMacroMessagesCovered: every error that the macro rewrite (internal/load/macros.go) and the messages about
// macros (internal/check/macromsg.go) can report has an entry that explains it (step 10.9). The
// templates are the string literals of the calls that report them.
func TestMacroMessagesCovered(t *testing.T) {
	root := repoRoot(t)
	verbs := regexp.MustCompile(`%[-+# 0-9.]*[sdqv]`)
	n := 0
	for _, file := range []string{"internal/load/macros.go", "internal/check/macromsg.go", "internal/load/derive.go", "internal/check/derive.go"} {
		path := filepath.Join(root, filepath.FromSlash(file))
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		check := func(lit *ast.BasicLit) {
			tmpl, err := strconv.Unquote(lit.Value)
			if err != nil || strings.HasPrefix(tmpl, "internal error") {
				return
			}
			n++
			// The first line is the message: the lines after it are continuations.
			tmpl, _, _ = strings.Cut(tmpl, "\n")
			sample := verbs.ReplaceAllString(tmpl, "X")
			if !covered(tmpl, sample) {
				t.Errorf("%s: the message %q has no entry in internal/explain/messages.go", fset.Position(lit.Pos()), tmpl)
			}
		}
		ast.Inspect(f, func(node ast.Node) bool {
			switch x := node.(type) {
			case *ast.CallExpr:
				// b.errorf(pos, "template", args...)
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "errorf" && len(x.Args) >= 2 {
					if lit, ok := x.Args[1].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						check(lit)
					}
				}
			case *ast.KeyValueExpr:
				// Msg: fmt.Sprintf("template", ...) in a diag.Diagnostic, or Msg: msg
				if id, ok := x.Key.(*ast.Ident); ok && id.Name == "Msg" {
					if c, ok := x.Value.(*ast.CallExpr); ok && len(c.Args) >= 1 {
						if lit, ok := c.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							check(lit)
						}
					}
				}
			case *ast.AssignStmt:
				// msg := fmt.Sprintf("template", ...)
				if len(x.Lhs) == 1 && len(x.Rhs) == 1 {
					if id, ok := x.Lhs[0].(*ast.Ident); ok && id.Name == "msg" {
						if c, ok := x.Rhs[0].(*ast.CallExpr); ok && len(c.Args) >= 1 {
							if lit, ok := c.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
								check(lit)
							}
						}
					}
				}
			}
			return true
		})
	}
	if n < 25 {
		t.Errorf("only %d macro messages found: the scan is broken", n)
	}
}
