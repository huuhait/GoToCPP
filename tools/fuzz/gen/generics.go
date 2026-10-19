package gen

import "strings"

// The generic functions of the programs: a fixed set of templates (the bodies are what the
// compiler's lowering of generics must handle: operators on a type parameter, range over a
// slice and a map of type parameters, closures of a type parameter, the zero value, address
// of a parameter), written when a program calls one, and instantiated at random types, with
// explicit type arguments or inferred ones.

var templates = map[string]string{
	"Number":   "type Number interface {\n\t~int | ~int64 | ~float64\n}\n",
	"Sum":      "func Sum[T Number](a, b T) T {\n\treturn a + b\n}\n",
	"Max":      "func Max[T ~int | ~float64 | ~string](a, b T) T {\n\tif a > b {\n\t\treturn a\n\t}\n\treturn b\n}\n",
	"Pick":     "func Pick[T any](a, b T, c bool) T {\n\tif c {\n\t\treturn a\n\t}\n\treturn b\n}\n",
	"Ptr":      "func Ptr[T any](v T) *T {\n\treturn &v\n}\n",
	"Zero":     "func Zero[T any]() T {\n\tvar z T\n\treturn z\n}\n",
	"Apply":    "func Apply[T any](x T, f func(T) T) T {\n\treturn f(x)\n}\n",
	"Map":      "func Map[T, U any](s []T, f func(T) U) []U {\n\tr := make([]U, 0, len(s))\n\tfor _, x := range s {\n\t\tr = append(r, f(x))\n\t}\n\treturn r\n}\n",
	"Filter":   "func Filter[T any](s []T, f func(T) bool) []T {\n\tvar r []T\n\tfor _, x := range s {\n\t\tif f(x) {\n\t\t\tr = append(r, x)\n\t\t}\n\t}\n\treturn r\n}\n",
	"Keys":     "func Keys[K comparable, V any](m map[K]V) []K {\n\tr := make([]K, 0, len(m))\n\tfor k := range m {\n\t\tr = append(r, k)\n\t}\n\treturn r\n}\n",
	"Contains": "func Contains[T comparable](s []T, v T) bool {\n\tfor _, x := range s {\n\t\tif x == v {\n\t\t\treturn true\n\t\t}\n\t}\n\treturn false\n}\n",
	"Reduce":   "func Reduce[T, A any](s []T, init A, f func(A, T) A) A {\n\tacc := init\n\tfor _, x := range s {\n\t\tacc = f(acc, x)\n\t}\n\treturn acc\n}\n",
}

var templateOrder = []string{"Number", "Sum", "Max", "Pick", "Ptr", "Zero", "Apply", "Map", "Filter", "Keys", "Contains", "Reduce"}

func (g *gen) use(names ...string) {
	for _, n := range names {
		g.gfuncs[n] = true
	}
}

// writeGenerics writes the templates that the program uses.
func (g *gen) writeGenerics() {
	if g.gfuncs["Sum"] {
		g.gfuncs["Number"] = true
	}
	for _, n := range templateOrder {
		if g.gfuncs[n] {
			g.out.WriteString("\n" + templates[n])
		}
	}
}

// comparableScalar returns a scalar type to instantiate a comparable parameter with.
func (g *gen) comparableScalar() *typ {
	ts := g.scalarTypes()
	return ts[g.intn(len(ts))]
}

// genericCall returns a call of a generic function with the one result t, if there is one.
func (g *gen) genericCall(t *typ, d int) (ex, bool) {
	if !g.o.Generics || d <= 1 || !g.chance(2) {
		return ex{}, false
	}
	var c []func() ex
	ts := t.String()
	numeric := t.isNumeric() && (t.under().kind == kInt || t.under().kind == kInt64 || t.under().kind == kFloat)
	if numeric {
		c = append(c, func() ex {
			g.use("Sum")
			return g.callGeneric("Sum", t, []*typ{t, t}, d)
		})
	}
	if k := t.under().kind; k == kInt || k == kFloat || k == kString {
		c = append(c, func() ex {
			g.use("Max")
			return g.callGeneric("Max", t, []*typ{t, t}, d)
		})
	}
	c = append(c, func() ex {
		g.use("Pick")
		return g.callGeneric("Pick", t, []*typ{t, t, tBool}, d)
	}, func() ex {
		g.use("Zero")
		return ex{s: "Zero[" + ts + "]()"}
	}, func() ex {
		g.use("Apply")
		f := &typ{kind: kFunc, params: []*typ{t}, results: []*typ{t}}
		return g.callGeneric("Apply", t, []*typ{t, f}, d)
	})
	if t.kind == kPtr {
		c = append(c, func() ex {
			g.use("Ptr")
			return g.callGeneric("Ptr", t.elem, []*typ{t.elem}, d)
		})
	}
	if t.kind == kSlice {
		c = append(c, func() ex {
			g.use("Map")
			a := g.comparableScalar()
			f := &typ{kind: kFunc, params: []*typ{a}, results: []*typ{t.elem}}
			return ex{s: "Map[" + a.String() + ", " + t.elem.String() + "](" + g.expr(&typ{kind: kSlice, elem: a}, d-1).s + ", " + g.funcLit(f, d-1).s + ")"}
		}, func() ex {
			g.use("Filter")
			f := &typ{kind: kFunc, params: []*typ{t.elem}, results: []*typ{tBool}}
			return ex{s: "Filter[" + t.elem.String() + "](" + g.expr(t, d-1).s + ", " + g.funcLit(f, d-1).s + ")"}
		})
		if t.elem.comparable() && t.elem.isScalar() && t.elem.kind != kBool {
			c = append(c, func() ex {
				g.use("Keys")
				v := g.valueType(1, nil)
				m := &typ{kind: kMap, key: t.elem, elem: v}
				return ex{s: "Keys[" + t.elem.String() + ", " + v.String() + "](" + g.expr(m, d-1).s + ")"}
			})
		}
	}
	if t.kind == kBool {
		c = append(c, func() ex {
			g.use("Contains")
			a := g.comparableScalar()
			return ex{s: "Contains[" + a.String() + "](" + g.expr(&typ{kind: kSlice, elem: a}, d-1).s + ", " + g.expr(a, d-1).s + ")"}
		})
	}
	c = append(c, func() ex {
		g.use("Reduce")
		a := g.comparableScalar()
		f := &typ{kind: kFunc, params: []*typ{t, a}, results: []*typ{t}}
		return ex{s: "Reduce[" + a.String() + ", " + ts + "](" + g.expr(&typ{kind: kSlice, elem: a}, d-1).s + ", " + g.expr(t, d-1).s + ", " + g.funcLit(f, d-1).s + ")"}
	})
	return c[g.intn(len(c))](), true
}

// callGeneric writes a call of the template name with the type argument targ and the arguments
// of the types params: with the type argument inferred when the first argument is not a constant.
func (g *gen) callGeneric(name string, targ *typ, params []*typ, d int) ex {
	var args []string
	first := true
	inferable := false
	for _, p := range params {
		var e ex
		if first && p.String() == targ.String() && p.isScalar() && !p.isBool() {
			e = g.nonconst(p, d-1)
			inferable = true
		} else {
			e = g.expr(p, d-1)
		}
		first = false
		args = append(args, e.s)
	}
	// An untyped constant in another place would still infer the type of the first argument.
	if inferable && g.chance(2) && targ.isScalar() {
		return ex{s: name + "(" + strings.Join(args, ", ") + ")"}
	}
	return ex{s: name + "[" + targ.String() + "](" + strings.Join(args, ", ") + ")"}
}
