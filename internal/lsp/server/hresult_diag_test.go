package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// A valid program that uses the error of a call that returns an HRESULT (ADR-137) has no diagnostic
// in the editor. The editor does not run the hook that types such a call as an error (it would have
// to ask the compiler before every diagnostic), so the checker there sees the call as a C/C++ value:
// where Go's checker goes on only with an operand that it can type (a key of a map literal that it
// can't assign skips the value next to it), the value must not be reported as an expression
// without a type.

const hresultMapShapes = `package main

import (
	"fmt"
	str "strings"

	"./hresult.hpp"
)

type T struct{ a, b int }

type Ts map[any]T

type W struct {
	S hresult.Surface
	N int
}

func main() {
	e := hresult.status_of(4)
	cv := hresult.nt_result(1)
	byError := map[error]string{e: "dev"}
	byCall := map[error]int{hresult.status_of(4): 1, hresult.status_of(2): 2}
	byValue := map[string]error{"k": e}
	mixed := map[error]error{e: e, hresult.status_of(2): nil}
	elided1 := map[error]T{e: {1, 2}}
	elided2 := map[error]map[string]int{e: {"a": 1}}
	elided3 := map[error][]int{e: {1, 2}}
	elided4 := map[error][]error{e: {e, hresult.status_of(2)}}
	elided5 := map[error]*T{e: {1, 2}}
	elided6 := map[error]struct{}{e: {}}
	byC := map[C.long]T{cv: {1, 2}, cv: {3, 4}}
	byAny := map[any][]int{cv: {1, 2}, e: {3}}
	both := map[any]string{cv: "c", e: "e", 1: "n"}
	named := Ts{cv: {1, 2}, e: {3, 4}}
	nested := map[any]map[any][]int{cv: {cv: {1}}, e: {e: {2}}}
	tagged := map[any]struct {
		A int "a"
	}{cv: {1}}
	aliased := map[any]str.Builder{cv: {}}
	withC := map[any]W{cv: {N: 1}}
	fmt.Println(len(byError), len(byCall), len(byValue), len(mixed), len(elided1), len(elided2), len(elided3),
		len(elided4), len(elided5), len(elided6), len(byC), len(byAny), len(both), len(named), len(nested), len(tagged), len(aliased), len(withC))
}
`

// hresultClient starts a server over the package p with the HRESULT header of tests/typed.
func hresultClient(t *testing.T, program string) *dclient {
	t.Helper()
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	defer probe.SetRacyWindow(0)()
	if _, err := probe.FindClang(context.Background()); err != nil {
		t.Skip(err)
	}
	header, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "typed", "hresult.hpp"))
	if err != nil {
		t.Fatal(err)
	}
	root := writeTree(t, map[string]string{
		"gocpp.mod":     "module example.com/hresult\n",
		"p/main.gp":     program,
		"p/hresult.hpp": string(header),
	})
	cfg := testConfig()
	cfg.CXX = func(ctx context.Context) (build.CXX, error) { return build.FindCXXFor(ctx, cfg.GOOS, cfg.GOARCH) }
	s := newServer(nil, cfg, debounce.RealClock{})
	// (cleanups run last to first: the hook is cleared after the pipeline has ended, so that no call is late)
	t.Cleanup(func() { hookAdapted.Store(nil) })
	t.Cleanup(s.pipeline.Close)
	c := &dclient{client: startServer(t, s, nil), root: root}
	c.call("initialize", diagInitParams)
	c.notify("initialized", "{}")
	return c
}

// noDiagnostics opens the program and fails when a publish of the document has a diagnostic: the first
// one, and every one up to the end of the background analysis of the package (the editor's own check
// ends before it, which is the one that has the typed answers), and a moment after it.
func noDiagnostics(t *testing.T, c *dclient, program string) {
	t.Helper()
	analysed := make(chan string, 16)
	hook := func(dir string) { analysed <- dir }
	hookAdapted.Store(&hook)
	c.open("p/main.gp", program)
	first := c.next("p/main.gp")
	if len(first.Diagnostics) != 0 {
		t.Fatalf("diagnostics for a valid program: %s", first.raw)
	}
	select {
	case <-analysed:
	case <-time.After(hangLimit):
		t.Fatalf("the background analysis did not end before the hang limit; the log:\n%s", c.logs.String())
	}
	// The publish that the end of the analysis causes comes after it: one more moment, then the
	// publishes seen so far are all of them.
	if p := c.publishWith("p/main.gp", 5*time.Second, func(p *publish) bool { return len(p.Diagnostics) > 0 }); p != nil {
		t.Fatalf("a later publish has diagnostics for a valid program: %s", p.raw)
	}
}

func TestHResultMapLiteralsHaveNoDiagnosticsInTheEditor(t *testing.T) {
	noDiagnostics(t, hresultClient(t, hresultMapShapes), hresultMapShapes)
}

// The whole valid program of the typed run test (error text as a string, append, nil, map keys).
func TestHResultValidProgramHasNoDiagnosticsInTheEditor(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "typed", "hresult_uses.gp"))
	if err != nil {
		t.Fatal(err)
	}
	noDiagnostics(t, hresultClient(t, string(src)), string(src))
}

// A value of the wrong type next to a key that is a C/C++ value is an error in the editor, as in
// `gocpp check`, with the words of go/types: go/types skips it, and check asks go/types for it
// against the element type of the map (an elided literal, one of a named map type or two levels
// deep, a type parameter, a constant that overflows).
func TestHResultMapValueOfTheWrongTypeIsAnErrorInTheEditor(t *testing.T) {
	const program = `package main

import (
	"fmt"

	"./hresult.hpp"
)

type W struct{ a int }

type Ws map[any]W

type G struct {
	S hresult.Surface
	N int
}

type Gen[V any] struct {
	V V
	N int
}

func g[V any]() map[any]V {
	k := hresult.nt_result(1)
	return map[any]V{k: 5}
}

func main() {
	cv := hresult.nt_result(1)
	m := map[any]int{cv: "x"}
	f32 := map[any]float32{cv: 1e40}
	f64 := map[any]float64{cv: 1e400}
	deep := map[any]map[any][]int{cv: {cv: {"y"}}}
	named := Ws{cv: {"z"}}
	under := map[any]int{cv: {1}}
	err := map[any]error{cv: 5}
	nilW := map[any]W{cv: nil}
	boolean := map[any]int{cv: true}
	cstruct := map[any]hresult.Surface{cv: {7}}
	goStruct := map[any]G{cv: {N: 1}, cv: {N: 2}}
	goStructWrong := map[any]G{cv: {N: "q"}}
	goStructSlice := map[any][]G{cv: {{N: 1}, {N: "r"}}}
	generic := map[any]Gen[hresult.NTSTATUS]{cv: {N: 1}}
	fmt.Println(len(m), f32, f64, deep, named, under, err, nilW, boolean, cstruct, g[int](), goStruct, goStructWrong, goStructSlice, generic)
}
`
	c := hresultClient(t, program)
	c.open("p/main.gp", program)
	p := c.next("p/main.gp")
	want := []string{
		`cannot use 5 (untyped int constant) as V value in map literal`,
		`cannot use "x" (untyped string constant) as int value in map literal`,
		`cannot use 1e40 (untyped float constant 1e+40) as float32 value in map literal (overflows)`,
		`cannot use 1e400 (untyped float constant 1e+400) as float64 value in map literal (overflows)`,
		`cannot use "y" (untyped string constant) as int value in array or slice literal`,
		`cannot use "z" (untyped string constant) as int value in struct literal`,
		`invalid composite literal element type int`,
		`cannot use 5 (constant of type int) as error value in map literal: int does not implement error (missing method Error)`,
		`cannot use nil as W value in map literal`,
		`cannot use true (untyped bool constant) as int value in map literal`,
		`a composite literal without its type ({…}) for a C/C++ type next to a C/C++ map key is not supported yet: write the type`,
		// a Go struct with a C/C++ field is a Go type: its literal is typed and checked like any other
		`cannot use "q" (untyped string constant) as int value in struct literal`,
		`cannot use "r" (untyped string constant) as int value in struct literal`,
		// the limit of an instance of a generic type with a C/C++ type argument
		`an instance of a generic type with a C/C++ type argument (Gen[C.NTSTATUS]) is not supported yet: the type arguments of a generic type are Go types`,
	}
	for _, msg := range want {
		if !hasMessage(p, msg) {
			t.Errorf("no error %q: %s", msg, p.raw)
		}
	}
	if len(p.Diagnostics) != len(want) {
		t.Errorf("%d diagnostics, want %d: %s", len(p.Diagnostics), len(want), p.raw)
	}
}

// The key is an error of Go (an undefined name), not a C/C++ value: the value next to it is not
// checked, as Go does not (go vet reports the key and nothing else).
func TestHResultErrorInTheKeyIsTheOnlyOneInTheEditor(t *testing.T) {
	const program = `package main

import (
	"fmt"

	"./hresult.hpp"
)

func main() {
	e := hresult.status_of(4)
	m := map[string]int{nosuch: "x"}
	fmt.Println(len(m), e)
}
`
	c := hresultClient(t, program)
	c.open("p/main.gp", program)
	p := c.next("p/main.gp")
	if !hasMessage(p, "undefined: nosuch") {
		t.Fatalf("no error for the key: %s", p.raw)
	}
	if len(p.Diagnostics) != 1 {
		t.Errorf("%d diagnostics, want 1: %s", len(p.Diagnostics), p.raw)
	}
}

// The scope that check makes round the value next to a C/C++ key (check/mapvalue.go) holds the names of
// the element type and the key type: a completion in the value must not offer them.
func TestHResultMapValueScopeNamesAreNotCompleted(t *testing.T) {
	const program = `package main

import (
	"fmt"

	"./hresult.hpp"
)

func main() {
	cv := hresult.nt_result(1)
	other := 2
	m := map[any]int{cv: other}
	fmt.Println(len(m))
}
`
	c := hresultClient(t, program)
	c.open("p/main.gp", program)
	c.next("p/main.gp")
	offset := strings.Index(program, "cv: other") + len("cv: ")
	params, err := json.Marshal(protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: c.uri("p/main.gp")},
			Position:     protocol.NewMapper(program).Position(offset),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp := c.call("textDocument/completion", string(params))
	if resp.Error != nil {
		t.Fatalf("completion: %v", resp.Error)
	}
	var list protocol.CompletionList
	if err := json.Unmarshal(resp.Result, &list); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range list.Items {
		if strings.HasPrefix(it.Label, "gp·") {
			t.Errorf("the completion offers %q", it.Label)
		}
		found = found || it.Label == "other"
	}
	if !found {
		t.Errorf("the completion in the value does not offer other (%d items)", len(list.Items))
	}
}
