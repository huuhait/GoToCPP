package diff

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/tools/difftest/gen"
)

func TestSplitOutput(t *testing.T) {
	out := "noise\n#### 3\n== a\nx\n#### 9\n== b\n#### 12\n"
	ids, parts := SplitOutput(out)
	if fmt.Sprint(ids) != "[3 9 12]" {
		t.Fatalf("ids = %v", ids)
	}
	if parts[3] != "== a\nx\n" || parts[9] != "== b\n" || parts[12] != "" {
		t.Fatalf("parts = %q", parts)
	}
}

func TestFirstDiff(t *testing.T) {
	sig, line := FirstDiff("a\nb\nc\n", "a\nB\nc\n")
	if sig != "- b\n+ B" || line != 2 {
		t.Fatalf("got %q at %d", sig, line)
	}
	sig, _ = FirstDiff("a\n", "a\nb\n")
	if !strings.Contains(sig, "b") {
		t.Fatalf("a longer output must differ: %q", sig)
	}
	if sig, _ := FirstDiff("same", "same"); sig != "" {
		t.Fatalf("equal outputs: %q", sig)
	}
}

func TestMaskStderr(t *testing.T) {
	a := MaskStderr("panic: runtime error: invalid memory address at 0xc000012345 [recovered]\ngoroutine 1 [running]:\n")
	b := MaskStderr("panic: runtime error: invalid memory address at 0x7ffd1234\n")
	if a != b {
		t.Fatalf("masks differ: %q vs %q", a, b)
	}
	if MaskStderr("panic: a") == MaskStderr("panic: b") {
		t.Fatal("different panics must differ")
	}
}

func TestCompareBundle(t *testing.T) {
	ref := "#### 1\n== p\nA\n#### 2\n== p\nB\n#### 3\n== p\nC\n#### 4\n== p\nD\n"
	_, rout := SplitOutput(ref)
	rparts := RefOut{Out: rout}
	ids := []int{1, 2, 3, 4}
	t.Run("equal", func(t *testing.T) {
		mm, un := CompareBundle(ids, rparts, Result{BuildOK: true, Stdout: ref})
		if len(mm) != 0 || len(un) != 0 {
			t.Fatalf("%v %v", mm, un)
		}
	})
	t.Run("one program differs", func(t *testing.T) {
		got := strings.Replace(ref, "B\n", "b\n", 1)
		mm, _ := CompareBundle(ids, rparts, Result{BuildOK: true, Stdout: got})
		if len(mm) != 1 || mm[0].Prog != 2 || mm[0].Kind != KindOutput || mm[0].Sig != "- B\n+ b" {
			t.Fatalf("%+v", mm)
		}
	})
	t.Run("crash in program 2 leaves 3 and 4 to run again", func(t *testing.T) {
		got := "#### 1\n== p\nA\n#### 2\n== p\nB\n"
		mm, un := CompareBundle(ids, rparts, Result{BuildOK: true, Stdout: got, Exit: 139, Stderr: "Segmentation fault\n"})
		if len(mm) != 1 || mm[0].Prog != 2 || mm[0].Kind != KindCrash {
			t.Fatalf("mismatches %+v", mm)
		}
		if fmt.Sprint(un) != "[3 4]" {
			t.Fatalf("unrun = %v", un)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		got := "#### 1\n== p\nA\n#### 2\n== p\n"
		mm, un := CompareBundle(ids, rparts, Result{BuildOK: true, Stdout: got, TimedOut: true, Exit: 137})
		if len(mm) != 1 || mm[0].Kind != KindTimeout || mm[0].Prog != 2 || len(un) != 2 {
			t.Fatalf("%+v %v", mm, un)
		}
	})
	t.Run("sanitizer report", func(t *testing.T) {
		got := "#### 1\n== p\nA\n#### 2\n"
		mm, _ := CompareBundle(ids, rparts, Result{BuildOK: true, Stdout: got, Exit: 1,
			Stderr: "==1==ERROR: AddressSanitizer: heap-use-after-free on address 0x1\n    #0 0x1 in f main.cpp:3\n"})
		if len(mm) != 1 || mm[0].Kind != KindSanitizer || !strings.Contains(mm[0].Sig, "AddressSanitizer") {
			t.Fatalf("%+v", mm)
		}
	})
	t.Run("a leak report at the end names the program in its frames", func(t *testing.T) {
		got := ref
		mm, _ := CompareBundle(ids, rparts, Result{BuildOK: true, Stdout: got,
			Stderr: "=====\n==1==ERROR: LeakSanitizer: detected memory leaks\n\nIndirect leak of 40 byte(s)\n    #3 0x1 in main::p2_s7() x.cpp:9\n    #4 0x1 in p4_s1() x.cpp:3\n"})
		if len(mm) != 1 || mm[0].Prog != 2 || mm[0].Kind != KindSanitizer || !strings.Contains(mm[0].Sig, "LeakSanitizer") {
			t.Fatalf("%+v", mm)
		}
	})
	t.Run("a program the subject never printed without a crash", func(t *testing.T) {
		got := "#### 1\n== p\nA\n#### 3\n== p\nC\n#### 4\n== p\nD\n"
		mm, un := CompareBundle(ids, rparts, Result{BuildOK: true, Stdout: got})
		if len(mm) != 1 || mm[0].Prog != 2 || len(un) != 0 {
			t.Fatalf("%+v %v", mm, un)
		}
	})
}

func TestIsSanitizer(t *testing.T) {
	for s, want := range map[string]bool{
		"==1==ERROR: AddressSanitizer: heap-use-after-free\n":                                                                       true,
		"x.hpp:10:3: runtime error: signed integer overflow: 1 + 2 cannot be represented\nSUMMARY: UndefinedBehaviorSanitizer: x\n": true,
		"runtime/include/gocpp/slice.hpp:30:12: runtime error: load of null pointer of type 'int'\n":                                true,
		"panic: runtime error: index out of range [3] with length 2\n\ngoroutine 1 [running]:\n":                                    false,
		"panic: runtime error: integer divide by zero\n":                                                                            false,
		"": false,
	} {
		if got := IsSanitizer(s); got != want {
			t.Errorf("IsSanitizer(%q) = %v", s, got)
		}
	}
}

func TestCompareBundleStderr(t *testing.T) {
	out := "#### 1\n== p\nA\n#### 2\n== p\nB\n"
	_, rout := SplitOutput(out)
	rerr := "#### 1\n== p\nhello 1\n#### 2\n== p\n"
	_, rerrs := SplitOutput(rerr)
	ref := RefOut{Out: rout, Err: rerrs}
	ids := []int{1, 2}
	if mm, _ := CompareBundle(ids, ref, Result{BuildOK: true, Stdout: out, Stderr: rerr}); len(mm) != 0 {
		t.Fatalf("%v", mm)
	}
	got := "#### 1\n== p\nhello 2\n#### 2\n== p\n"
	mm, _ := CompareBundle(ids, ref, Result{BuildOK: true, Stdout: out, Stderr: got})
	if len(mm) != 1 || mm[0].Prog != 1 || mm[0].Kind != KindStderr || mm[0].Sig != "- hello 1\n+ hello 2" || mm[0].Scenario != "p" {
		t.Fatalf("%+v", mm)
	}
	// a sanitizer's text at the end of stderr is the sanitizer's business, not a stderr difference
	if mm, _ := CompareBundle(ids, ref, Result{BuildOK: true, Stdout: out, Stderr: got + "==1==ERROR: LeakSanitizer: detected memory leaks\n"}); len(mm) != 1 || mm[0].Kind != KindSanitizer {
		t.Fatalf("%+v", mm)
	}
	// a program missing from stderr when the reference printed it
	if mm, _ := CompareBundle(ids, ref, Result{BuildOK: true, Stdout: out, Stderr: "#### 2\n== p\n"}); len(mm) != 1 || mm[0].Kind != KindStderr {
		t.Fatalf("%+v", mm)
	}
}

func TestStandaloneStderrBeforePanic(t *testing.T) {
	ref := Result{BuildOK: true, Exit: 2, Stderr: "p 1 true\npanic: boom\n\ngoroutine 1 [running]:\nmain.f()\n"}
	same := Result{BuildOK: true, Exit: 2, Stderr: "p 1 true\npanic: boom [recovered]\nstack"}
	if mm := CompareStandalone(1, ref, same); len(mm) != 0 {
		t.Fatalf("%v", mm)
	}
	bad := Result{BuildOK: true, Exit: 2, Stderr: "p 2 true\npanic: boom\n"}
	mm := CompareStandalone(1, ref, bad)
	if len(mm) != 1 || mm[0].Kind != KindStderr || mm[0].Sig != "- p 1 true\n+ p 2 true" {
		t.Fatalf("a difference in what println printed before the panic must be found: %+v", mm)
	}
	// a program that ends normally: only println's lines
	if mm := CompareStandalone(1, Result{Stderr: "a\nb\n"}, Result{Stderr: "a\nc\n"}); len(mm) != 1 {
		t.Fatalf("%v", mm)
	}
}

func TestScenarioAndOutputMismatch(t *testing.T) {
	want := "== ints\n1\n2\n== strings\nx\ny\n"
	m := outputMismatch(7, want, "== ints\n1\n2\n== strings\nx\nz\n")
	if m.Scenario != "strings" || m.Sig != "- y\n+ z" || m.WantFull != want {
		t.Fatalf("%+v", m)
	}
	a := clusterKey(outputMismatch(1, "== ints\n5\n", "== ints\n6\n"))
	b := clusterKey(outputMismatch(2, "== maps\n5\n", "== maps\n6\n"))
	c := clusterKey(outputMismatch(3, "== ints\n7\n", "== ints\n8\n"))
	if a == b || a != c {
		t.Fatalf("numeric-only differences of two scenarios share a cluster, or of one do not: %s %s %s", a, b, c)
	}
}

func TestSanitizerSigHasTheStack(t *testing.T) {
	leak := func(frame string) string {
		return "=====\n==1==ERROR: LeakSanitizer: detected memory leaks\n\nDirect leak of 40 byte(s)\n    #0 0x1 in operator new(unsigned long) a.so+0x1\n    #1 0x2 in " + frame + " x.cpp:3\n"
	}
	a, b := sanitizerSig(leak("main::p12_s7()")), sanitizerSig(leak("main::p99_s7()"))
	c := sanitizerSig(leak("gp::slice<long>::grow()"))
	if a != b {
		t.Fatalf("the numbers of generated names must not make two reports different:\n%s\n%s", a, b)
	}
	if a == c || !strings.Contains(a, "LeakSanitizer") {
		t.Fatalf("different stacks must differ: %q %q", a, c)
	}
}

func TestCompareStandalone(t *testing.T) {
	ref := Result{BuildOK: true, Stdout: "x\n", Exit: 2, Stderr: "panic: boom\n\ngoroutine 1 [running]:\n..."}
	same := Result{BuildOK: true, Stdout: "x\n", Exit: 2, Stderr: "panic: boom [recovered]\nstack"}
	if mm := CompareStandalone(5, ref, same); len(mm) != 0 {
		t.Fatalf("%v", mm)
	}
	bad := Result{BuildOK: true, Stdout: "x\n", Exit: 1, Stderr: "panic: bam\n"}
	mm := CompareStandalone(5, ref, bad)
	kinds := map[Kind]bool{}
	for _, m := range mm {
		kinds[m.Kind] = true
	}
	if !kinds[KindExit] || !kinds[KindStderr] || kinds[KindOutput] {
		t.Fatalf("%v", mm)
	}
}

func TestProgOfLine(t *testing.T) {
	src := "package main\n\n// ---- program 4 (a) ----\n\nfunc x() {}\n// ---- program 9 (b) ----\nfunc y() {}\nfunc main() {\n}\n"
	for _, c := range []struct{ line, want int }{{1, -1}, {3, 4}, {5, 4}, {7, 9}, {8, -1}} {
		if got := ProgOfLine(src, c.line); got != c.want {
			t.Errorf("line %d: program %d, want %d", c.line, got, c.want)
		}
	}
	ids := ProgsOfErrors(src, "./main.go:5:2: oops\nmain.gp:7:1: again\nmain.go:5:9: dup\n")
	if fmt.Sprint(ids) != "[4 9]" {
		t.Fatalf("ids %v", ids)
	}
}

func TestMaskSig(t *testing.T) {
	a := FirstError("# command-line-arguments\n./main.go:12:3: undefined: p17_foo3 (x 99)\n")
	b := FirstError("./main.go:880:1: undefined: p9_foo7 (x 5)")
	if a != b || a == "" {
		t.Fatalf("%q vs %q", a, b)
	}
}

const knownText = `
# a comment
name: D22
kind: output
sig: (?m)^\+ runtime error: index
reason: which of two panics is reported
doc: docs/spec/language.md#d22

name: any-build
kind: build
sig: unsupported
src: Foo\(
reason: not built yet
doc: plans/x.md
`

func TestKnown(t *testing.T) {
	k, err := ParseKnown(strings.NewReader(knownText))
	if err != nil {
		t.Fatal(err)
	}
	if len(k.Entries) != 2 {
		t.Fatalf("%d entries", len(k.Entries))
	}
	if e := k.Match(Mismatch{Kind: KindOutput, Sig: "- a\n+ runtime error: index"}, ""); e == nil || e.Name != "D22" {
		t.Fatalf("D22 not matched: %v", e)
	}
	if e := k.Match(Mismatch{Kind: KindExit, Sig: "+ runtime error: index"}, ""); e != nil {
		t.Fatal("kind must match")
	}
	if e := k.Match(Mismatch{Kind: KindBuild, Sig: "unsupported thing"}, "x Bar()"); e != nil {
		t.Fatal("src must match")
	}
	if e := k.Match(Mismatch{Kind: KindBuild, Sig: "unsupported thing"}, "x Foo()"); e == nil {
		t.Fatal("src matches")
	}
	for _, bad := range []string{"name: x\nsig: (\nreason: r\ndoc: d\n", "name: x\nreason: r\ndoc: d\n", "foo: bar\n"} {
		if _, err := ParseKnown(strings.NewReader(bad)); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestKnownFileParses(t *testing.T) {
	k, err := LoadKnown("../known.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range k.Entries {
		if _, err := os.Stat(filepath.Join("..", "..", "..", strings.SplitN(e.Doc, "#", 2)[0])); err != nil && !strings.HasPrefix(e.Doc, "http") {
			t.Errorf("known entry %s: doc %s does not exist", e.Name, e.Doc)
		}
	}
}

// ---- reducer ----

func progWith(id int, src string) *gen.Program {
	return &gen.Program{ID: id, Src: src, Entry: fmt.Sprintf("prog_%d", id), Imports: []string{"fmt"}}
}

const reducibleSrc = `// scenario: a
func p7_s1() {
	x := 3
	y := x * 2
	if y > 4 {
		fmt.Println("big", y)
	}
	for i := 0; i < 3; i++ {
		fmt.Println(i, 12345)
	}
	z := "hello"
	_ = z
}

// scenario: b
func p7_s2() {
	fmt.Println("this one has the bug")
	BUGGY(7)
	w := 5
	_ = w
}

func BUGGY(n int) { fmt.Println("buggy", n) }

type p7_T struct{ A, B int }

func (t p7_T) Sum() int { return t.A + t.B }

func prog_7() {
	try("a", p7_s1)
	try("b", p7_s2)
}
`

func buggySub() *fakeBackend {
	return fakeCpp(func(src string, id int) string {
		if strings.Contains(src, "BUGGY(7)") {
			return "wrong"
		}
		return ""
	})
}

func TestReducer(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks with the source importer")
	}
	p := progWith(7, reducibleSrc)
	r := &Reducer{Ref: fakeGo(), Sub: buggySub(), Budget: 100, Mode: gen.Recovered}
	m := Mismatch{Prog: 7, Kind: KindOutput, Sig: "- result 49\n+ wrong"}
	min, mm, builds, err := r.Reduce(context.Background(), p, m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(min, "BUGGY(7)") {
		t.Fatalf("the reduced program lost the bug:\n%s", min)
	}
	if len(min) > len(reducibleSrc)/2 {
		t.Fatalf("not reduced enough (%d of %d bytes):\n%s", len(min), len(reducibleSrc), min)
	}
	for _, gone := range []string{"hello", "12345", "p7_T", "Sum"} {
		if strings.Contains(min, gone) {
			t.Errorf("%q should be gone:\n%s", gone, min)
		}
	}
	if mm.Kind != KindOutput || builds < 2 || builds > 100 {
		t.Fatalf("kind %s builds %d", mm.Kind, builds)
	}
	if e := typeCheckErr(pkgPrefix + min); e != "" && !strings.Contains(e, "declared and not used") {
		t.Logf("note: reduced program: %s", e)
	}
}

func TestReducerBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks with the source importer")
	}
	p := progWith(7, reducibleSrc)
	sub := buggySub()
	r := &Reducer{Ref: fakeGo(), Sub: sub, Budget: 3, Mode: gen.Recovered}
	min, _, builds, err := r.Reduce(context.Background(), p, Mismatch{Prog: 7, Kind: KindOutput})
	if err != nil {
		t.Fatal(err)
	}
	if builds > 3 || sub.runs.Load() > 3 {
		t.Fatalf("the budget of 3 builds was exceeded: %d builds, %d runs", builds, sub.runs.Load())
	}
	if !strings.Contains(min, "BUGGY(7)") {
		t.Fatal("a stopped reduction must still show the bug")
	}
}

func TestReducerNotReproducible(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks with the source importer")
	}
	p := progWith(7, strings.ReplaceAll(reducibleSrc, "BUGGY(7)", "fmt.Println(7)"))
	r := &Reducer{Ref: fakeGo(), Sub: buggySub(), Budget: 20, Mode: gen.Recovered}
	min, _, builds, err := r.Reduce(context.Background(), p, Mismatch{Prog: 7, Kind: KindOutput})
	if err != nil || min != "" || builds != 1 {
		t.Fatalf("min %q builds %d err %v", min, builds, err)
	}
}

func TestReducerKeepsBuildErrorSignature(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks with the source importer")
	}
	src := strings.ReplaceAll(reducibleSrc, "BUGGY(7)", "REJECTED(7)") + "\nfunc REJECTED(n int) {}\n"
	r := &Reducer{Ref: fakeGo(), Sub: fakeCpp(func(string, int) string { return "" }), Budget: 60, Mode: gen.Recovered}
	min, mm, _, err := r.Reduce(context.Background(), progWith(7, src), Mismatch{Prog: 7, Kind: KindBuild, Sig: FirstError("main.gp:7:2: unsupported construct\n")})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(min, "REJECTED(7)") || mm.Kind != KindBuild {
		t.Fatalf("%s\n%+v", min, mm)
	}
}

func TestApplyEdits(t *testing.T) {
	src := "0123456789"
	got := apply(src, []edit{{2, 4, ""}, {3, 3, "x"}, {6, 8, "AB"}})
	if got != "01"+"45"+"AB"+"89" {
		t.Fatalf("got %q", got)
	}
}

func TestEditPasses(t *testing.T) {
	text := pkgPrefix + `func f() {
	a := 1
	if a > 0 {
		fmt.Println("x", 5)
	} else {
		a = 2
	}
	for i := 0; i < 2; i++ {
		a++
	}
	fmt.Println(a, 3.25, "s")
}

func prog_1() {}

type T struct{}
`
	count := func(f func(string) []edit) int { return len(f(text)) }
	if n := count(stmtEdits); n < 7 {
		t.Errorf("stmtEdits: %d", n)
	}
	if n := count(entryEdits); n != 0 { // prog_1 has no statements
		t.Errorf("entryEdits: %d", n)
	}
	if n := count(declEdits); n != 2 { // f and T, not prog_1
		t.Errorf("declEdits: %d", n)
	}
	if n := count(unwrapEdits); n != 3 { // if (then, else), for
		t.Errorf("unwrapEdits: %d", n)
	}
	if n := count(literalEdits); n < 3 {
		t.Errorf("literalEdits: %d", n)
	}
	if n := count(argEdits); n < 2 {
		t.Errorf("argEdits: %d", n)
	}
	for name, f := range map[string]func(string) []edit{"entry": entryEdits, "stmt": stmtEdits, "decl": declEdits, "unwrap": unwrapEdits, "lit": literalEdits, "arg": argEdits} {
		for _, e := range f(text) {
			out := apply(text, []edit{e})
			if _, err := parseOnly(out); err != nil {
				t.Errorf("%s edit %+v does not parse: %v\n%s", name, e, err, out)
			}
		}
	}
}

func TestSameModuloAddresses(t *testing.T) {
	if !sameModuloAddresses("a func() 0x49c060 b", "a func() 0x555555564bf6 b") {
		t.Fatal("addresses must be masked")
	}
	if !sameModuloAddresses("&{0xc000012345 1}", "&{0x7ffff7a12345 1}") {
		t.Fatal("heap pointers must be masked")
	}
	if sameModuloAddresses("x 0x49c060", "y 0x49c060") {
		t.Fatal("other text differs")
	}
	// a wrong digit of a long hexadecimal value is a difference, not an address
	if sameModuloAddresses("%#x 0xdeadbeef", "%#x 0xdeadbeee") {
		t.Fatal("0xdeadbeef and 0xdeadbeee must differ")
	}
	if sameModuloAddresses("0x123456789abc", "0x123456789abd") {
		t.Fatal("a value that is not in an address range must differ")
	}
	// a real difference behind an address is still found
	if sameModuloAddresses("0xc000012345 a", "0x555555564bf6 b") {
		t.Fatal("the rest of the output must be equal")
	}
	if sameModuloAddresses("0xff", "0xfe") {
		t.Fatal("short hex numbers are values, not addresses")
	}
}
