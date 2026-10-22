package lower

import (
	"go/parser"
	"testing"

	"github.com/huuhait/gocpp/internal/cppir"
)

func TestPointsIntoTemporary(t *testing.T) {
	holder := &cppir.Call{Fun: &cppir.Member{X: call("gp::cstr", cxxName("s")), Name: "gp_ptr"}}
	if !pointsIntoTemporary(holder) {
		t.Errorf("gp::cstr(s).gp_ptr() lives in a temporary of the full expression")
	}
	for name, x := range map[string]cppir.Expr{
		"a call":   call("g", cxxName("a")),
		"a name":   cxxName("a"),
		"a member": &cppir.Call{Fun: &cppir.Member{X: cxxName("a"), Name: "size"}},
	} {
		if pointsIntoTemporary(x) {
			t.Errorf("%s does not point into a temporary", name)
		}
	}
}

func TestContainsCall(t *testing.T) {
	for src, want := range map[string]bool{
		"f(x)":      true,
		"a + g()":   true,
		"x":         false,
		"a.b + 1":   false,
		"-n":        false,
		"[]int{1}":  false,
		"h.one(&a)": true,
	} {
		e, err := parser.ParseExpr(src)
		if err != nil {
			t.Fatal(err)
		}
		if got := containsCall(e); got != want {
			t.Errorf("containsCall(%s) = %v, want %v", src, got, want)
		}
	}
}
