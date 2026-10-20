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

func TestInstanceMethodPlacement(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		inline bool
	}{
		{"accessor", "return b.n", true},
		{"three lowered statements", "n := b.n; return n + 1", true},
		{"larger body", "a := b.n; c := a + 1; d := c + 1; return d", false},
		{"nested loop", "for i := 0; i < b.n; i++ { b.n += i }; return b.n", false},
		{"closure", "f := func() int { return b.n }; return f()", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := instProgram(t, "package main\n"+
				"type Box[T any] struct { n int; v T }\n"+
				"func (b Box[T]) Method() int { "+tt.body+" }\n"+
				"func main() { println(Box[int]{}.Method()) }\n")
			files, err := p.Lower()
			if err != nil {
				t.Fatal(err)
			}
			file := files[0]
			header := string(cppir.Print(file.Header, ""))
			var desc string
			if file.Desc != nil {
				desc = string(cppir.Print(file.Desc, ""))
			}
			definition := regexp.MustCompile(`(?m)^(inline )?int64_t gp_inst_\w+::Method\([^\n]*\) const \{`)
			if got := len(definition.FindAllString(header, -1)); (got == 1) != tt.inline {
				t.Fatalf("header has %d definitions, inline = %v:\n%s", got, tt.inline, header)
			}
			if got := len(definition.FindAllString(desc, -1)); (got == 0) != tt.inline {
				t.Fatalf("descriptor unit has %d definitions, inline = %v:\n%s", got, tt.inline, desc)
			}
			if !strings.Contains(header, "int64_t Method() const;") {
				t.Fatalf("header lost the method declaration:\n%s", header)
			}
			if !tt.inline && strings.Contains(definition.FindString(desc), "inline ") {
				t.Fatal("out-of-line definition is inline")
			}
		})
	}
}

func TestInstanceSmallLoopStaysInline(t *testing.T) {
	p := instProgram(t, `package main

type Box[T any] struct { n int; v T }
func (b *Box[T]) Step() {
	for i := 0; i < 8; i++ { b.n = b.n * 3 + 1 }
}
func main() { var b Box[int]; b.Step(); println(b.n) }
`)
	files, err := p.Lower()
	if err != nil {
		t.Fatal(err)
	}
	header := string(cppir.Print(files[0].Header, ""))
	definition := regexp.MustCompile(`(?m)^inline void gp_inst_\w+::Step\([^\n]*\) \{`)
	if !definition.MatchString(header) {
		t.Fatalf("small scalar loop is not inline:\n%s", header)
	}
	if files[0].Desc != nil {
		t.Fatal("small scalar loop unexpectedly needs a descriptor unit")
	}
}

func TestInstanceExpandedMethodOutOfLine(t *testing.T) {
	p := instProgram(t, `package main

type Box[T any] struct { items []T; n int }
func (b *Box[T]) Add(v T) { b.items = append(b.items, v); b.n++ }
func main() { var b Box[int]; b.Add(1) }
`)
	files, err := p.Lower()
	if err != nil {
		t.Fatal(err)
	}
	file := files[0]
	if file.Desc == nil {
		t.Fatal("expanded append body has no descriptor unit")
	}
	header := string(cppir.Print(file.Header, ""))
	desc := string(cppir.Print(file.Desc, ""))
	definition := regexp.MustCompile(`(?m)^void gp_inst_\w+::Add\([^\n]*\) \{`)
	if !definition.MatchString(desc) || definition.MatchString(header) {
		t.Fatalf("expanded two-statement body is not out of line:\nheader:\n%s\ndesc:\n%s", header, desc)
	}
}

func TestInstanceMethodDefinitionOnceAcrossFiles(t *testing.T) {
	fset, pkg, _ := checkFiles(t, map[string]string{
		"main.gp": `package main

type Box[T any] struct { n int; v T }
func (b Box[T]) Method() int {
	a := b.n
	c := a + 1
	d := c + 1
	return d
}
func main() { println(Box[int]{}.Method(), other()) }
`,
		"other.gp": `package main
func other() int { return Box[int]{}.Method() }
`,
	})
	p := NewProgram(fset, []PackageInput{{Pkg: pkg}}, Options{})
	files, err := p.Lower()
	if err != nil {
		t.Fatal(err)
	}
	if len(files[0].Units) != 2 {
		t.Fatalf("%d source units, want 2", len(files[0].Units))
	}
	definitions := 0
	for _, it := range p.instTypes {
		if len(it.bodies) == 0 {
			continue
		}
		definitions++
		fn := it.bodies[0].(*cppir.Func)
		if fn.Pos.Line != 4 || fn.Body.End.Line != 9 {
			t.Fatalf("moved method source span = %v..%v, want lines 4..9", fn.Pos, fn.Body.End)
		}
		if filepath.Base(fn.Pos.File) != "main.gp" || fn.Pos.File != fn.Body.End.File {
			t.Fatalf("moved method source files = %q..%q", fn.Pos.File, fn.Body.End.File)
		}
		definition := fn.Name + "("
		desc := string(cppir.Print(files[0].Desc, ""))
		mapped := make(map[int]bool)
		for _, stmt := range fn.Body.Stmts {
			switch stmt := stmt.(type) {
			case *cppir.Line:
				mapped[stmt.Pos.Line] = true
			case *cppir.VarDecl:
				mapped[stmt.Pos.Line] = true
			case *cppir.Return:
				mapped[stmt.Pos.Line] = true
			}
		}
		for _, line := range []int{5, 6, 7, 8} {
			if !mapped[line] {
				t.Errorf("moved method lost source line %d", line)
			}
		}
		count := strings.Count(desc, definition)
		if count != 1 {
			t.Fatalf("descriptor unit has %d definitions of %s", count, fn.Name)
		}
		for _, unit := range files[0].Units {
			if strings.Contains(string(cppir.Print(unit.File, "")), definition) {
				t.Fatalf("%s also defines %s", unit.Name, fn.Name)
			}
		}
	}
	if definitions != 1 {
		t.Fatalf("%d out-of-line methods, want 1", definitions)
	}
}

func TestInstanceMethodBodyUsesImportingPackage(t *testing.T) {
	p := borrowForeignProgram(t, map[string]string{
		"gocpp.mod": "module example.com/placement\n",
		"a/a.gp": `package a

type Box[T any] struct { Value T }
func (b Box[T]) Copy() T {
	x := b.Value
	y := x
	z := y
	return z
}
`,
		"b/b.gp": `package b
import "example.com/placement/a"
type Local struct { N int }
func New() a.Box[Local] { return a.Box[Local]{Local{7}} }
func Copy() Local { return New().Copy() }
`,
		"main.gp": `package main
import "example.com/placement/b"
func main() { println(b.Copy().N, b.New().Copy().N) }
`,
	})
	files, err := p.Lower()
	if err != nil {
		t.Fatal(err)
	}
	definitions := 0
	for _, file := range files {
		if file.ImportPath == "example.com/placement/a" {
			if strings.Contains(string(cppir.Print(file.Header, "")), "Local") {
				t.Fatal("declaring package's header names its importer's type")
			}
		}
		if file.Desc == nil {
			continue
		}
		text := string(cppir.Print(file.Desc, ""))
		count := len(regexp.MustCompile(`gp_inst_\w+::Copy\(\) const \{`).FindAllString(text, -1))
		definitions += count
		if count > 0 && file.ImportPath != "example.com/placement/b" {
			t.Fatalf("method body is in %s, want the first using package b", file.ImportPath)
		}
	}
	if definitions != 1 {
		t.Fatalf("%d imported method definitions, want 1", definitions)
	}
}

func TestInstanceMethodBodyMacroProtection(t *testing.T) {
	p := borrowForeignProgram(t, map[string]string{
		"gocpp.mod": "module example.com/methodmacro\n",
		"lib/lib.gp": `package lib

type Box[T any] struct { Value T }
func (b Box[T]) Copy() T {
	local := b.Value
	next := local
	result := next
	return result
}
`,
		"main.gp": `package main
import "example.com/methodmacro/lib"
func main() { println(lib.Box[int]{7}.Copy()) }
`,
	})
	// The importer's native includes may introduce a macro whose name only occurs in the
	// declaring package's method body, not in the instance's header declarations.
	p.gens[len(p.gens)-1].macros = map[string]bool{"local": true}
	files, err := p.Lower()
	if err != nil {
		t.Fatal(err)
	}
	file := files[len(files)-1]
	if file.Desc == nil {
		t.Fatal("no descriptor unit for imported method")
	}
	text := string(cppir.Print(file.Desc, ""))
	if !strings.Contains(text, "#undef local\n") {
		t.Fatalf("moved method is not protected from the importer's macro:\n%s", text)
	}
}

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
