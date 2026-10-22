package icheck

import (
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The refusals of the argument adaptations in the checker (adapt.go). The declarations are made up
// (no compiler runs): the checker reads the overloads of the callee from the declaration table,
// as the lowering does.

// declare gives the report a function of the header package with the overloads, each a C++ function
// type, and says that the name is a function.
func (f *fab) declare(name string, kind string, types ...string) {
	f.t.Helper()
	decls := f.rep.Decls
	if decls == nil {
		decls = &probe.DeclTable{Funcs: map[string]*probe.FuncSet{}}
		f.rep.Decls = decls
	}
	set := &probe.FuncSet{Name: name}
	for _, text := range types {
		ty, err := ctype.Parse(text)
		if err != nil {
			f.t.Fatalf("%s: %v", text, err)
		}
		set.Overloads = append(set.Overloads, probe.Overload{Kind: kind, Sig: probe.TypeText{Text: text, T: ty}})
	}
	decls.Funcs[name] = set
	f.rep.Mapper = ftype.New(decls, ftype.LP64)
	if f.rep.Kinds == nil {
		f.rep.Kinds = map[string]interop.Kind{}
	}
	for _, r := range f.rep.Set.Refs {
		collect.Walk(r.Expr, func(e collect.Expr) bool {
			if p, ok := e.(*collect.Path); ok && len(p.Parts) == 1 && p.Parts[0] == name {
				f.rep.Kinds[p.KindPath()] = interop.Value
			}
			return true
		})
	}
}

const adaptSrc = `package main

import "<cstdlib>"

func main() {
	cstdlib.abs(200)
}
`

func adaptErrors(r *Result) []string {
	var out []string
	for _, d := range r.Diags {
		if d.Severity == diag.Error {
			out = append(out, strings.TrimSpace(d.Msg))
		}
	}
	return out
}

// A constant that does not fit the parameter is an error, and with g++ too: it is the language's
// error, not the probe's guess.
func TestAdaptationRefusalIsAnErrorWithEitherCompiler(t *testing.T) {
	for _, fam := range []build.Family{build.Clang, build.GCC} {
		f := newFab(t, adaptSrc)
		f.declare("abs", "function", "int (char)")
		r := f.check(fam)
		got := adaptErrors(r)
		if len(got) != 1 || !strings.Contains(got[0], "cannot use 200 (untyped int constant) as C.char value in argument to") {
			t.Fatalf("family %v: want one overflow error, got %v (diags %v)", fam, got, lines(r))
		}
		if pos := r.Diags[0].Pos; pos.Line != 6 || pos.Column != 14 {
			t.Errorf("family %v: the error is at %d:%d, want the argument at 6:14", fam, pos.Line, pos.Column)
		}
		if r.Stats.Adapt != 1 {
			t.Errorf("family %v: Stats.Adapt = %d, want 1", fam, r.Stats.Adapt)
		}
	}
}

// A constant that fits, an overload set that disagrees, a template, and a name without declaration
// are left alone.
func TestAdaptationSilentWhenNotCertain(t *testing.T) {
	cases := []struct {
		name, kind string
		types      []string
	}{
		{"fits", "function", []string{"int (short)"}},
		{"the overloads disagree", "function", []string{"int (char)", "int (long)"}},
		{"a template", "template", []string{"int (char)"}},
	}
	for _, c := range cases {
		f := newFab(t, adaptSrc)
		f.declare("abs", c.kind, c.types...)
		if got := adaptErrors(f.check(build.Clang)); len(got) != 0 {
			t.Errorf("%s: %v", c.name, got)
		}
	}
	// The kind probe says a function template, whatever the declarations list: the overloads may be
	// only the non-template ones.
	f := newFab(t, adaptSrc)
	f.declare("abs", "function", "int (char)")
	for key := range f.rep.Kinds {
		f.rep.Kinds[key] = interop.ValueTemplate
	}
	if got := adaptErrors(f.check(build.Clang)); len(got) != 0 {
		t.Errorf("a function template: %v", got)
	}
	f = newFab(t, adaptSrc)
	f.rep.Decls = &probe.DeclTable{Funcs: map[string]*probe.FuncSet{}}
	f.rep.Mapper = ftype.New(f.rep.Decls, ftype.LP64)
	if got := adaptErrors(f.check(build.Clang)); len(got) != 0 {
		t.Errorf("no declaration: %v", got)
	}
}

// The lowering is not typed in a package that declares a generic function or type, so it refuses
// nothing there: neither does the checker.
func TestAdaptationSkipsPackagesWithGenerics(t *testing.T) {
	for _, extra := range []string{
		"\nfunc id[T any](x T) T { return x }\n",
		"\ntype Box[T any] struct{ v T }\n",
	} {
		f := newFab(t, adaptSrc+extra)
		f.declare("abs", "function", "int (char)")
		if got := adaptErrors(f.check(build.Clang)); len(got) != 0 {
			t.Errorf("%q: %v", extra, got)
		}
	}
}

// BenchmarkAdaptations is the pass over a package of 4000 calls into C++ with a constant argument
// (declarations made up): its cost is a walk over the calls and one decision for each.
func BenchmarkAdaptations(b *testing.B) {
	var src strings.Builder
	src.WriteString("package main\n\nimport \"<cstdlib>\"\n\nfunc main() {\n")
	for i := 0; i < 4000; i++ {
		src.WriteString("\tcstdlib.abs(100)\n")
	}
	src.WriteString("}\n")
	f := newFab(b, src.String())
	f.declare("abs", "function", "int (char)")
	f.rep.Values = make([]ftype.Value, len(f.rep.Table.Results))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := &checker{p: f.p, fset: f.fset, rep: f.rep, res: &Result{}}
		c.adaptations()
	}
}

// The address of a wide character pointer next to a string is refused with the types the program
// wrote for the variable, when the probe has no typed view of &end: the declared type is enough.
func TestAdaptationUsesTheDeclaredTypeOfAPointerVariable(t *testing.T) {
	f := newFab(t, `package main

import "<cwchar>"

func main() {
	var end *C.wchar_t
	text := "12"
	cwchar.wcstol(text, &end, 10)
}
`)
	f.declare("wcstol", "function", "long (const wchar_t *, wchar_t **, int)")
	got := adaptErrors(f.check(build.Clang))
	if len(got) != 1 || !strings.Contains(got[0], "wcstol is given the address of a wchar_t pointer (&end) next to the string text") {
		t.Fatalf("errors = %v", got)
	}
	// A variable that gets its type from its value, or is a parameter, has no written type here: no error.
	f = newFab(t, `package main

import "<cwchar>"

func take(end **C.wchar_t) {
	text := "12"
	cwchar.wcstol(text, end, 10)
}
`)
	f.declare("wcstol", "function", "long (const wchar_t *, wchar_t **, int)")
	if got := adaptErrors(f.check(build.Clang)); len(got) != 0 {
		t.Errorf("a parameter: %v", got)
	}
}
