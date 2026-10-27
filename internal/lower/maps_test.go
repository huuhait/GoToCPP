package lower

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/cppir"
)

// TestMapAppendShape lowers one program per map key expression and reports whether the statement
// took gc's mapAppend path: the map entry is taken once, as a reference (gp::ref), instead of a
// read and a store (gp::get and gp::set). gc's walk/order.go uses mapAppend when
// ir.SameSafeExpr accepts both m[k] expressions; tests/diff/maps_append_key_expr.gp checks the
// resulting evaluation order against real Go.
func TestMapAppendShape(t *testing.T) {
	for _, tc := range []struct {
		name string
		decl string
		key  string // "m[key]" both sides, or "m[lhs]:m[rhs]"
		want bool
	}{
		{"name", "map[string][]int", "m[s]", true},
		{"string add", "map[string][]int", `m[s+"x"]`, false},
		{"string add right", "map[string][]int", `m["x"+s]`, false},
		{"conversion both sides", "map[string][]int", "m[string(s)]", true},
		{"conversion one side", "map[string][]int", "m[string(s)]:m[s]", true},
		{"runestr", "map[string][]int", "m[string(i)]", false},
		{"integer add", "map[int][]int", "m[i+1]", true},
		{"integer conversion", "map[int64][]int", "m[int64(i)]", true},
		{"integer comparison", "map[bool][]int", "m[i == i]", false},
		{"float addition", "map[float64][]int", "m[fl+1]", true},
		{"float conversion", "map[float64][]int", "m[float64(fl)]:m[fl]", false},
		{"deep conversion chain", "map[int64][]int", "m[" + deepConv("i", "int64", 200) + "]:m[int64(i)]", true},
		{"pointer to array, star on the left", "map[int][]int", "m[(*parr)[0]]:m[parr[0]]", true},
		{"pointer to array, star on the right", "map[int][]int", "m[parr[0]]:m[(*parr)[0]]", true},
		{"pointer to array, different indexes", "map[int][]int", "m[(*parr)[0]]:m[parr[1]]", false},
	} {
		lhs, rhs := tc.key, tc.key
		if l, r, ok := strings.Cut(tc.key, ":"); ok {
			lhs, rhs = l, r
		}
		src := `package main

var s = "a"
var i = 1
var fl = 1.0
var arr = [2]int{1, 2}
var parr = &arr

func f() int { return 7 }

func main() {
	m := ` + tc.decl + `{}
	` + lhs + ` = append(` + rhs + `, f())
}
`
		out := lowerSrc(t, src)
		if got := strings.Contains(out, "gp::ref("); got != tc.want {
			t.Errorf("%s: mapAppend = %v, want %v (gp::ref in the statement)\n%s", tc.name, got, tc.want, out)
		}
	}
}

// deepConv returns the source expression inner wrapped in n conversions to int and then one to
// out, a chain that gc's SameSafeExpr looks through at any depth (a "for l.Op() == OCONVNOP"
// loop). A depth cap in stripConvNop would stop early and make the row unsafe.
func deepConv(inner, out string, n int) string {
	for range n {
		inner = "int(" + inner + ")"
	}
	return out + "(" + inner + ")"
}

// TestMapAppendInterfaceConv covers conversions with an interface operand: gc's convertOp stops
// at assignOp for them ("the rules for interfaces are no different in conversions than
// assignments"), so a non-empty interface converted to another one with the same methods is an
// OCONVIFACE (it recomputes the itab) rather than an OCONVNOP that SameSafeExpr strips. Only an
// identical type, and an empty interface to another interface, stay OCONVNOPs. One side of each
// row is the plain name, so the row fails if the conversion is stripped when gc does not strip it.
func TestMapAppendInterfaceConv(t *testing.T) {
	for _, tc := range []struct {
		name string
		decl string
		key  string // "m[key]" both sides, or "m[lhs]:m[rhs]"
		want bool
	}{
		{"non-empty to non-empty with the same methods", "map[A][]int", "m[A(xv)]:m[xv]", false},
		{"non-empty to itself", "map[I][]int", "m[I(xv)]:m[xv]", true},
		{"non-empty to empty", "map[E][]int", "m[E(xv)]:m[xv]", false},
		{"empty to empty", "map[E][]int", "m[E(ev)]:m[ev]", true},
		{"to any", "map[any][]int", "m[any(ev)]:m[ev]", true},
	} {
		lhs, rhs := tc.key, tc.key
		if l, r, ok := strings.Cut(tc.key, ":"); ok {
			lhs, rhs = l, r
		}
		src := `package main

type I interface{ V() int }
type A interface{ V() int }
type E interface{}

var xv I
var ev E

func f() int { return 7 }

func main() {
	m := ` + tc.decl + `{}
	` + lhs + ` = append(` + rhs + `, f())
}
`
		out := lowerSrc(t, src)
		if got := strings.Contains(out, "gp::ref("); got != tc.want {
			t.Errorf("%s: mapAppend = %v, want %v (gp::ref in the statement)\n%s", tc.name, got, tc.want, out)
		}
	}
}

func lowerSrc(t *testing.T, src string) string {
	t.Helper()
	gp := filepath.Join(t.TempDir(), "main.gp")
	if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	fset, pkg := checkFile(t, gp)
	f, err := Package(fset, pkg, nil)
	if err != nil {
		t.Fatal(err)
	}
	return string(cppir.Print(f, ""))
}
