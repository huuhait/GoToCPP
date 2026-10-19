package collect

import (
	"go/ast"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// TestPromotedRefs checks the references of the members that a struct takes from an embedded class
// (step 12.9): the object is an lvalue of the field's C++ type (a temporary for the member of a call's
// result), the class is a reference of its own that the selector's name stands for, and a pointer, a
// named type and an alias are the C++ type behind them.
func TestPromotedRefs(t *testing.T) {
	const src = `package main

import (
	"<vector>"
	"<string>"
)

type Inner struct {
	k int
	*std.vector[int]
}

type Outer struct {
	n int
	Inner
}

type Alias = std.string

type Named std.string

type A struct{ Alias }
type N struct{ Named }
type V struct{ std.vector[int] }

func f() V { return V{} }

func main() {
	var o Outer
	o.push_back(1)
	_ = o.k
	var a A
	a.append("x")
	var n N
	n.clear()
	f().clear()
	var p *V
	p.pop_back()
}
`
	p, fset := tsupport.CheckSource(t, src, nil)
	if errs := p.Errors.Diagnostics(); len(errs) > 0 {
		t.Fatalf("the program does not check: %v", errs)
	}
	set := Collect(p, fset)
	has := func(key string) bool { return set.Ref(key) != nil }
	var keys []string
	for _, r := range set.Refs {
		keys = append(keys, r.Key)
	}
	for _, want := range []string{
		"call(sel(arg&(star(index(std::vector|go(int64_t))))|push_back)|lit(1))", // through Outer.Inner.*vector
		"call(sel(arg&(std::string)|append)|lit(\"x\"))",                         // an alias
		"call(sel(arg&(std::string)|clear)|)",                                    // a named type over the class
		"call(sel(arg(index(std::vector|go(int64_t)))|clear)|)",                  // the result of a call is a temporary
		"call(sel(arg&(index(std::vector|go(int64_t)))|pop_back)|)",              // through a pointer to the struct
		"star(index(std::vector|go(int64_t)))",                                   // the class of the first
	} {
		if !has(want) {
			t.Errorf("no reference %q in\n%s", want, strings.Join(keys, "\n"))
		}
	}
	// The name of each promoted selector stands for the reference of its class.
	n := 0
	for _, f := range p.Load.Files {
		ast.Inspect(f.AST, func(node ast.Node) bool {
			sel, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			promoted := p.PromotedPath(sel) != nil
			r := set.RefOf(sel.Sel)
			if promoted != (r != nil) {
				t.Errorf("%s: promoted %v, class reference %v", sel.Sel.Name, promoted, r)
			}
			if promoted {
				n++
			}
			return true
		})
	}
	if n != 5 {
		t.Errorf("%d promoted selectors, want 5", n)
	}
	// The reference of a class has no site of its own (the declaration of the field is its site).
	if r := set.Ref("std::string"); r == nil {
		t.Errorf("no reference for the class std::string")
	}
}
