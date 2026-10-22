package collect

import (
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// Code that does not type (an undefined type name, a package that is not imported) reaches the
// collector when the HRESULT hook of check asks for the calls before the type errors are known. The
// collector has to end on it, with Unknown pieces: conv, goValue and typeIR used to send one another
// round in a circle for a qualified type name that go/types had left invalid (a stack overflow).
var illTypedPrograms = map[string]string{
	"undefined type in a literal":          "p := []fmt.Nosuch{cstdlib.abs(-3)}\n\t_ = p",
	"package not imported":                 "k := cstdlib.abs(-3)\n\t_ = lib.Pair{k, 2}",
	"package not imported in a map":        "k := cstdlib.abs(-3)\n\t_ = map[any]lib.Pair{k: {}}",
	"undefined type in a map key":          "k := cstdlib.abs(-3)\n\t_ = map[fmt.Nosuch]int{k: 1}",
	"undefined type as a map element":      "k := cstdlib.abs(-3)\n\t_ = map[any]fmt.Nosuch{k: {1}}",
	"undefined type in a nested literal":   "k := cstdlib.abs(-3)\n\t_ = [][]fmt.Nosuch{{k}, {k, k}}",
	"undefined type in a conversion":       "k := cstdlib.abs(-3)\n\t_ = fmt.Nosuch(k)",
	"undefined type in a pointer literal":  "k := cstdlib.abs(-3)\n\t_ = &lib.Pair{A: k}",
	"undefined type in an array literal":   "k := cstdlib.abs(-3)\n\t_ = [2]lib.Pair{{k}, {k}}",
	"undefined generic instance":           "k := cstdlib.abs(-3)\n\t_ = lib.List[int]{k}",
	"undefined type in a func literal":     "k := cstdlib.abs(-3)\n\tf := func(a lib.Pair) lib.Pair { return a }\n\t_, _ = f, k",
	"undefined type in a variable":         "k := cstdlib.abs(-3)\n\tvar v lib.Pair = lib.Pair{k, 1}\n\t_ = v",
	"undefined type in a struct literal":   "k := cstdlib.abs(-3)\n\tv := struct{ A lib.Pair }{lib.Pair{k, 1}}\n\t_ = v",
	"undefined type in a conversion call":  "k := cstdlib.abs(-3)\n\t_ = []lib.Pair(k)",
	"undefined name next to a C value":     "k := cstdlib.abs(-3)\n\t_ = []int{k, nosuch}",
	"undefined package function with C":    "k := cstdlib.abs(-3)\n\tlib.F(k, lib.Pair{})",
	"undefined type of a channel element":  "k := cstdlib.abs(-3)\n\tch := make(chan lib.Pair, 1)\n\tch <- lib.Pair{k, 2}",
	"undefined type of a func signature":   "k := cstdlib.abs(-3)\n\tvar f func(lib.Pair) lib.Pair\n\t_ = f(lib.Pair{k, 2})",
	"undefined type in a type assertion":   "k := cstdlib.abs(-3)\n\tvar a any = k\n\t_ = a.(lib.Pair)",
	"undefined type in a type switch":      "k := cstdlib.abs(-3)\n\tvar a any = k\n\tswitch a.(type) {\n\tcase lib.Pair:\n\t}",
	"undefined type in a make":             "k := cstdlib.abs(-3)\n\t_ = make([]lib.Pair, k)",
	"undefined type in a new":              "k := cstdlib.abs(-3)\n\t_ = new(lib.Pair)\n\t_ = k",
	"undefined type in a pointer receiver": "k := cstdlib.abs(-3)\n\tvar p *lib.Pair\n\t_ = p.A + k",
}

// Declarations next to main that do not type either.
const illTypedDecls = `
type Holder struct {
	A lib.Pair
	B []fmt.Nosuch
}

func sig(a lib.Pair, rest ...fmt.Nosuch) (n lib.Pair, err error) { return }

func generic[T any](v T) map[any]lib.Pair {
	k := cstdlib.abs(-3)
	return map[any]lib.Pair{k: {}, k: lib.Pair{1, 2}}
}

func generic2[T fmt.Nosuch](v T) []T {
	k := cstdlib.abs(-3)
	return []T{v, k}
}
`

func TestCollectorEndsOnIllTypedCode(t *testing.T) {
	for name, body := range illTypedPrograms {
		t.Run(name, func(t *testing.T) {
			collectIllTyped(t, "package main\n\nimport (\n"+stdImports+")\n"+illTypedDecls+"\nfunc main() {\n"+body+"\n\tfmt.Println()\n}\n")
		})
	}
}

// collectIllTyped checks src (it has errors) and collects it, with a bound on the time.
func collectIllTyped(t *testing.T, src string) {
	t.Helper()
	p, fset := tsupport.CheckSource(t, src, nil)
	if len(p.Errors.Diagnostics()) == 0 {
		t.Fatalf("the program has no error:\n%s", src)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = Collect(p, fset)
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatalf("the collector does not end on:\n%s", src)
	}
}

// The valid programs of the shapes with one thing broken (an import taken out, a type misspelled)
// are ill-typed variants: the collector ends on each of them.
func TestCollectorEndsOnBrokenVariantsOfTheShapes(t *testing.T) {
	breaks := []struct{ name, from, to string }{
		{"no fmt", "\t\"fmt\"\n", ""},
		{"misspelled cstring", "cstring.", "cstrin."},
		{"misspelled std", "std.", "sdt."},
	}
	n := 0
	for _, s := range shapeCases {
		src := "package main\n\nimport (\n" + stdImports + ")\n\nfunc main() {\n" + s.body + "\n\tfmt.Println()\n}\n"
		for _, b := range breaks {
			if !strings.Contains(src, b.from) {
				continue
			}
			broken := strings.Replace(src, b.from, b.to, 1)
			n++
			t.Run(s.name+"/"+b.name, func(t *testing.T) {
				p, fset := tsupport.CheckSource(t, broken, nil)
				_ = p
				done := make(chan struct{})
				go func() {
					defer close(done)
					_ = Collect(p, fset)
				}()
				select {
				case <-done:
				case <-time.After(60 * time.Second):
					t.Fatalf("the collector does not end on:\n%s", broken)
				}
			})
		}
	}
	if n == 0 {
		t.Fatal("no variant was made")
	}
}
