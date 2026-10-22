package check

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// Config.HResults (hresults.go, ADR-137): the calls the hook names are wrapped in error(...) before
// the package is done, so go/types gives the variable that holds one the type error.

// hresultHook names the calls of cstdlib.abs that are not statements (the hook of typed interop
// names the calls whose function returns an HRESULT).
func hresultHook(_ context.Context, _ *token.FileSet, p *Package) ([]*ast.CallExpr, error) {
	var out []*ast.CallExpr
	for _, f := range p.Load.Files {
		stmts := make(map[ast.Expr]bool)
		ast.Inspect(f.AST, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.ExprStmt:
				stmts[n.X] = true
			case *ast.CallExpr:
				if types.ExprString(n.Fun) == "C.abs" && !stmts[n] && !p.IsErrorConversion(n) {
					out = append(out, n)
				}
			}
			return true
		})
	}
	return out, nil
}

// hresultSource is the program of checkHResults.
func hresultSource(body string) string {
	return "package main\n\nimport \"<cstdlib>\"\nimport \"<errors>\"\nimport \"time\"\n\nvar _ = errors.New\nvar _ = time.Second\n\nfunc main() {\n" + body + "\n}\n"
}

// checkHResults checks the body of main with the hook and returns the text of the errors.
func checkHResults(t *testing.T, body string) (p *Package, errs string) {
	t.Helper()
	return checkHResultsWith(t, body, hresultHook)
}

// checkHResultsWith is checkHResults with the hook (nil: none, as in the editor).
func checkHResultsWith(t *testing.T, body string, hook HResultCalls) (p *Package, errs string) {
	t.Helper()
	return checkHResultsFiles(t, map[string]string{"main.gp": hresultSource(body)}, hook)
}

// checkHResultsFiles checks a package of these files (main.gp has main) and returns the errors of main.gp.
func checkHResultsFiles(t *testing.T, files map[string]string, hook HResultCalls) (p *Package, errs string) {
	t.Helper()
	tree := map[string]string{"gocpp.mod": "module example.com/m\n"}
	for name, src := range files {
		tree[name] = src
	}
	root := writeTree(t, tree)
	cfg := load.Config{Dir: root}
	cfg.Stdlib, cfg.GoStd = stdlib.FS(), stdlib.IsStd
	cfg.GOOS, cfg.GOARCH = "linux", "amd64"
	prog := load.Load(cfg, ".")
	c := fakeProbeConfig(LastPhase)
	c.HResults = hook
	r := Check(prog, c)
	for _, pk := range r.Packages {
		if pk.Types == nil || pk.Info == nil {
			continue
		}
		checkScopeTree(t, pk) // the scopes of the values checked on the side are in Info too
	}
	return mainPkg(t, prog, r), errorText(t, prog, r, root, true)
}

// The variable made from the call is an error everywhere: the text is a string, the value goes into
// append and into the literals of slices and maps, and it can be set to nil.
func TestHResultVariableIsAnError(t *testing.T) {
	_, errs := checkHResults(t, `
	e := cstdlib.abs(-3)
	var s string = e.Error()
	t := "failed: " + e.Error() + "!"
	_ = e.Error()[:4]
	for _, r := range e.Error() {
		_ = r
	}
	var xs []error
	xs = append(xs, e, cstdlib.abs(-4))
	_ = []error{e, cstdlib.abs(-5)}
	_ = map[error]string{e: "dev"}
	_ = map[error]int{cstdlib.abs(-6): 1}
	e = nil
	var f error = e
	_, _, _, _ = s, t, xs, f
`)
	if errs != "" {
		t.Errorf("errors:\n%s", errs)
	}
}

// The call is typed error where it is: the conversion that check wrote is the call's parent.
func TestHResultCallIsAConversionToError(t *testing.T) {
	p, errs := checkHResults(t, "\tvar e error = cstdlib.abs(-3)\n\t_ = e")
	if errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	n := 0
	for _, c := range p.Conversions {
		if p.IsErrorConversion(c.Call) {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d conversions to error, want 1", n)
	}
}

// A call the hook leaves out stays what it was: a statement is not an error.
func TestHResultHookLeavesTheOthers(t *testing.T) {
	p, errs := checkHResults(t, "\tcstdlib.abs(-3)")
	if errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	for _, c := range p.Conversions {
		if p.IsErrorConversion(c.Call) {
			t.Error("a statement was wrapped")
		}
	}
}

// Without the hook, nothing is wrapped: the opaque pipeline is what it was.
func TestHResultNoHookNoWrap(t *testing.T) {
	src := "package main\n\nimport \"<cstdlib>\"\n\nfunc main() {\n\te := cstdlib.abs(-3)\n\t_ = e\n}\n"
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	cfg := load.Config{Dir: root}
	cfg.Stdlib, cfg.GoStd = stdlib.FS(), stdlib.IsStd
	cfg.GOOS, cfg.GOARCH = "linux", "amd64"
	prog := load.Load(cfg, ".")
	p := mainPkg(t, prog, Check(prog, fakeProbeConfig(LastPhase)))
	if len(p.Conversions) != 0 {
		t.Errorf("%d conversions without a hook", len(p.Conversions))
	}
}

// The misuse of the error is go/types' own message, with the call as the user wrote it and the
// line that says what an HRESULT is here.
func TestHResultMisuseMessages(t *testing.T) {
	for _, c := range []struct {
		name, body, want string
	}{
		{"compare", "\te := cstdlib.abs(-3)\n\tif e != 0 {\n\t}", "cannot convert 0 (untyped int constant) to type interface{Error() string}"},
		{"increment", "\te := cstdlib.abs(-3)\n\te++", "invalid operation: e++ (non-numeric type error)"},
		{"index", "\te := cstdlib.abs(-3)\n\t_ = e[0]", "cannot index e (variable of interface type error)"},
		{"number", "\tvar n int32 = cstdlib.abs(-3)\n\t_ = n", "cannot use cstdlib.abs(-3) (value of interface type error) as int32 value in variable declaration"},
		{"assign", "\te := cstdlib.abs(-3)\n\te = 5", "cannot use 5 (constant of type int) as error value in assignment"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, errs := checkHResults(t, c.body)
			if !strings.Contains(errs, c.want) {
				t.Errorf("errors:\n%s\nwant %q", errs, c.want)
			}
			if !strings.Contains(errs, "\n\t"+HResultAdvice) {
				t.Errorf("no advice in:\n%s", errs)
			}
			if strings.Contains(errs, "gp_cgoh") || strings.Contains(errs, "C.abs") {
				t.Errorf("the message shows the rewrite:\n%s", errs)
			}
		})
	}
}

// An error that is not about the HRESULT does not get the advice.
func TestHResultAdviceOnlyForTheStatement(t *testing.T) {
	_, errs := checkHResults(t, "\te := cstdlib.abs(-3)\n\t_ = e\n\tvar s string = 5\n\t_ = s")
	if !strings.Contains(errs, "cannot use 5") || strings.Contains(errs, HResultAdvice) {
		t.Errorf("errors:\n%s", errs)
	}
}

// An error goes where Go puts it, not into a C type: go/types has no type for the target, so check
// says it (the lowering says the same at the build).
func TestHResultErrorToACType(t *testing.T) {
	for _, c := range []struct{ name, body, want string }{
		{"declaration", "\te := cstdlib.abs(-3)\n\tvar n C.long = e\n\t_ = n", "cannot use e (variable of type error) as C.long value in variable declaration"},
		{"assignment", "\te := cstdlib.abs(-3)\n\tvar n C.long\n\tn = e", "cannot use e (variable of type error) as n value in assignment"},
		{"call", "\tvar n C.long = cstdlib.abs(-3)\n\t_ = n", "cannot use cstdlib.abs(-3) (value of type error) as C.long value in variable declaration"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, errs := checkHResults(t, c.body)
			if !strings.Contains(errs, c.want) || !strings.Contains(errs, "\n\t"+HResultAdvice) {
				t.Errorf("errors:\n%s\nwant %q and the advice", errs, c.want)
			}
		})
	}
}

// The wrapper keeps the place of the call: it starts and ends where the call did, so a message, a
// range of the editor and the statement spans of the hint have the user's text, not one byte more.
func TestHResultWrapperHasTheSpanOfTheCall(t *testing.T) {
	const call = "cstdlib.abs(-3)"
	p, errs := checkHResults(t, "\tvar e error = "+call+"\n\t_ = e")
	if errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	n := 0
	for _, c := range p.Conversions {
		if p.IsErrorConversion(c.Call) {
			n++
			if got := int(c.Call.End() - c.Call.Pos()); got != len(call) {
				t.Errorf("the wrapper is %d bytes long, the call %d", got, len(call))
			}
		}
	}
	if n != 1 {
		t.Fatalf("%d wrappers, want 1", n)
	}
}

// The program may declare its own error where the call is: the wrapper is Go's predeclared error
// all the same (it names gp_cgoh.Error, not error).
func TestHResultWrapperIsNotShadowedByTheProgram(t *testing.T) {
	_, errs := checkHResults(t, `
	type error int
	var n error = 7
	e := cstdlib.abs(-3)
	_ = e.Error()
	e = nil
	_ = n
`)
	if errs != "" {
		t.Errorf("errors:\n%s", errs)
	}
	_, errs = checkHResults(t, "\terror := \"x\"\n\te := cstdlib.abs(-3)\n\t_ = e.Error() + error")
	if errs != "" {
		t.Errorf("errors with a variable named error:\n%s", errs)
	}
}

// The wrapper is where the call was: its start and its end are those of the call in the source, and
// the message of a misuse of the second call on a line is at the second call.
func TestHResultWrapperIsWhereTheCallWas(t *testing.T) {
	const body = "\tvar a, b string = cstdlib.abs(-1), cstdlib.abs(-2)\n\t_, _ = a, b"
	p, errs := checkHResults(t, body)
	src := hresultSource(body)
	start := p.Load.Files[0].AST.FileStart
	n := 0
	for _, c := range p.Conversions {
		if !p.IsErrorConversion(c.Call) {
			continue
		}
		n++
		text := "cstdlib.abs(-" + string(rune('0'+n)) + ")"
		want := start + token.Pos(strings.Index(src, text))
		if c.Call.Pos() != want || c.Call.End() != want+token.Pos(len(text)) {
			t.Errorf("wrapper %d is at %d..%d, the call %s at %d..%d", n, c.Call.Pos(), c.Call.End(), text, want, want+token.Pos(len(text)))
		}
	}
	if n != 2 {
		t.Fatalf("%d wrappers, want 2", n)
	}
	line := strings.Count(src[:strings.Index(src, body)], "\n") + 1
	for _, call := range []string{"cstdlib.abs(-1)", "cstdlib.abs(-2)"} {
		col := strings.Index(strings.Split(body, "\n")[0], call) + 1
		want := ":" + itoa(line) + ":" + itoa(col) + ": cannot use " + call
		if !strings.Contains(errs, want) {
			t.Errorf("no %q in:\n%s", want, errs)
		}
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

// Only the wrapper of an HRESULT call is a conversion to error: a conversion to a type of
// a package (time.Duration), or to a type of the program that is named error (type error uint8) is not.
func TestHResultOnlyTheWrapperIsAnErrorConversion(t *testing.T) {
	p, errs := checkHResults(t, `
	e := cstdlib.abs(-3)
	n := cstdlib.labs(2)
	d := time.Duration(n)
	type error uint8
	u := error(n)
	_, _, _ = e, d, u
`)
	if errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	var wrappers, others int
	for _, c := range p.Conversions {
		if c.Kind != AsCall {
			continue
		}
		if p.IsErrorConversion(c.Call) {
			wrappers++
		} else {
			others++
		}
	}
	if wrappers != 1 || others != 2 {
		t.Errorf("%d conversions to error and %d others, want 1 and 2", wrappers, others)
	}
}

// The value next to a key that is the error of a call has the type that the map declares.
func TestHResultMapValueHasTheElementType(t *testing.T) {
	_, errs := checkHResults(t, "\te := cstdlib.abs(-3)\n\t_ = map[error]string{e: 5}\n\t_ = map[error][]int{e: {1, \"x\"}}")
	for _, want := range []string{
		"cannot use 5 (untyped int constant) as string value in map literal",
		`cannot use "x" (untyped string constant) as int value in array or slice literal`,
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("no %q in:\n%s", want, errs)
		}
	}
}

// Without the hook (the editor) the call is a C/C++ value, and the key of a map literal that go/types
// can't assign makes it skip the value next to it. The value is checked against the element type of
// the map all the same, by go/types itself (mapvalue.go), and an elided literal has the element type;
// the valid shapes have no error at all. A case with a key that Go accepts (the call changed to a
// number, the key type any) has to have the same errors, word for word: that is what go vet says.
func TestMapValuesNextToACKeyAreChecked(t *testing.T) {
	const prelude = "\ttype T struct{ a, b int }\n\ttype W struct{ a int }\n\ttype F float64\n" +
		"\ttype Str interface{ String() string }\n\ttype M map[any]T\n" +
		"\tcv := cstdlib.abs(-3)\n\ts := \"a\"\n\t_ = s\n"
	// the cases of a function of the file, not of main
	const generic = "\n}\n\nfunc g[V any]() map[any]V {\n\tk := cstdlib.abs(-1)\n\treturn map[any]V{k: 5}"
	for _, c := range []struct{ name, body, want string }{
		{"valid", "_ = map[any]int{cv: 1, 2: 3}\n\t_ = map[error]string{cv: \"dev\"}\n\t_ = map[C.long]int{cv: 1}", ""},
		{"valid elided", "_ = map[any]T{cv: {1, 2}}\n\t_ = map[any][]int{cv: {1, 2}}\n\t_ = map[any]*T{cv: {1, 2}}\n" +
			"\t_ = map[any]struct{}{cv: {}}\n\t_ = map[any]map[string]int{cv: {\"a\": 1}}\n\t_ = map[any][]T{cv: {{1, 2}}}", ""},
		{"valid Go struct with a C field", "type G struct {\n\t\tS C.div_t\n\t\tN int\n\t}\n\t_ = map[any]G{cv: {N: 1}}\n\t_ = map[any]*G{cv: {N: 2}}", ""},
		{"valid Go struct with a C field, every shape", "type G struct {\n\t\tS C.div_t\n\t\tN int\n\t}\n\ttype H struct {\n\t\tG G\n\t\tN int\n\t}\n\t" +
			"_ = map[any]G{cv: {N: 1}}\n\t_ = map[any]*G{cv: {N: 2}}\n\t_ = map[any][]G{cv: {{N: 3}}}\n\t_ = map[any]map[string]G{cv: {\"a\": {N: 4}}}\n" +
			"\t_ = map[any][2]G{cv: {{N: 5}, {N: 6}}}\n\t_ = map[any]H{cv: {G: G{N: 7}, N: 8}}\n\t_ = map[any]H{cv: {G: G{N: 7}}}\n" +
			"\t_ = map[any]*H{cv: {N: 9}}\n\t_ = map[any][]*H{cv: {{N: 9}}}\n\t_ = map[any]map[any]G{cv: {cv: {N: 1}}}", ""},
		{"valid C element", "_ = map[any]C.long{cv: 1}\n\t_ = map[any]*C.div_t{cv: nil}\n\ttype D C.div_t\n\t_ = map[any]C.div_t{cv: C.div_t{}}", ""},
		{"valid shadowed element type", "{\n\t\tT := 5\n\t\t_ = T\n\t\t_ = M{cv: {1, 2}}\n\t}", ""},
		{"valid pointer", "_ = map[any]*T{cv: nil}\n\t_ = map[any]*T{cv: &T{1, 2}}\n\t_ = map[any]time.Duration{cv: time.Second}", ""},
		{"valid tagged struct", "var tagged struct {\n\t\tA int `json:\"a\"`\n\t}\n\t_ = map[any]struct {\n\t\tA int `json:\"a\"`\n\t}{cv: tagged}", ""},
		{"valid embedded field", "var emb struct{ T }\n\t_ = map[any]struct{ T }{cv: emb}", ""},
		{"valid named map", "_ = M{cv: {1, 2}}\n\t_ = M{cv: T{3, 4}}", ""},
		{"valid closure in a nested value of a Go key", "_ = map[any]map[string]func(){cv: {\"a\": func() {\n\t\tx := 1\n\t\t_ = x\n\t}}}", ""},
		{"valid nested", "_ = map[any]map[any][]int{cv: {cv: {1}}}\n\t_ = map[any]map[any]T{cv: {cv: {1, 2}}}", ""},
		{"valid bounds", "_ = map[any]int32{cv: -2147483648}\n\t_ = map[any]int32{cv: 2147483647}\n" +
			"\t_ = map[any]uint32{cv: 4294967295}\n\t_ = map[any]int16{cv: -32768}\n\t_ = map[any]uint16{cv: 65535}\n" +
			"\t_ = map[any]int64{cv: 9223372036854775807}\n\t_ = map[any]uint64{cv: 18446744073709551615}\n" +
			"\t_ = map[any]float32{cv: 3e38}\n\t_ = map[any]float64{cv: 1e308}\n\t_ = map[any]float64{cv: 1 + 0i}", ""},
		// an elided literal of a C/C++ type: the lowering can't write it, so check refuses it
		{"elided C struct", "_ = map[any]C.div_t{cv: {1, 2}}", "for a C/C++ type next to a C/C++ map key is not supported yet: write the type"},
		{"elided C slice", "_ = map[any][]C.div_t{cv: {{1, 2}}}", "is not supported yet: write the type"},
		{"elided C nested", "_ = map[any]map[any]C.div_t{cv: {cv: {1, 2}}}", "is not supported yet: write the type"},
		// a Go type with a C/C++ field is a Go type: its elided literal is typed and checked like any other
		{"Go struct with a C field, wrong value", "type G struct {\n\t\tS C.div_t\n\t\tN int\n\t}\n\ttype H struct {\n\t\tG G\n\t\tN int\n\t}\n\t" +
			"_ = map[any]G{cv: {N: \"x\"}}", `cannot use "x" (untyped string constant) as int value in struct literal`},
		{"Go struct with a C field, pointer, wrong value", "type G struct {\n\t\tS C.div_t\n\t\tN int\n\t}\n\ttype H struct {\n\t\tG G\n\t\tN int\n\t}\n\t" +
			"_ = map[any]*G{cv: {N: \"x\"}}", `cannot use "x" (untyped string constant) as int value in struct literal`},
		{"Go struct with a C field, slice, wrong value", "type G struct {\n\t\tS C.div_t\n\t\tN int\n\t}\n\ttype H struct {\n\t\tG G\n\t\tN int\n\t}\n\t" +
			"_ = map[any][]G{cv: {{N: \"x\"}}}", `cannot use "x" (untyped string constant) as int value in struct literal`},
		{"Go struct with a C field, map, wrong value", "type G struct {\n\t\tS C.div_t\n\t\tN int\n\t}\n\ttype H struct {\n\t\tG G\n\t\tN int\n\t}\n\t" +
			"_ = map[any]map[string]G{cv: {\"a\": {N: \"x\"}}}", `cannot use "x" (untyped string constant) as int value in struct literal`},
		{"Go struct with a C field, array, wrong value", "type G struct {\n\t\tS C.div_t\n\t\tN int\n\t}\n\ttype H struct {\n\t\tG G\n\t\tN int\n\t}\n\t" +
			"_ = map[any][2]G{cv: {{N: 1}, {N: \"x\"}}}", `cannot use "x" (untyped string constant) as int value in struct literal`},
		{"nested Go struct with a C field, wrong value", "type G struct {\n\t\tS C.div_t\n\t\tN int\n\t}\n\ttype H struct {\n\t\tG G\n\t\tN int\n\t}\n\t" +
			"_ = map[any]H{cv: {G: G{N: \"x\"}}}", `cannot use "x" (untyped string constant) as int value in struct literal`},
		{"Go struct with a C field, too many", "type G struct {\n\t\tS C.div_t\n\t\tN int\n\t}\n\ttype H struct {\n\t\tG G\n\t\tN int\n\t}\n\t" +
			"_ = map[any]G{cv: {1, 2, 3}}", "too many values in struct literal of type G"},
		{"Go struct with a C field, a number", "type G struct {\n\t\tS C.div_t\n\t\tN int\n\t}\n\ttype H struct {\n\t\tG G\n\t\tN int\n\t}\n\t" +
			"_ = map[any]G{cv: 5}", "cannot use 5 (untyped int constant) as G value in map literal"},
		// an instance of a generic Go type with a C/C++ type argument is the limit, not the elided literal
		{"generic with a C argument", "\n}\n\ntype Gen[V any] struct {\n\tV V\n\tN int\n}\n\nfunc g2() {\n\tk := cstdlib.abs(-1)\n\t_ = map[any]Gen[C.div_t]{k: {N: 1}}",
			"an instance of a generic type with a C/C++ type argument (Gen[C.div_t]) is not supported yet: the type arguments of a generic type are Go types"},
		{"generic with a C argument, slice", "\n}\n\ntype Gen[V any] struct {\n\tV V\n\tN int\n}\n\nfunc g2() {\n\tk := cstdlib.abs(-1)\n\t_ = map[any][]Gen[C.div_t]{k: {{N: 1}}}",
			"an instance of a generic type with a C/C++ type argument (Gen[C.div_t]) is not supported yet"},
		{"generic with a C argument and a type error elsewhere", "\n}\n\ntype Gen[V any] struct {\n\tV V\n\tN int\n}\n\nfunc g2() {\n\tk := cstdlib.abs(-1)\n\t_ = map[any]Gen[C.div_t]{k: {N: 1}}\n\tvar x int = \"s\"\n\t_ = x",
			"cannot use \"s\" (untyped string constant) as int value in variable declaration"},
		{"generic with a Go argument", "\n}\n\ntype Gen[V any] struct {\n\tV V\n\tN int\n}\n\nfunc g2() {\n\tk := cstdlib.abs(-1)\n\t_ = map[any]Gen[int]{k: {N: \"x\"}}",
			`cannot use "x" (untyped string constant) as int value in struct literal`},
		{"constant", "_ = map[any]int{cv: \"x\"}", `cannot use "x" (untyped string constant) as int value in map literal`},
		{"variable", "_ = map[C.long]int{cv: s}", "cannot use s (variable of type string) as int value in map literal"},
		{"error element", "_ = map[any]error{cv: 5}", "cannot use 5 (constant of type int) as error value in map literal: int does not implement error (missing method Error)"},
		{"error element string", "_ = map[any]error{cv: \"e\"}", "cannot use \"e\" (constant of type string) as error value in map literal: string does not implement error (missing method Error)"},
		{"interface element", "_ = map[any]Str{cv: 5}", "as Str value in map literal: int does not implement Str (missing method String)"},
		{"interface variable", "var n int\n\t_ = map[any]Str{cv: n}", "as Str value in map literal: int does not implement Str (missing method String)"},
		{"overflow", "_ = map[any]int8{cv: 300}", "cannot use 300 (untyped int constant) as int8 value in map literal (overflows)"},
		{"unsigned", "_ = map[any]uint{cv: -1}", "cannot use -1 (untyped int constant) as uint value in map literal (overflows)"},
		{"int32 high", "_ = map[any]int32{cv: 2147483648}", "(overflows)"},
		{"int32 low", "_ = map[any]int32{cv: -2147483649}", "(overflows)"},
		{"uint32", "_ = map[any]uint32{cv: 4294967296}", "(overflows)"},
		{"int16", "_ = map[any]int16{cv: 32768}", "(overflows)"},
		{"uint16", "_ = map[any]uint16{cv: 65536}", "(overflows)"},
		{"int64", "_ = map[any]int64{cv: 9223372036854775808}", "(overflows)"},
		{"uint64", "_ = map[any]uint64{cv: 18446744073709551616}", "(overflows)"},
		{"float32", "_ = map[any]float32{cv: 1e40}", "cannot use 1e40 (untyped float constant 1e+40) as float32 value in map literal (overflows)"},
		{"float64", "_ = map[any]float64{cv: 1e400}", "as float64 value in map literal (overflows)"},
		{"complex to float", "_ = map[any]float64{cv: 1 + 2i}", "(overflows)"},
		{"truncated", "_ = map[any]int{cv: 2.5}", "cannot use 2.5 (untyped float constant) as int value in map literal (truncated)"},
		{"bool", "_ = map[any]bool{cv: 1}", "cannot use 1 (untyped int constant) as bool value in map literal"},
		{"bool constant", "_ = map[any]int{cv: true}", "cannot use true (untyped bool constant) as int value in map literal"},
		{"nil", "_ = map[any]int{cv: nil}", "cannot use nil as int value in map literal"},
		{"nil struct", "_ = map[any]W{cv: nil}", "cannot use nil as W value in map literal"},
		{"pointer value", "_ = map[any]*T{cv: T{1, 2}}", "cannot use T{…} (value of struct type T) as *T value in map literal"},
		{"package type", "_ = map[any]time.Duration{cv: \"x\"}", `cannot use "x" (untyped string constant) as time.Duration value in map literal`},
		{"nested typed", "_ = map[any]map[any]int{cv: map[any]int{cv: \"x\"}}", `cannot use "x" (untyped string constant) as int value in map literal`},
		{"nested error in the key", "_ = map[any]map[string]int{cv: {nosuch: \"x\"}}", "undefined: nosuch"},
		{"typed", "var f float64\n\t_ = map[any]int{cv: f}", "cannot use f (variable of type float64) as int value in map literal"},
		{"struct value", "_ = map[any]int{cv: W{}}", "(value of struct type W)"},
		{"float type", "var f F\n\t_ = map[any]int{cv: f}", "(variable of float64 type F)"},
		{"elided field", "_ = map[any]T{cv: {1, \"x\"}}", `cannot use "x" (untyped string constant) as int value in struct literal`},
		{"elided too many", "_ = map[any]T{cv: {1, 2, 3}}", "too many values in struct literal of type T"},
		{"elided pointer", "_ = map[any]*T{cv: {\"x\", 2}}", `cannot use "x" (untyped string constant) as int value in struct literal`},
		{"elided under int", "_ = map[any]int{cv: {1}}", "invalid composite literal element type int"},
		{"nested", "_ = map[any]map[string]int{cv: {\"a\": \"x\"}}", `cannot use "x" (untyped string constant) as int value in map literal`},
		{"nested duplicate", "_ = map[any]map[string]int{cv: {\"a\": 1, \"a\": 2}}", `duplicate key "a" in map literal`},
		{"two levels", "_ = map[any]map[any][]int{cv: {cv: {\"x\"}}}", `cannot use "x" (untyped string constant) as int value in array or slice literal`},
		{"three levels", "_ = map[any]map[any]map[any]int{cv: {cv: {cv: \"x\"}}}", `cannot use "x" (untyped string constant) as int value in map literal`},
		{"inner level of a Go key", "_ = map[any]map[string]map[any]int{cv: {\"a\": {cv: \"x\"}}}", `cannot use "x" (untyped string constant) as int value in map literal`},
		{"named map", "_ = M{cv: {1, \"x\"}}", `cannot use "x" (untyped string constant) as int value in struct literal`},
		{"type parameter", generic, "cannot use 5 (untyped int constant) as V value in map literal"},
		{"duplicate keys", "_ = map[any]int{cv: 1, 2: 3, 2: 4}", "duplicate key 2 in map literal"},
		// The key is a Go error, not a C/C++ value: Go reports the key and nothing about the value.
		{"error in the key", "_ = map[string]int{nosuch: \"x\"}", "undefined: nosuch"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, errs := checkHResultsWith(t, prelude+"\t"+c.body, nil)
			if c.want == "" {
				if errs != "" {
					t.Errorf("errors:\n%s", errs)
				}
				return
			}
			if !strings.Contains(errs, c.want) {
				t.Errorf("no %q in:\n%s", c.want, errs)
			}
			if strings.Contains(errs, "(ast:") || strings.Contains(errs, "C/C++ value") {
				t.Errorf("a leak of the machinery:\n%s", errs)
			}
			if strings.HasSuffix(c.name, "type error elsewhere") && strings.Contains(errs, "not supported yet") {
				t.Errorf("a limit of C/C++ types next to a type error of the program:\n%s", errs)
			}
			if c.name == "error in the key" && strings.Contains(errs, "cannot use") {
				t.Errorf("an error about the value, which Go does not report:\n%s", errs)
			}
			if strings.Contains(c.body, "C.") || strings.Contains(c.body, "map[error]") {
				return
			}
			// the same program with a key of Go: the same errors, word for word
			src := strings.NewReplacer("cstdlib.abs(-3)", "7", "cstdlib.abs(-1)", "7").Replace(prelude + "\t" + c.body)
			_, goErrs := checkHResultsWith(t, src, nil)
			if got, want := errorMessages(errs), errorMessages(goErrs); got != want {
				t.Errorf("not the errors of Go:\n%s\nGo says:\n%s", got, want)
			}
		})
	}
}

// The element type of the map is not written as a name again where the value is, so a type that
// needs no name there (an aliased or shadowed import, an anonymous struct with a tag or an embedded
// field, a type of another package or file that this file doesn't import, an unexported one, a local
// type that an inner block shadows) is checked like any other. Each program is valid, or it has the
// errors that Go gives for it with a key of Go (compared word for word).
func TestMapValuesTypesThatNeedNoName(t *testing.T) {
	const lib = "package lib\n\ntype Pair struct{ A, B int }\n\ntype pair struct{ A, B int }\n\n" +
		"type PM map[any]Pair\n\ntype HM map[any]pair\n\ntype Shape interface{ Area() float64 }\n\n" +
		"type List[T any] struct{ Items []T }\n"
	const head = "package main\n\nimport \"<cstdlib>\"\nimport str \"bytes\"\nimport \"unsafe\"\nimport . \"io\"\n" +
		"import \"example.com/m/lib\"\n\nvar _ = str.NewBufferString\nvar _ unsafe.Pointer\nvar _ Reader\nvar _ lib.Pair\n\n"
	const key = "cv := cstdlib.abs(-3)\n\t_ = cv\n\t"
	for _, c := range []struct {
		name, body string
		extra      map[string]string
		want       string // "" for a valid program
	}{
		{name: "aliased import", body: "_ = map[any]str.Buffer{cv: {}}"},
		{name: "aliased import value", body: "_ = map[any]*str.Buffer{cv: str.NewBufferString(\"x\")}"},
		{name: "aliased import error", body: "_ = map[any]str.Buffer{cv: 5}", want: "as bytes.Buffer value in map literal"},
		{name: "anonymous struct with a tag", body: "_ = map[any]struct {\n\t\tA int `json:\"a\"`\n\t}{cv: {1}}"},
		{name: "anonymous struct with a tag error", body: "_ = map[any]struct {\n\t\tA int `json:\"a\"`\n\t}{cv: {\"x\"}}", want: `cannot use "x" (untyped string constant) as int value in struct literal`},
		{name: "anonymous struct with an embedded field", body: "type T struct{ A int }\n\t_ = map[any]struct{ T }{cv: {T{1}}}"},
		{name: "anonymous struct with an embedded field error", body: "type T struct{ A int }\n\t_ = map[any]struct{ T }{cv: {5}}", want: "as T value in struct literal"},
		{name: "package type of a file that does not import it", body: "_ = MP{cv: {1, 2}}",
			extra: map[string]string{"a.gp": "package main\n\nimport \"example.com/m/lib\"\n\ntype MP map[any]lib.Pair\n\nfunc UseA() { _ = lib.Pair{} }\n"}},
		{name: "package type of a file that does not import it error", body: "_ = MP{cv: {1, \"x\"}}",
			extra: map[string]string{"a.gp": "package main\n\nimport \"example.com/m/lib\"\n\ntype MP map[any]lib.Pair\n\nfunc UseA() { _ = lib.Pair{} }\n"},
			want:  `cannot use "x" (untyped string constant) as int value in struct literal`},
		{name: "map type of another package", body: "_ = lib.PM{cv: {1, 2}}"},
		{name: "unexported type of another package", body: "_ = lib.HM{cv: {1, 2}}"},
		{name: "unexported type error", body: "_ = lib.HM{cv: {1, \"x\"}}", want: `cannot use "x" (untyped string constant) as int value in struct literal`},
		{name: "shadowed package name", body: "{\n\t\tstr := 5\n\t\t_ = str\n\t\t_ = map[any]*lib.Pair{cv: {1, 2}}\n\t\t_ = MQ{cv: {}}\n\t}",
			extra: map[string]string{"a.gp": "package main\n\nimport str \"bytes\"\n\ntype MQ map[any]str.Buffer\n"}},
		{name: "shadowed local type", body: "type L struct{ A int }\n\ttype ML map[any]L\n\t{\n\t\ttype L int\n\t\tvar l L\n\t\t_ = l\n\t\t_ = ML{cv: {1}}\n\t}"},
		{name: "shadowed local type error", body: "type L struct{ A int }\n\ttype ML map[any]L\n\t{\n\t\ttype L int\n\t\t_ = ML{cv: {\"x\"}}\n\t}", want: `cannot use "x" (untyped string constant) as int value in struct literal`},
		{name: "shadowed int", body: "type int string\n\t_ = map[any]int{cv: 5}", want: "cannot use 5 (untyped int constant) as int value in map literal"},
		{name: "interface with a method", body: "_ = map[any]interface{ Area() float64 }{cv: 5}", want: "int does not implement interface{Area() float64} (missing method Area)"},
		{name: "named interface of a package", body: "_ = map[any]lib.Shape{cv: 5}", want: "int does not implement lib.Shape (missing method Area)"},
		{name: "dot imported type", body: "_ = map[any]Reader{cv: 5}", want: "int does not implement io.Reader (missing method Read)"},
		{name: "unsafe pointer", body: "_ = map[any]unsafe.Pointer{cv: 5}", want: "as unsafe.Pointer value in map literal"},
		{name: "closure in the value", body: "_ = map[any]func() map[any]int{cv: func() map[any]int {\n\t\tx := 1\n\t\treturn map[any]int{cv: x}\n\t}}"},
		{name: "closure in the value error", body: "_ = map[any]func() map[any]int{cv: func() map[any]int {\n\t\tx := \"s\"\n\t\treturn map[any]int{cv: x}\n\t}}", want: "cannot use x (variable of type string) as int value in map literal"},
		{name: "undefined type", body: "_ = map[any]Nosuch{cv: {1}}", want: "undefined: Nosuch"},
		{name: "unsafe pointer nil", body: "_ = map[any]unsafe.Pointer{cv: nil}"},
		// the shapes that a type written out by name could get wrong
		{name: "generic instance", body: "_ = map[any]lib.List[int]{cv: {Items: []int{1}}}"},
		{name: "generic instance error", body: "_ = map[any]lib.List[int]{cv: {Items: []string{\"a\"}}}", want: "as []int value in struct literal"},
		{name: "generic instance of the program", body: "_ = map[any]Gen[string]{cv: {V: \"a\"}}",
			extra: map[string]string{"a.gp": "package main\n\ntype Gen[T any] struct{ V T }\n"}},
		{name: "generic instance of the program error", body: "_ = map[any]Gen[string]{cv: {V: 1}}", want: "as string value in struct literal",
			extra: map[string]string{"a.gp": "package main\n\ntype Gen[T any] struct{ V T }\n"}},
		{name: "channel direction", body: "ch := make(<-chan int)\n\t_ = map[any]chan<- int{cv: ch}", want: "as chan<- int value in map literal"},
		{name: "channel direction valid", body: "ch := make(chan int)\n\t_ = map[any]chan<- int{cv: ch}"},
		{name: "array length", body: "_ = map[any][3]int{cv: [...]int{1, 2, 3, 4}}", want: "as [3]int value in map literal"},
		{name: "array elided too many", body: "_ = map[any][3]int{cv: {1, 2, 3, 4}}", want: "index 3 is out of bounds (>= 3)"},
		{name: "variadic", body: "_ = map[any]func(int, ...string){cv: func(a int, b []string) {}}", want: "as func(int, ...string) value in map literal"},
		{name: "variadic valid", body: "_ = map[any]func(int, ...string){cv: func(a int, b ...string) {}}"},
		{name: "function parameters swapped", body: "_ = map[any]func(a int, b string){cv: func(a string, b int) {}}", want: "as func(a int, b string) value in map literal"},
		{name: "function results swapped", body: "_ = map[any]func() (int, string){cv: func() (string, int) { return \"\", 0 }}", want: "as func() (int, string) value in map literal"},
		{name: "struct field types", body: "_ = map[any]struct {\n\t\tA string\n\t\tB int\n\t}{cv: struct {\n\t\tA int\n\t\tB int\n\t}{}}", want: "as struct{A string; B int} value in map literal"},
		{name: "type parameter", body: "G(1)",
			extra: map[string]string{"a.gp": "package main\n\nimport \"<cstdlib>\"\n\nfunc G[V any](v V) {\n\tcv := cstdlib.abs(-3)\n\t_ = map[any][]V{cv: {5}}\n\t_ = map[any]V{cv: v}\n}\n"},
			want:  "cannot use 5 (untyped int constant) as V value in array or slice literal"},
		{name: "type parameter constraint", body: "G(1)",
			extra: map[string]string{"a.gp": "package main\n\nimport \"<cstdlib>\"\n\nfunc G[V ~int](v V) {\n\tcv := cstdlib.abs(-3)\n\t_ = map[any]V{cv: 5}\n\t_ = map[any]V{cv: v}\n}\n"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			files := map[string]string{
				"lib/lib.gp": lib,
				"main.gp":    head + "func main() {\n\t" + key + c.body + "\n}\n",
			}
			for name, src := range c.extra {
				files[name] = src
			}
			_, errs := checkHResultsFiles(t, files, nil)
			if c.want == "" {
				if errs != "" {
					t.Errorf("errors:\n%s", errs)
				}
				return
			}
			if !strings.Contains(errs, c.want) {
				t.Errorf("no %q in:\n%s", c.want, errs)
			}
			if strings.Contains(errs, "(ast:") || strings.Contains(errs, "C/C++ value") {
				t.Errorf("a leak of the machinery:\n%s", errs)
			}
			goFiles := map[string]string{}
			for name, src := range files {
				goFiles[name] = strings.ReplaceAll(src, "cstdlib.abs(-3)", "7")
			}
			_, goErrs := checkHResultsFiles(t, goFiles, nil)
			if got, want := errorMessages(errs), errorMessages(goErrs); got != want {
				t.Errorf("not the errors of Go:\n%s\nGo says:\n%s", got, want)
			}
		})
	}
}

// errorMessages returns the messages of the text of errors, without the positions.
func errorMessages(errs string) string {
	var out []string
	for _, line := range strings.Split(errs, "\n") {
		if i := strings.Index(line, ": "); i >= 0 && strings.HasPrefix(line, "./") {
			out = append(out, line[i+2:])
		}
	}
	sort.Strings(out)
	return strings.Join(out, "\n")
}

// go/types names the function of a call in "2 variables but f returns 1 value": for the wrapper of an
// HRESULT call that is the function the user wrote, as Go says it for any call, not the machinery.
func TestHResultAssignmentMismatchNamesTheFunction(t *testing.T) {
	for _, body := range []string{
		"\ta, b := cstdlib.abs(-3)\n\t_, _ = a, b",
		"\tvar a, b error = cstdlib.abs(-3)\n\t_, _ = a, b",
		"\ta, b := (cstdlib.abs(-3))\n\t_, _ = a, b",
		"\tn := 1\n\ta, b := ( /* c */ cstdlib.abs(n))\n\t_, _ = a, b",
	} {
		_, errs := checkHResults(t, body)
		if !strings.Contains(errs, "assignment mismatch: 2 variables but cstdlib.abs returns 1 value") {
			t.Errorf("errors for %q:\n%s", body, errs)
		}
		if strings.Contains(errs, "gp_cgoh") || strings.Contains(errs, "\x00") {
			t.Errorf("the machinery in the message:\n%s", errs)
		}
	}
}

// A package without any C/C++ use never asks the hook (it costs a compiler analysis).
func TestHResultHookIsNotAskedWithoutC(t *testing.T) {
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": "package main\n\nfunc main() {\n\t_ = 1\n}\n"})
	cfg := load.Config{Dir: root}
	cfg.Stdlib, cfg.GoStd = stdlib.FS(), stdlib.IsStd
	cfg.GOOS, cfg.GOARCH = "linux", "amd64"
	prog := load.Load(cfg, ".")
	asked := 0
	c := fakeProbeConfig(LastPhase)
	c.HResults = func(context.Context, *token.FileSet, *Package) ([]*ast.CallExpr, error) {
		asked++
		return nil, nil
	}
	Check(prog, c)
	if asked != 0 {
		t.Errorf("the hook was asked %d times for a package without C/C++", asked)
	}
}

// A message that holds an elided literal ({1} in []T{{1}}, which has no type expression) writes it
// as {…}: go/types prints the missing type as "(ast: <nil>)", which no message of ours may show.
func TestSourceExprStringHasNoMissingType(t *testing.T) {
	e, err := parser.ParseExpr("[]T{{1}, {2, 3}}")
	if err != nil {
		t.Fatal(err)
	}
	inner := e.(*ast.CompositeLit).Elts[0]
	if got := sourceExprString(inner); strings.Contains(got, "ast:") || got != "{…}" {
		t.Errorf("elided literal is %q, want {…}", got)
	}
	if got := sourceExprString(e); strings.Contains(got, "ast:") {
		t.Errorf("literal with elided ones is %q", got)
	}
	// a string that holds the words is a string
	s, err := parser.ParseExpr(`[]string{"(ast: <nil>){"}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := sourceExprString(s); got != "[]string{…}" {
		t.Errorf("literal of strings is %q", got)
	}
	str, err := parser.ParseExpr(`"(ast: <nil>){1}"`)
	if err != nil {
		t.Fatal(err)
	}
	if got := sourceExprString(str); got != `"(ast: <nil>){1}"` {
		t.Errorf("string is %q", got)
	}
}

// The values of nested map literals that have a C/C++ key at every level are each checked once: a
// literal of 24 levels takes as long as a few of them (it took 2^levels checks).
func TestMapValuesNestedLevelsAreLinear(t *testing.T) {
	const levels = 24
	typ := "map[any]int"
	for range levels - 1 {
		typ = "map[any]" + typ
	}
	value := func(innermost string) string {
		v := innermost
		for range levels - 1 {
			v = "{cv: " + v + "}"
		}
		return v
	}
	prelude := "\tcv := cstdlib.abs(-3)\n\t_ = " + typ + "{cv: "
	start := time.Now()
	_, errs := checkHResultsWith(t, prelude+value("1")+"}", nil)
	if errs != "" {
		t.Errorf("errors:\n%s", errs)
	}
	if d := time.Since(start); d > 20*time.Second {
		t.Errorf("%d levels take %v", levels, d)
	}
	_, errs = checkHResultsWith(t, prelude+value(`"x"`)+"}", nil)
	if n := strings.Count(errs, `cannot use "x"`); n != 1 {
		t.Errorf("%d errors for the innermost value, want 1:\n%s", n, errs)
	}
}

// The advice about the number is for the statement that treats the error as one: not for a mistake
// next to a use of the error as an error (its method).
func TestHResultAdviceIsNotForAMethodOfTheError(t *testing.T) {
	_, errs := checkHResults(t, "\tk, l := cstdlib.abs(-3).Error()\n\t_, _ = k, l\n\thr := cstdlib.abs(-3)\n\tm, n := hr.Error()\n\t_, _ = m, n")
	if !strings.Contains(errs, "assignment mismatch: 2 variables but") {
		t.Fatalf("no error:\n%s", errs)
	}
	if strings.Contains(errs, "returns an HRESULT") {
		t.Errorf("advice for the use of the error as an error:\n%s", errs)
	}
	_, errs = checkHResults(t, "\thr := cstdlib.abs(-3)\n\tif hr != 0 {\n\t}")
	if !strings.Contains(errs, "returns an HRESULT") {
		t.Errorf("no advice for the comparison with 0:\n%s", errs)
	}
}

// The error of a value next to a C/C++ key is at the value, where Go puts it (the line and the column
// of the text that is wrong), and the names of the machinery are not names of the program: a program can
// have a type called elem and one called key (the scope of the value has its own names, which no
// program can write: the lexer takes no U+00B7 in a name).
func TestMapValueErrorsAreAtTheValue(t *testing.T) {
	for _, c := range []struct{ name, body, at string }{
		{"constant", "_ = map[any]int{cv: \"x\"}", `"x"`},
		{"second element", "_ = map[any]int{1: 2, cv: \"x\"}", `"x"`},
		{"field", "type T struct{ a, b int }\n\t_ = map[any]T{cv: {1, \"x\"}}", `"x"`},
		{"nested", "_ = map[any]map[string]int{cv: {\"a\": \"x\"}}", `"x"`},
		{"too many", "type T struct{ a, b int }\n\t_ = map[any]T{cv: {1, 2, 3}}", `3`},
		{"second line", "_ = map[any]int{\n\t\tcv: 1,\n\t\tcv: \"x\",\n\t}", `"x"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			body := "\tcv := cstdlib.abs(-3)\n\t" + c.body
			_, errs := checkHResultsWith(t, body, nil)
			src := hresultSource(body)
			i := strings.LastIndex(src, c.at)
			line := 1 + strings.Count(src[:i], "\n")
			col := i - strings.LastIndex(src[:i], "\n")
			if want := fmt.Sprintf("./main.gp:%d:%d: ", line, col); !strings.Contains(errs, want) {
				t.Errorf("no error at %s in:\n%s", want, errs)
			}
		})
	}
}

func TestMapValueScopeNamesAreNoNamesOfTheProgram(t *testing.T) {
	_, errs := checkHResultsWith(t, `
	cv := cstdlib.abs(-3)
	type elem struct{ a int }
	type key string
	_ = map[any]int{cv: elem{1}.a}
	_ = map[any]string{cv: string(key("x"))}
	_ = map[any]elem{cv: {1}}
	_ = map[any]key{cv: "k"}
`, nil)
	if errs != "" {
		t.Errorf("errors:\n%s", errs)
	}
	_, errs = checkHResultsWith(t, "\tvar gp·elem int\n\t_ = gp·elem", nil)
	if !strings.Contains(errs, "U+00B7") {
		t.Errorf("a name with U+00B7 is accepted:\n%s", errs)
	}
}

// An elided literal of a C/C++ type is refused once, with the literal as it is written ({} for an
// empty one), and nothing else is said.
func TestElidedLiteralOfACTypeIsRefusedOnce(t *testing.T) {
	for _, c := range []struct{ body, text string }{
		{"_ = map[any]C.div_t{cv: {1, 2}}", "{…}"},
		{"_ = map[any]C.div_t{cv: {}}", "{}"},
		{"_ = map[any][]C.div_t{cv: {{1, 2}}}", "{…}"},
	} {
		_, errs := checkHResultsWith(t, "\tcv := cstdlib.abs(-3)\n\t"+c.body, nil)
		want := "a composite literal without its type (" + c.text + ") for a C/C++ type next to a C/C++ map key is not supported yet: write the type"
		if got := errorMessages(errs); got != want {
			t.Errorf("%s:\n%s\nwant only:\n%s", c.body, got, want)
		}
	}
}

// A limit of C/C++ types is not said next to a type error of the program: go/types leaves a type invalid
// for an error of its own too (an undefined type, an instance of a generic type with a bad argument).
func TestElidedLiteralLimitIsNotSaidNextToATypeError(t *testing.T) {
	for _, body := range []string{
		"cv := cstdlib.abs(-3)\n\t_ = map[any]Nosuch{cv: {1}}",
		"cv := cstdlib.abs(-3)\n\t_ = map[any]Gen[Nosuch]{cv: {1}}\n\ttype Gen[V any] struct{ V V }",
		"cv := cstdlib.abs(-3)\n\t_ = map[any]C.div_t{cv: {1, 2}}\n\tvar x int = \"s\"\n\t_ = x",
	} {
		_, errs := checkHResultsWith(t, body, nil)
		if errs == "" || strings.Contains(errs, "not supported yet") {
			t.Errorf("%s:\n%s", body, errs)
		}
	}
}
