package check

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// derived loads the single file src as package main (with the syntax of Phase 10), checks it, and
// returns the source text of the methods that #Derive generated ("Type.Method" -> text) and the
// errors that gocpp check prints.
func derived(t *testing.T, src string) (map[string]string, string) {
	t.Helper()
	dir := writeTree(t, map[string]string{"main.gp": src})
	prog, r := loadAndCheck(t, load.Config{Phase10Syntax: true}, dir, "main.gp")
	p := mainPkg(t, prog, r)
	out := map[string]string{}
	for _, m := range p.Derived {
		out[m.Type+"."+m.Method] = m.Src
	}
	return out, errorText(t, prog, r, dir, true)
}

// TestDeriveFile prints the methods that #Derive writes for the file named by DERIVE_FILE (a
// development aid).
func TestDeriveFile(t *testing.T) {
	path := os.Getenv("DERIVE_FILE")
	if path == "" {
		t.Skip("DERIVE_FILE not set")
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, errs := derived(t, string(src))
	fmt.Println(errs)
	var keys []string
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Println("//", k)
		fmt.Println(strings.TrimSpace(got[k]))
	}
}

// TestDeriveSource: the text of the methods for the shapes of fields that decide what the generators
// write (step 10.10). A mutation of a generator changes one of these.
func TestDeriveSource(t *testing.T) {
	got, errs := derived(t, `package main

import "time"

#Derive(String)
type Level int8

const (
	Low Level = iota - 1
	Mid
	High
	Top = High
)

#Derive(Equal, Clone)
type Flat struct {
	A, B int
	S    string
}

#Derive(Equal, Clone)
type Deep struct {
	F    Flat
	L    []Flat
	M    map[string][]int
	P    *Flat
	T    time.Time
	Any  any
	Skip int `+"`json:\"-\"`"+`
	_    int
}

#Derive(Equal, Clone)
type Names []string

#Derive(Equal, Clone)
type Pair[K comparable, V any] struct {
	Key K
	Val V
}
`)
	if errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	for _, tc := range []struct{ key, want string }{
		{"Flat.Equal", "return f == other"},
		{"Flat.Clone", "return f"},
		{"Names.Clone", "c = make([]string, len(n))"},
		{"Pair.Equal", "func (p Pair[K, V]) Equal(other Pair[K, V]) bool {"},
		{"Pair.Equal", "if any(p.Val) != any(other.Val) {"},
		{"Pair.Clone", "func (p Pair[K, V]) Clone() Pair[K, V] {"},
		{"Level.String", "func (l Level) String() string {"},
		{"Level.String", "case Low:"},
		{"Level.String", "case Mid:"},
		{"Level.String", "case High:"},
		{"Level.String", "n = -n"},
		{"Deep.Equal", "if d.F != other.F {"},                    // a flat derived struct: ==
		{"Deep.Equal", "for i := range d.L {"},                   // a slice
		{"Deep.Equal", "if d.L[i] != other.L[i] {"},              // of flat values
		{"Deep.Equal", "w, ok := other.M[k]"},                    // a map
		{"Deep.Equal", "if d.P != other.P {"},                    // a pointer
		{"Deep.Equal", "if (*d.P) != (*other.P) {"},              // follows the pointee
		{"Deep.Equal", "if !d.T.Equal(other.T) {"},               // time.Time has Equal
		{"Deep.Equal", "if d.Any != other.Any {"},                // an interface: ==
		{"Deep.Clone", "c.L = make([]Flat, len(d.L))"},           // a slice of values: copy
		{"Deep.Clone", "c.M = make(map[string][]int, len(d.M))"}, // a map
		{"Deep.Clone", "c1 := *d.P"},                             // a pointer: a new pointee
	} {
		if !strings.Contains(got[tc.key], tc.want) {
			t.Errorf("%s has no %q:\n%s", tc.key, tc.want, got[tc.key])
		}
	}
	if strings.Contains(got["Level.String"], "case Top:") {
		t.Errorf("a name for a value that another constant has:\n%s", got["Level.String"])
	}
	for _, key := range []string{"Deep.Equal", "Deep.Clone", "Flat.Equal"} {
		if strings.Contains(got[key], "reflect") || strings.Contains(got[key], "_") && strings.Contains(got[key], "d._") {
			t.Errorf("%s uses a blank field or a package:\n%s", key, got[key])
		}
	}
}

// TestDeriveErrors: what the check says (the same messages are in tests/errors).
func TestDeriveErrors(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"#Derive(Equal)\ntype T struct{ F func() }", "field F has a func type, which cannot be compared"},
		{"#Derive(String)\ntype T int", "no constants of type T are declared"},
		{"type In struct{ V []int }\n#Derive(Clone)\ntype T struct{ I In }", "field I has the type In, which refers to memory and has no Clone method"},
		{"type In struct{ V []int }\n#Derive(Equal)\ntype T struct{ I In }", "field I has the type In, which has no Equal method"},
		{"type A []B\ntype B []D\ntype D []B\n#Derive(Equal)\ntype T struct{ A A }", "the type B refers to itself"},
	} {
		_, errs := derived(t, "package main\n\n"+tc.src+"\n\nfunc main() {}\n")
		if !strings.Contains(errs, tc.want) {
			t.Errorf("%s: errors %q, want %q", tc.src, errs, tc.want)
		}
	}
}

// TestDeriveNoBodies: with Config.NoDerivedBodies (the editor, gocpp check) the generators still find
// their errors, and nothing else happens: no text is kept and the package is checked once.
func TestDeriveNoBodies(t *testing.T) {
	check := func(src string) (*Package, string) {
		dir := writeTree(t, map[string]string{"main.gp": src})
		prog := load.Load(load.Config{Dir: dir, Phase10Syntax: true, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}, "main.gp")
		cfg := fakeProbeConfig(LastPhase)
		cfg.NoDerivedBodies = true
		r := Check(prog, cfg)
		return mainPkg(t, prog, r), errorText(t, prog, r, dir, true)
	}
	p, errs := check("package main\n\n#Derive(Equal, Clone)\ntype T struct{ A []int }\n\nfunc main() { _ = T{}.Equal(T{}) }\n")
	if errs != "" || len(p.Derived) != 0 || p.checks != 1 {
		t.Errorf("errors %q, %d derived methods, %d checks", errs, len(p.Derived), p.checks)
	}
	_, errs = check("package main\n\n#Derive(Equal)\ntype T struct{ F func() }\n\nfunc main() {}\n")
	if !strings.Contains(errs, "field F has a func type") {
		t.Errorf("errors %q", errs)
	}
}
