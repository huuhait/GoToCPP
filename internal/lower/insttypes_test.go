package lower

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppir"
)

// instProgram returns the Program of the main package src (the split layout, which instances of
// generic types need), checked but not lowered.
func instProgram(t *testing.T, src string) *Program {
	t.Helper()
	gp := filepath.Join(t.TempDir(), "main.gp")
	if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	fset, pkg := checkFile(t, gp)
	return NewProgram(fset, []PackageInput{{Pkg: pkg}}, Options{})
}

// lowerInst lowers the program p and returns the text of the files of the main package.
func lowerInst(t *testing.T, p *Program) (string, error) {
	t.Helper()
	files, err := p.Lower()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	f := files[len(files)-1]
	b.Write(cppir.Print(f.Header, ""))
	for _, u := range f.Units {
		b.Write(cppir.Print(u.File, ""))
	}
	if f.Desc != nil {
		b.Write(cppir.Print(f.Desc, ""))
	}
	b.Write(cppir.Print(f.Init, ""))
	return regexp.MustCompile(`#line \d+ "[^"]*"`).ReplaceAllString(b.String(), "#line"), nil
}

const instSrc = `package main

import "fmt"

type Pair[K, V any] struct {
	Key K
	Val V
}

func (p Pair[K, V]) Swap() Pair[V, K] { return Pair[V, K]{p.Val, p.Key} }

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

type Point struct{ X, Y int }

func main() {
	a := Pair[string, []int]{"a", nil}
	b := Pair[Point, *Stack[int]]{}
	c := Pair[int, int]{1, 2}.Swap()
	var s Stack[Pair[int, map[string]bool]]
	s.Push(Pair[int, map[string]bool]{})
	fmt.Printf("%T %T %T %T\n", a, b, c, s)
}
`

// TestInstanceTypeNames checks the name that an instance's descriptor has, which is what %T
// prints: Go's text, with the type arguments' package paths and no space after a comma.
func TestInstanceTypeNames(t *testing.T) {
	out, err := lowerInst(t, instProgram(t, instSrc))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"main.Pair[string,[]int]"`,
		`"main.Pair[main.Point,*main.Stack[int]]"`,
		`"main.Pair[int,int]"`,
		`"main.Stack[main.Pair[int,map[string]bool]]"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no descriptor named %s in:\n%s", want, out)
		}
	}
	if n := len(regexp.MustCompile(`(?m)^struct gp_inst_\w+ \{`).FindAllString(out, -1)); n != 9 {
		t.Errorf("%d instance structs, want 9: the 4 Pairs and 2 Stacks written, and the 3 Pairs that Swap returns (Pair[int,int] swaps to itself): the methods of an instance are lowered with it", n)
	}
}

// TestInstanceTypesDeterministic lowers one program several times, each with a program of its own:
// the same names, order and text.
func TestInstanceTypesDeterministic(t *testing.T) {
	first, err := lowerInst(t, instProgram(t, instSrc))
	if err != nil {
		t.Fatal(err)
	}
	for range 4 {
		again, err := lowerInst(t, instProgram(t, instSrc))
		if err != nil {
			t.Fatal(err)
		}
		if again != first {
			t.Fatal("two lowerings of one program differ")
		}
	}
}

// TestInstanceNameClash checks that an instance whose name is taken by another one gets more hex
// digits of its hash: the names are distinct, and the digits are 8, 10 or 12.
func TestInstanceNameClash(t *testing.T) {
	p := instProgram(t, instSrc)
	if _, err := lowerInst(t, p); err != nil {
		t.Fatal(err)
	}
	var key string
	for k := range p.instTypes {
		if strings.HasPrefix(k, "main.Stack[int]") {
			key = k
		}
	}
	if key == "" {
		t.Fatalf("no instance Stack[int] in %v", p.instTypes)
	}
	sum := sha256.Sum256([]byte(key))
	name8 := "gp_inst_" + hex.EncodeToString(sum[:4]) + "_Stack"
	name10 := "gp_inst_" + hex.EncodeToString(sum[:5]) + "_Stack"
	name12 := "gp_inst_" + hex.EncodeToString(sum[:6]) + "_Stack"
	if p.instTypeNames[name8] != key {
		t.Fatalf("the name of Stack[int] is not %s: %v", name8, p.instTypeNames)
	}
	// The same program, with the 8-digit name taken, and then the 10-digit name too.
	for _, taken := range [][]string{{name8}, {name8, name10}} {
		q := instProgram(t, instSrc)
		q.instTypeNames = map[string]string{}
		q.instTypes = map[string]*instType{}
		for _, n := range taken {
			q.instTypeNames[n] = "other instance"
		}
		out, err := lowerInst(t, q)
		if err != nil {
			t.Fatal(err)
		}
		want := name10
		if len(taken) == 2 {
			want = name12
		}
		if q.instTypeNames[want] != key || !strings.Contains(out, "struct "+want+" {") {
			t.Errorf("with %v taken, Stack[int] is not %s", taken, want)
		}
		for _, n := range taken {
			if strings.Contains(out, "struct "+n+" {") {
				t.Errorf("the taken name %s is used", n)
			}
		}
	}
}

// TestInstanceNeedsSplit checks that an instance of a generic type in the unit layout is a clear
// user error (the instance is shared text, which only the split layout has).
func TestInstanceNeedsSplit(t *testing.T) {
	_, err := lowerGeneric(t, `package main

type Box[T any] struct{ v T }

func main() { _ = Box[int]{} }
`)
	if err == nil || !strings.Contains(err.Error(), "generic types need the split layout") {
		t.Errorf("error %v", err)
	}
}

var _ = check.LastPhase

// TestInstanceForwarders checks that an instance that embeds another type has the members that
// forward the promoted methods: its descriptor has a method table that calls them, and an instance
// is one text for all the packages that mention it, so what it has can't depend on which asks.
func TestInstanceForwarders(t *testing.T) {
	out, err := lowerInst(t, instProgram(t, `package main

type Base[T any] struct{ v T }

func (b *Base[T]) Get() T { return b.v }

type Outer[T any] struct {
	Base[T]
	n int
}

func main() {
	var o Outer[int]
	println(o.n, o.Get())
}
`))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"static int64_t Get(gp::ptr<gp_inst_ffda826f_Outer> gp_p);",
		"inline int64_t gp_inst_ffda826f_Outer::Get(gp::ptr<gp_inst_ffda826f_Outer> gp_p) {",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in:\n%s", want, out)
		}
	}
}

// TestLocalTypesDeterministic lowers a program with types declared in generic functions (several, in
// the same function, with the same name in two functions) several times, each with a program of its
// own: the same names and text, and one struct for each (function, type, type arguments).
func TestLocalTypesDeterministic(t *testing.T) {
	const src = `package main

func F[T any](v T) any {
	type local struct{ x T }
	type other struct{ l local }
	return other{local{v}}
}

func G[T any](v T) any {
	type local struct{ x T }
	return local{v}
}

func main() { _, _, _ = F(1), F("s"), G(1) }
`
	first, err := lowerInst(t, instProgram(t, src))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"main.local[int]"`, `"main.local[string]"`, `"main.other[int]"`, `"main.other[string]"`} {
		if !strings.Contains(first, want) {
			t.Errorf("no descriptor named %s in:\n%s", want, first)
		}
	}
	if n := len(regexp.MustCompile(`(?m)^struct gp_inst_\w+ \{`).FindAllString(first, -1)); n != 5 {
		t.Errorf("%d instance structs, want 5: local and other of F[int] and F[string], local of G[int]", n)
	}
	for range 4 {
		again, err := lowerInst(t, instProgram(t, src))
		if err != nil {
			t.Fatal(err)
		}
		if again != first {
			t.Fatal("two lowerings of one program differ")
		}
	}
}
