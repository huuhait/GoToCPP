package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestFixtureOf(t *testing.T) {
	for _, c := range []struct {
		file, fn, name string
		ok             bool
	}{
		{"tests/run/algo_bfs.gp", "TestRun", "algo_bfs", true},
		{"tests/errors/x.gp", "TestErrors", "x", true},
		{"tests/golden/control.cpp.golden", "TestGolden", "control", true},
		{"tests/golden/control.gp", "TestGolden", "control", true},
		{"tests/interop/vector_values.gp", "TestInterop", "vector_values", true},
		{"examples/hello/main.gp", "TestExamples", "hello", true},
		{"examples/hello/output.txt", "TestExamples", "hello", true},
		{"tests/interop/crange.h", "", "", false},                                 // a helper header: not one fixture
		{"tests/interop/helper_pair/main.gp", "TestInterop", "helper_pair", true}, // a folder test
		{"tests/zoo/vector.gp", "TestZoo", "vector", true},
		{"tests/harness_test.go", "", "", false},
		{"tests/pkgs/basic_cross/geom/geom.gp", "TestPkgs", "basic_cross", true},
		{"tests/pkgs/errors/internal_rule/main.gp", "TestPkgErrors", "internal_rule", true},
		{"tests/pkgs/README.md", "", "", false},
		{"tests/run/README.md", "", "", false},
		{"internal/lower/order.go", "", "", false},
	} {
		fn, name, ok := fixtureOf(c.file)
		if fn != c.fn || name != c.name || ok != c.ok {
			t.Errorf("fixtureOf(%q) = %q, %q, %v; want %q, %q, %v", c.file, fn, name, ok, c.fn, c.name, c.ok)
		}
	}
}

func TestFixtureRegex(t *testing.T) {
	got := fixtureRegex("TestRun", []string{"a_b", "c.d"})
	if want := `^TestRun$/^(a_b|c\.d)$`; got != want {
		t.Errorf("fixtureRegex = %q, want %q", got, want)
	}
}

func TestMakePlan(t *testing.T) {
	t.Chdir("../..") // makePlan checks that a changed Go file still exists, from the repository root
	// A change in a Go package: gofmt, vet and the package's tests, then the smoke fixtures only.
	p := makePlan([]string{"internal/lower/doc.go"}, "g++")
	var names []string
	for _, s := range p.steps {
		names = append(names, s.name)
	}
	if want := []string{"gofmt", "go vet", "go test packages", "TestErrors", "TestGolden", "TestRun"}; !reflect.DeepEqual(names, want) {
		t.Errorf("steps = %q, want %q", names, want)
	}
	// A changed fixture joins the smoke set of its kind, and gets its own kind if it has none.
	p = makePlan([]string{"tests/interop/vector_values.gp", "tests/run/algo_bfs.gp"}, "clang++")
	var cmds []string
	for _, s := range p.steps {
		cmds = append(cmds, s.String())
	}
	joined := strings.Join(cmds, "\n")
	for _, want := range []string{
		`GOCPP_CXX=clang++ go test -count=1 -timeout 30m ./tests -run ^TestInterop$/^(vector_values)$`,
		`^TestRun$/^(algo_bfs|algo_primes|arrays_basics)$`,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("no step %q in\n%s", want, joined)
		}
	}
	// A runtime change adds the runtime's fast tests and the debug-mode C++ unit tests of one compiler.
	p = makePlan([]string{"runtime/include/gocpp/slice.hpp"}, "g++")
	joined = ""
	for _, s := range p.steps {
		joined += s.String() + "\n"
	}
	if !strings.Contains(joined, `./runtime -run ^TestCXX$/^g\+\+$/^O0$`) || !strings.Contains(joined, "TestLayout") {
		t.Errorf("runtime steps missing:\n%s", joined)
	}
	// The interop docs and the files they copy get their snippet test.
	for _, f := range []string{"docs/spec/interop-cookbook.md", "tests/interop/cookbook_zlib.gp", "tests/zoo/sdl2.gp"} {
		p = makePlan([]string{f}, "g++")
		joined = ""
		for _, s := range p.steps {
			joined += s.String() + "\n"
		}
		if !strings.Contains(joined, "./tests -run ^TestDocSnippets$") {
			t.Errorf("%s: no doc snippet step in\n%s", f, joined)
		}
	}
	// Testdata and spikes are ignored; a helper header gets a note.
	p = makePlan([]string{"internal/syntax/testdata/x.go", "spikes/a/b.go", "tests/interop/crange.h"}, "g++")
	if len(p.notes) != 1 || !strings.Contains(p.notes[0], "crange.h") {
		t.Errorf("notes = %q", p.notes)
	}
}
