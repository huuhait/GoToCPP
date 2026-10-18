package lower

import (
	"errors"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/cppir"
)

// lowerGeneric lowers the main package src and returns its C++ (the unit layout), or the error.
func lowerGeneric(t *testing.T, src string) (string, error) {
	t.Helper()
	gp := filepath.Join(t.TempDir(), "main.gp")
	if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	fset, pkg := checkFile(t, gp)
	f, err := Package(fset, pkg, nil)
	if err != nil {
		return "", err
	}
	return string(cppir.Print(f, "")), nil
}

// instances returns the C++ names of the definitions of instances in the unit text out, which are
// the lines of the form "<result> gp_inst_<hash>_<name>(...) {".
func instances(out string) []string {
	var names []string
	rx := regexp.MustCompile(`(?m)^[^\n#/]*\b(gp_inst_[0-9a-f]{8}_\w+)\([^\n]*\) \{$`)
	for _, m := range rx.FindAllStringSubmatch(out, -1) {
		names = append(names, m[1])
	}
	return names
}

const genericSrc = `package main

import "fmt"

func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

func Fib[T ~int | ~int8](n T) T {
	if n < 2 {
		return n
	}
	return Fib(n-1) + Fib(n-2)
}

func Twice[T any](x T) []T { return Map([]T{x, x}, func(v T) T { return v }) }

func main() {
	a := Map([]int{1}, func(i int) string { return fmt.Sprint(i) })
	b := Map([]int{2}, func(i int) string { return fmt.Sprint(i) })
	c := Map([]string{"x"}, func(s string) int { return len(s) })
	f := Map[int, string]
	fmt.Println(a, b, c, f != nil, Fib(10), Fib(int8(10)), Twice(1), Twice("s"), Twice(2))
}
`

// TestInstances checks that each list of type arguments gets one instance per unit: Map[int,string]
// four times (two calls, a value, ...) is one function, recursion adds none, and an instance that
// another instance asks for is found too.
func TestInstances(t *testing.T) {
	out, err := lowerGeneric(t, genericSrc)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, n := range instances(out) {
		kinds = append(kinds, n[len("gp_inst_xxxxxxxx_"):])
	}
	// Map[int,string], Map[string,int], Map[int,int] and Map[string,string] (from Twice), Fib[int],
	// Fib[int8], Twice[int], Twice[string]: eight instances, in the order that they were found.
	counts := map[string]int{}
	for _, k := range kinds {
		counts[k]++
	}
	if counts["Map"] != 4 || counts["Fib"] != 2 || counts["Twice"] != 2 || len(kinds) != 8 {
		t.Errorf("instances %v, want 4 Map, 2 Fib, 2 Twice", kinds)
	}
	// Each name is defined once (not twice by two requests of the same instance).
	seen := map[string]bool{}
	for _, n := range instances(out) {
		if seen[n] {
			t.Errorf("%s is defined twice", n)
		}
		seen[n] = true
	}
	if strings.Contains(out, "template <") {
		t.Errorf("the instances of Go types are no templates:\n%s", out)
	}
}

// TestInstancesDeterministic lowers the program twice: the same text, names and order.
func TestInstancesDeterministic(t *testing.T) {
	a, err := lowerGeneric(t, genericSrc)
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		b, err := lowerGeneric(t, genericSrc)
		if err != nil {
			t.Fatal(err)
		}
		// (the temporary folder is in the #line paths)
		strip := func(s string) string {
			return regexp.MustCompile(`#line \d+ "[^"]*"`).ReplaceAllString(s, "#line")
		}
		if strip(a) != strip(b) {
			t.Fatal("two lowerings of one program differ")
		}
	}
}

// TestInstanceTypes checks that the body of an instance is lowered with the type arguments: the
// small-integer cast back for int8, the C++ types in the signature, and the descriptors of any(x).
func TestInstanceTypes(t *testing.T) {
	out, err := lowerGeneric(t, `package main

import "fmt"

type MyInt int

func Add[T ~int8 | ~int | ~uint8](a, b T) T { return a + b }

func Box[T any](v T) any { return v }

func Join[S ~[]E, E ~string](s S) E {
	var out E
	for _, v := range s {
		out += v
	}
	return out
}

func main() {
	fmt.Println(Add[int8](1, 2), Add[int](1, 2), Add(MyInt(1), 2), Box(1), Box(MyInt(2)), Join([]string{"a"}))
}
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"int8_t gp_inst_", "(int8_t a, int8_t b)", "int8_t(a + b)", // the cast back
		"int64_t gp_inst_", "MyInt gp_inst_",
		"v, &gp::td_int)", "v, &gp_td_MyInt)", // Box[int] and Box[MyInt] have their own descriptors
		"gp::string gp_inst_", "gp::slice<gp::string>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the C++ has no %q:\n%s", want, out)
		}
	}
}

// TestGenericErrors checks the programs that lowering turns down for a user error, with the message.
func TestGenericErrors(t *testing.T) {
	tests := []struct{ name, src, want string }{
		{"local type", `package main

func F[T any](v T) any {
	type box struct{ v T }
	return box{v}
}

func main() { _ = F(1) }
`, "main.gp:4:2: type declarations inside generic functions are not supported yet (Phase 07)"},
		{"generic type", `package main

type Box[T any] struct{ v T }

func main() { _ = Box[int]{} }
`, "main.gp:3:6: generic types are not supported yet (Phase 07)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := lowerGeneric(t, tc.src)
			var le *Error
			if !errors.As(err, &le) {
				t.Fatalf("error %v, want a lowering error", err)
			}
			if !le.User {
				t.Errorf("%v is not a user error", err)
			}
			got := filepath.Base(le.Pos.Filename) + ":" + itoa(le.Pos.Line) + ":" + itoa(le.Pos.Column) + ": " + le.Msg
			if got != tc.want {
				t.Errorf("error %q, want %q", got, tc.want)
			}
		})
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

// TestSubst checks the substitution of type parameters in the types go/types has.
func TestSubst(t *testing.T) {
	gp := filepath.Join(t.TempDir(), "main.gp")
	src := `package main

type Pair[A, B any] struct{ a A; b B }

type Num interface{ ~int | ~float64 }

type Celsius float64

func F[K comparable, V Num](m map[K][]V, f func(K, ...V) (V, error), p *[3]V, ch <-chan K, s struct{ X V }) {}

func main() {}
`
	if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	_, pkg := checkFile(t, gp)
	fn := pkg.Types.Scope().Lookup("F").(*types.Func)
	sig := fn.Signature()
	tparams := sig.TypeParams()
	celsius := pkg.Types.Scope().Lookup("Celsius").Type()
	s := newSubster(tparams, []types.Type{types.Typ[types.String], celsius}, types.NewContext())
	var got []string
	for v := range sig.Params().Variables() {
		got = append(got, types.TypeString(s.typ(v.Type()), types.RelativeTo(pkg.Types)))
	}
	want := []string{
		"map[string][]Celsius",
		"func(string, ...Celsius) (Celsius, error)",
		"*[3]Celsius",
		"<-chan string",
		"struct{X Celsius}",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("substituted\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// A type without type parameters is returned as it is (the same pointer).
	if s.typ(types.Typ[types.Int]) != types.Typ[types.Int] || s.typ(celsius) != celsius {
		t.Error("a type without type parameters was copied")
	}
	// A variable whose type does not change is the same variable; one that changes is new, once.
	ps := sig.Params()
	v0 := ps.At(0)
	if n := s.varFor(v0); n == v0 || n != s.varFor(v0) || n.Name() != v0.Name() || n.Pos() != v0.Pos() {
		t.Error("varFor of a changed variable")
	}
	plain := types.NewVar(0, nil, "x", types.Typ[types.Int])
	if s.varFor(plain) != plain {
		t.Error("varFor of an unchanged variable")
	}
}

// TestTinfoSelections checks that the lowering's selections are those of go/types.
func TestTinfoSelections(t *testing.T) {
	gp := filepath.Join(t.TempDir(), "main.gp")
	src := `package main

type T struct{ n int }

func (t T) V() int   { return t.n }
func (t *T) P() int  { return t.n }

func main() {
	var t T
	_ = t.n
	_ = t.V()
	_ = t.P()
	_ = T.V
}
`
	if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	_, pkg := checkFile(t, gp)
	ti := newTinfo(pkg.Info)
	if len(ti.Selections) != len(pkg.Info.Selections) || len(ti.Selections) != 6 {
		t.Fatalf("%d selections, go/types has %d", len(ti.Selections), len(pkg.Info.Selections))
	}
	for x, s := range pkg.Info.Selections {
		o := ti.Selections[x]
		if o.Kind() != s.Kind() || o.Obj() != s.Obj() || o.Recv() != s.Recv() || o.Indirect() != s.Indirect() || len(o.Index()) != len(s.Index()) {
			t.Errorf("selection of %s differs", x.Sel.Name)
		}
	}
	var _ ast.Node
}
