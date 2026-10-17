package gpast

import (
	"go/ast"
	"go/token"
	"testing"
)

func TestNames(t *testing.T) {
	for _, tc := range []struct {
		name            string
		must, macroUse  bool
		mustBase, macro string
	}{
		{"F!", true, false, "F", "F!"},
		{"ReadFile!", true, false, "ReadFile", "ReadFile!"},
		{"#Name", false, true, "#Name", "Name"},
		{"#func", false, true, "#func", "func"},
		{"F", false, false, "F", "F"},
		{"!", false, false, "!", "!"},
		{"#", false, false, "#", "#"},
		{"", false, false, "", ""},
	} {
		if got := IsMust(tc.name); got != tc.must {
			t.Errorf("IsMust(%q) = %v", tc.name, got)
		}
		if got := IsMacroUse(tc.name); got != tc.macroUse {
			t.Errorf("IsMacroUse(%q) = %v", tc.name, got)
		}
		if got := MustBase(tc.name); got != tc.mustBase {
			t.Errorf("MustBase(%q) = %q", tc.name, got)
		}
		if got := MacroBase(tc.name); got != tc.macro {
			t.Errorf("MacroBase(%q) = %q", tc.name, got)
		}
	}
	if !IsMacroDecl(&ast.FuncDecl{Name: &ast.Ident{Name: "#M"}}) || IsMacroDecl(&ast.FuncDecl{Name: &ast.Ident{Name: "f!"}}) {
		t.Error("IsMacroDecl")
	}
}

func TestLazy(t *testing.T) {
	typ := &ast.Ident{Name: "string"}
	l := NewLazy(5, typ)
	if got, ok := Lazy(l); !ok || got != typ {
		t.Errorf("Lazy(NewLazy) = %v, %v", got, ok)
	}
	if l.Pos() != 5 || l.End() != typ.End() {
		t.Errorf("the lazy type spans %d to %d", l.Pos(), l.End())
	}
	for _, e := range []ast.Expr{typ, &ast.UnaryExpr{Op: token.NOT, X: typ}, &ast.StarExpr{X: typ}} {
		if _, ok := Lazy(e); ok {
			t.Errorf("Lazy(%T) is true", e)
		}
	}
}

func TestOnTop(t *testing.T) {
	call := &ast.CallExpr{Fun: &ast.Ident{Name: "#a"}}
	d := NewOnTop(7, call)
	if got, ok := OnTop(d); !ok || got != call || d.Pos() != 7 {
		t.Errorf("OnTop(NewOnTop) = %v, %v, at %d", got, ok, d.Pos())
	}
	if _, ok := OnTop(&ast.GenDecl{Tok: token.VAR}); ok {
		t.Error("a var declaration is a line of macros")
	}
	if _, ok := OnTop(&ast.FuncDecl{}); ok {
		t.Error("a func declaration is a line of macros")
	}
	if _, ok := OnTop(&ast.GenDecl{Tok: token.ILLEGAL}); ok {
		t.Error("a GenDecl without a spec is a line of macros")
	}
	f1, f2 := &ast.FuncDecl{}, &ast.FuncDecl{}
	b := NewOnTop(9, &ast.CallExpr{Fun: &ast.Ident{Name: "#b"}})
	decls := []ast.Decl{f1, d, b, f2, f2}
	if got := OnTopOf(decls, 3); len(got) != 2 || got[0] != call {
		t.Errorf("OnTopOf(f2) = %v, want the two calls", got)
	}
	for _, i := range []int{0, 1, 2, 4} {
		if got := OnTopOf(decls, i); got != nil {
			t.Errorf("OnTopOf(%d) = %v, want none", i, got)
		}
	}
}
