package collect

import (
	"go/ast"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// The calls that return an HRESULT (ADR-137): check wraps the ones whose result the program takes
// as an error in error(...) (check.Config.HResults), and KeepsHResult is what tells the ones that
// stay raw.

// keepsFirstAbs reports KeepsHResult for the first call of cmath.abs in the body of main.
func keepsFirstAbs(t *testing.T, body string) bool {
	t.Helper()
	src := "package main\n\nimport (\n" + stdImports + ")\n\nfunc main() {\n" + body + "\n\tfmt.Println()\n}\n"
	p, _ := tsupport.CheckSource(t, src, nil)
	if errs := p.Errors.Diagnostics(); len(errs) > 0 {
		t.Fatalf("the program doesn't check: %v\n%s", errs, src)
	}
	parents := ParentsOf(p)
	var call *ast.CallExpr
	for _, f := range p.Load.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok && call == nil && strings.HasSuffix(exprText(c.Fun), "abs") {
				call = c
			}
			return call == nil
		})
	}
	if call == nil {
		t.Fatalf("no call of abs in:\n%s", src)
	}
	return KeepsHResult(p, func(n ast.Node) ast.Node { return parents[n] }, call)
}

func exprText(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.SelectorExpr:
		return exprText(e.X) + "." + e.Sel.Name
	case *ast.Ident:
		return e.Name
	}
	return ""
}

func TestHResultUsedValueIsAnError(t *testing.T) {
	for name, body := range map[string]string{
		"variable":  "\terr := cmath.abs(-3)\n\tfmt.Println(err)",
		"argument":  "\tfmt.Println(cmath.abs(-3))",
		"condition": "\tif cmath.abs(-3) != 0 {\n\t}",
		"to error":  "\tvar e error = cmath.abs(-3)\n\t_ = e",
	} {
		if keepsFirstAbs(t, body) {
			t.Errorf("%s: a call whose result is used is kept raw", name)
		}
	}
}

func TestHResultDroppedIsKept(t *testing.T) {
	for name, body := range map[string]string{
		"statement":     "\tcmath.abs(-3)",
		"defer":         "\tdefer cmath.abs(-3)",
		"go":            "\tgo cmath.abs(-3)",
		"blank":         "\t_ = cmath.abs(-3)",
		"parenthesized": "\t(cmath.abs(-3))",
	} {
		if !keepsFirstAbs(t, body) {
			t.Errorf("%s: a call whose result is dropped is not kept raw", name)
		}
	}
}

// int32(f()) and C.long(f()) are how the program gets the code, with its success codes.
func TestHResultConvertedToANumberIsKept(t *testing.T) {
	for name, body := range map[string]string{
		"go number":       "\tcode := int32(cmath.abs(-3))\n\tfmt.Println(code)",
		"c number":        "\tcode := C.long(cmath.abs(-3))\n\t_ = code",
		"parenthesized":   "\t_ = int32((cmath.abs(-3)))",
		"unsigned number": "\t_ = uint32(cmath.abs(-3))",
	} {
		if !keepsFirstAbs(t, body) {
			t.Errorf("%s: a call converted to a number is not kept raw", name)
		}
	}
}

// A conversion to something that is not a number takes the error: it is not the code.
func TestHResultConvertedToANonNumberIsAnError(t *testing.T) {
	if keepsFirstAbs(t, "\t_ = error(cmath.abs(-3))") {
		t.Error("a call converted to error is kept raw")
	}
}

// error(f()) of an HRESULT call (which check wrote) makes no reference of its own: the call is the
// reference, and the conversion is not asked of C++.
func TestErrorConversionMakesNoReference(t *testing.T) {
	src := "package main\n\nimport (\n" + stdImports + ")\n\nfunc main() {\n\terr := error(cmath.abs(-3))\n\tfmt.Println(err)\n}\n"
	p, fset := tsupport.CheckSource(t, src, nil)
	if errs := p.Errors.Diagnostics(); len(errs) > 0 {
		t.Fatalf("the program doesn't check: %v\n%s", errs, src)
	}
	set := Collect(p, fset)
	got := format(set)
	if !strings.Contains(got, "call(::abs|") {
		t.Errorf("the call is not a reference:\n%s", got)
	}
	if strings.Contains(got, "call(error") || strings.Contains(got, "error(") {
		t.Errorf("the conversion to error is a reference:\n%s", got)
	}
}

func format(set *Set) string {
	var b strings.Builder
	set.Format(&b, false)
	return b.String()
}
