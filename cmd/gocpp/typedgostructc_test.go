package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// A Go struct with a C/C++ field is a Go type: the literal of its elements next to a C/C++ map key is
// typed and checked like any other (only the field is invalid for go/types), and it builds and runs. An
// elided literal of a C/C++ type, or of an instance of a generic type with a C/C++ type argument, has no
// type that the lowering can write, so check and build both refuse it, with the words of the limit.

const goStructCMain = `package main

import (
	"fmt"

	"./hresult.hpp"
)

type WS struct {
	S hresult.Surface
	N int
}

type HS struct {
	W WS
	N int
}

func main() {
	k := hresult.nt_result(1)
	m1 := map[any]WS{k: {N: 1}}
	m2 := map[any]*WS{k: {N: 2}}
	m3 := map[any][]WS{k: {{N: 3}}}
	m4 := map[any]map[string]WS{k: {"a": {N: 4}}}
	m5 := map[any][2]WS{k: {{N: 5}, {N: 6}}}
	m6 := map[any]HS{k: {W: WS{N: 7}, N: 8}}
	m7 := map[any]*HS{k: {N: 9}}
	m8 := map[any]map[any]WS{k: {k: {N: 10}}}
	n := 0
	for _, v := range m1 {
		n += v.N
	}
	for _, v := range m2 {
		n += v.N
	}
	for _, v := range m3 {
		n += v[0].N
	}
	for _, v := range m4 {
		n += v["a"].N
	}
	for _, v := range m5 {
		n += v[0].N + v[1].N
	}
	for _, v := range m6 {
		n += v.W.N + v.N
	}
	for _, v := range m7 {
		n += v.N
	}
	for _, m := range m8 {
		for _, v := range m {
			n += v.N
		}
	}
	fmt.Println(n)
}
`

// goStructCBad is a program with one map literal (BODY) that is not valid.
const goStructCBad = `package main

import (
	"fmt"

	"./hresult.hpp"
)

type WS struct {
	S hresult.Surface
	N int
}

type HS struct {
	W WS
	N int
}

type Gen[V any] struct {
	V V
	N int
}

func main() {
	k := hresult.nt_result(1)
	BODY
	fmt.Println(len(m))
}
`

func goStructCFiles(t *testing.T, main string) map[string]string {
	t.Helper()
	header, err := os.ReadFile(filepath.Join(typedDir, "hresult.hpp"))
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{"gocpp.mod": "module example.com/m\n", "hresult.hpp": string(header), "main.gp": main}
}

func TestTypedGoStructWithACFieldBuildsAndRuns(t *testing.T) {
	for _, ctype := range []string{"Surface", "NTSTATUS"} {
		src := strings.ReplaceAll(goStructCMain, "hresult.Surface", "hresult."+ctype)
		t.Run(ctype, func(t *testing.T) {
			forEachCXX(t, func(t *testing.T, c testCXX) {
				typedEnv(t)
				t.Setenv("GOCPP_INTEROP", "typed")
				inDir(t, goStructCFiles(t, src))
				needClang(t)
				if r := gocppRun("", false, "check", "."); r.code != 0 || r.stdout != "" {
					t.Errorf("check: exit %d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
				}
				r := gocppRun("", false, "run", ".")
				if r.code != 0 || r.stdout != "55\n" {
					t.Errorf("exit %d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
				}
			})
		})
	}
}

func TestTypedGoStructWithACFieldWrongValueIsGoStyle(t *testing.T) {
	const wrong = `cannot use "x" (untyped string constant) as int value in struct literal`
	for _, c := range []struct{ name, body, want string }{
		{"struct", `m := map[any]WS{k: {N: "x"}}`, wrong},
		{"pointer", `m := map[any]*WS{k: {N: "x"}}`, wrong},
		{"slice", `m := map[any][]WS{k: {{N: "x"}}}`, wrong},
		{"map", `m := map[any]map[string]WS{k: {"a": {N: "x"}}}`, wrong},
		{"array", `m := map[any][2]WS{k: {{N: 1}, {N: "x"}}}`, wrong},
		{"nested", `m := map[any]HS{k: {W: WS{N: "x"}}}`, wrong},
		{"nested map", `m := map[any]map[any]WS{k: {k: {N: "x"}}}`, wrong},
		{"too many", `m := map[any]WS{k: {1, 2, 3}}`, "too many values in struct literal of type WS"},
		{"number", `m := map[any]WS{k: 5}`, "cannot use 5 (untyped int constant) as WS value in map literal"},
		// the limits: no type that the lowering can write
		{"elided C struct", `m := map[any]hresult.Surface{k: {7}}`,
			"a composite literal without its type ({…}) for a C/C++ type next to a C/C++ map key is not supported yet: write the type"},
		{"elided C slice", `m := map[any][]hresult.NTSTATUS{k: {1, 2}}`, "for a C/C++ type next to a C/C++ map key is not supported yet"},
		{"elided C array", `m := map[any][2]hresult.NTSTATUS{k: {1, 2}}`, "for a C/C++ type next to a C/C++ map key is not supported yet"},
		{"elided generic instance", `m := map[any]Gen[hresult.NTSTATUS]{k: {N: 1}}`,
			"an instance of a generic type with a C/C++ type argument (Gen[C.NTSTATUS]) is not supported yet: the type arguments of a generic type are Go types"},
	} {
		t.Run(c.name, func(t *testing.T) {
			typedEnv(t)
			t.Setenv("GOCPP_INTEROP", "typed")
			inDir(t, goStructCFiles(t, strings.Replace(goStructCBad, "BODY", c.body, 1)))
			needClang(t)
			for _, verb := range []string{"check", "build"} {
				r := gocppRun("", true, verb, ".")
				if r.code == 0 || !strings.Contains(r.stdout, c.want) {
					t.Errorf("%s: exit %d, no %q in:\n%s", verb, r.code, c.want, r.stdout)
				}
				if strings.Contains(r.stdout, "(C/C++ value)") || strings.Contains(r.stdout, "internal error") {
					t.Errorf("%s: a leak of the machinery:\n%s", verb, r.stdout)
				}
			}
		})
	}
}

// The full form of an instance of a generic type with a C/C++ type argument is refused by the build
// with the same words as the elided literal; check does not know it yet (a Phase 04 gap, ADR-137).
func TestTypedGenericInstanceWithACArgumentIsRefusedByTheBuild(t *testing.T) {
	typedEnv(t)
	t.Setenv("GOCPP_INTEROP", "typed")
	body := `m := map[any]Gen[hresult.NTSTATUS]{k: Gen[hresult.NTSTATUS]{N: 1}}`
	inDir(t, goStructCFiles(t, strings.Replace(goStructCBad, "BODY", body, 1)))
	needClang(t)
	r := gocppRun("", true, "build", ".")
	if want := "an instance of a generic type with a C/C++ type argument (Gen[C.NTSTATUS]) is not supported yet"; r.code == 0 || !strings.Contains(r.stdout, want) {
		t.Errorf("exit %d, no %q in:\n%s", r.code, want, r.stdout)
	}
}

// The cost of `gocpp check` grows with the size of the program: a flat chain of N terms
// (int32(f()) + int32(f()) + ...) registered each of its N nodes with the whole chain below it, and
// making the key of each from its leaves cost the cube of N (1000 terms: 3.3 GB allocated, 40 s of CPU
// in the garbage collector; the fix allocates about a tenth of that).
func TestTypedCheckOfAFlatChainAllocatesLittle(t *testing.T) {
	typedEnv(t)
	t.Setenv("GOCPP_INTEROP", "typed")
	const terms = 1000
	chain := strings.Repeat("int32(hresult.status_of(2)) + ", terms-1) + "int32(hresult.status_of(2))"
	main := "package main\n\nimport (\n\t\"fmt\"\n\n\t\"./hresult.hpp\"\n)\n\nfunc main() {\n\tx := " + chain + "\n\tfmt.Println(x)\n}\n"
	inDir(t, goStructCFiles(t, main))
	needClang(t)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	start := time.Now()
	r := gocppRun("", true, "check", ".")
	took := time.Since(start)
	runtime.ReadMemStats(&after)
	alloc := (after.TotalAlloc - before.TotalAlloc) >> 20
	t.Logf("%d terms: %d MB allocated, %v", terms, alloc, took)
	if r.code != 0 {
		t.Fatalf("exit %d:\n%s", r.code, r.stdout)
	}
	if alloc > 1500 {
		t.Errorf("%d terms allocate %d MB", terms, alloc)
	}
}
