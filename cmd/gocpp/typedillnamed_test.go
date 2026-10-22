package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Code that does not type, in a package with C/C++ imports, is an error of `gocpp check` and `gocpp
// build` as it is in any other package: the HRESULT hook of check collects the calls before the type
// errors are known, and the collector used to overflow its stack on a qualified type name that
// go/types had left invalid (a package that is not imported, an undefined type).

// checkEnds runs `gocpp check file` with a bound on the time and returns the result.
func checkEnds(t *testing.T, file string) result {
	t.Helper()
	done := make(chan result, 1)
	go func() { done <- gocppRun("", true, "check", file) }()
	select {
	case r := <-done:
		return r
	case <-time.After(2 * time.Minute):
		t.Fatalf("check does not end on %s", file)
	}
	return result{}
}

var illTypedMains = map[string]struct{ body, want string }{
	"undefined type in a literal": {"p := []fmt.Nosuch{cstdlib.abs(-3)}\n\t_ = p", "fmt.Nosuch"},
	"package not imported":        {"k := cstdlib.abs(-3)\n\t_ = lib.Pair{k, 2}", "undefined: lib"},
	"package not imported in map": {"k := cstdlib.abs(-3)\n\t_ = map[any]lib.Pair{k: {}}", "undefined: lib"},
	"undefined type as map key":   {"k := cstdlib.abs(-3)\n\t_ = map[fmt.Nosuch]int{k: 1}", "fmt.Nosuch"},
	"nested literal":              {"k := cstdlib.abs(-3)\n\t_ = [][]fmt.Nosuch{{k}, {k, k}}", "fmt.Nosuch"},
	"undefined type in a func":    {"k := cstdlib.abs(-3)\n\tf := func(a lib.Pair) lib.Pair { return a }\n\t_, _ = f, k", "undefined: lib"},
}

const illTypedTop = `
type Holder struct {
	A lib.Pair
	B []fmt.Nosuch
}

func sig(a lib.Pair, rest ...fmt.Nosuch) (n lib.Pair, err error) { return }

func generic[T any](v T) map[any]lib.Pair {
	k := cstdlib.abs(-3)
	return map[any]lib.Pair{k: {}}
}
`

func TestTypedIllTypedPackagesAreErrors(t *testing.T) {
	typedEnv(t)
	t.Setenv("GOCPP_INTEROP", "typed")
	needClang(t)
	for name, c := range illTypedMains {
		t.Run(name, func(t *testing.T) {
			src := "package main\n\nimport (\n\t\"fmt\"\n\t\"<cstdlib>\"\n)\n" + illTypedTop + "\nfunc main() {\n\t" + c.body + "\n\tfmt.Println()\n}\n"
			inDir(t, map[string]string{"main.gp": src})
			r := checkEnds(t, "main.gp")
			if r.code == 0 || !strings.Contains(r.stdout, c.want) {
				t.Errorf("exit %d, want an error with %q:\n%s", r.code, c.want, r.stdout)
			}
			if strings.Contains(r.stdout, "not supported yet") {
				t.Errorf("a C/C++ limit next to an error of the program:\n%s", r.stdout)
			}
			if strings.Contains(r.stdout, "goroutine ") || strings.Contains(r.stdout, "internal error") {
				t.Errorf("a crash:\n%s", r.stdout)
			}
		})
	}
}

// brokenVariants returns the programs made of src with one thing broken: an import taken out, a
// package name or a type misspelled.
func brokenVariants(src string) map[string]string {
	out := map[string]string{}
	importLine := regexp.MustCompile(`(?m)^\t"(?:fmt|errors|time|strings|sort)"\n`)
	if loc := importLine.FindStringIndex(src); loc != nil {
		out["no import"] = src[:loc[0]] + src[loc[1]:]
	}
	if i := strings.Index(src, "fmt.Println"); i >= 0 {
		out["misspelled package"] = src[:i] + "fmtx.Println" + src[i+len("fmt.Println"):]
	}
	if i := strings.Index(src, "error"); i >= 0 {
		out["misspelled type"] = src[:i] + "errorr" + src[i+len("error"):]
	}
	return out
}

// TestTypedCheckEndsOnBrokenVariantsOfTheTypedPrograms feeds ill-typed variants of every program
// of tests/typed (a valid program with an import taken out or a name misspelled) to `gocpp check`:
// each ends, with an error or without, never with a crash.
func TestTypedCheckEndsOnBrokenVariantsOfTheTypedPrograms(t *testing.T) {
	typedEnv(t)
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "")
	typedIn(t)
	needClang(t)
	progs := loadTypedPrograms(t)
	n := 0
	for _, p := range progs {
		if strings.HasPrefix(p.name, "err_") || strings.HasPrefix(p.name, "reject_") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(".", p.file))
		if err != nil {
			t.Fatal(err)
		}
		for what, broken := range brokenVariants(string(b)) {
			file := "variant_" + p.name + "_" + strings.ReplaceAll(what, " ", "_") + ".gp"
			if err := os.WriteFile(file, []byte(broken), 0o644); err != nil {
				t.Fatal(err)
			}
			n++
			r := checkEnds(t, file)
			if strings.Contains(r.stdout, "goroutine ") || strings.Contains(r.stdout, "stack exceeds") || strings.Contains(r.stdout, "panic:") {
				t.Errorf("%s (%s) crashes:\n%s", p.name, what, r.stdout)
			}
		}
	}
	if n < 40 {
		t.Errorf("only %d variants were made", n)
	}
}

// The value next to a C/C++ key of a map literal is typed by the element type of the map, however the
// type is spelled where the map was declared: an aliased import, a tag, an embedded field, a type of a
// package or file that the using file doesn't import, an unexported one, a shadowed package name or
// type. Each of them builds and runs with both compilers and prints what the Go program prints
// (the same program with a Go number as the key).
func TestTypedMapKeyValueTypesNeedNoName(t *testing.T) {
	header, err := os.ReadFile(filepath.Join(typedDir, "hresult.hpp"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"gocpp.mod":   "module example.com/m\n",
		"hresult.hpp": string(header),
		"lib/lib.gp":  "package lib\n\ntype Pair struct{ A, B int }\n\ntype pair struct{ A, B int }\n\ntype HM map[any]pair\n\ntype PM map[any]Pair\n",
		"a.gp":        "package main\n\nimport (\n\ttm \"time\"\n\n\t\"example.com/m/lib\"\n)\n\ntype MP map[any]lib.Pair\n\ntype MQ map[any]tm.Time\n",
		"main.gp":     mapKeyMain,
	}
	forEachCXX(t, func(t *testing.T, c testCXX) {
		typedEnv(t)
		t.Setenv("GOCPP_INTEROP", "typed")
		t.Setenv("GOCPP_GATE_PHASE", "12")
		inDir(t, files)
		needClang(t)
		r := gocppRun("", false, "run", ".")
		if r.code != 0 || r.stdout != "1 1 2 4 6 7\n1\n1 9\n" {
			t.Errorf("exit %d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
		}
	})
}

const mapKeyMain = `package main

import (
	"fmt"
	tm "time"

	"./hresult.hpp"
	"example.com/m/lib"
)

type T struct{ A int }

func main() {
	k := hresult.nt_result(1)
	a := map[any]tm.Time{k: {}}
	b := map[any]struct {
		A int "a"
	}{k: {1}}
	c := map[any]struct{ T }{k: {T{2}}}
	d := MP{k: {3, 4}}
	e := lib.HM{k: {5, 6}}
	f := lib.PM{k: {7, 8}}
	fmt.Println(len(a), b[k].A, c[k].A, d[k].B, e[k].B, f[k].A)
	{
		tm := 5
		_ = tm
		g := MQ{k: {}}
		fmt.Println(len(g))
	}
	type L struct{ A int }
	type ML map[any]L
	{
		type L int
		h := ML{k: {9}}
		fmt.Println(len(h), h[k].A)
	}
}
`
