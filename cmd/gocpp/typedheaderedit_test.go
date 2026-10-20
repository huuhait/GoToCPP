package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// A header edit invalidates the probes (Phase 12 step 12.12, ADR-088): the answers of the probes are
// cached on disk under the contents of the headers, so an edited header must give the answers of the
// new contents on the next `gocpp check`, and an unedited one must give them with no compiler run.
// Two things of the program depend on the header, one for each kind of answer: the value of a macro
// (an array length that decides an index out of range) and the signature of a function (a call that
// C++ accepts or rejects).
//
// The edits keep the size of the file and its modification time, so a cache that keyed on either
// would serve the old answers.
func TestTypedHeaderEditInvalidatesProbes(t *testing.T) {
	const lengthProgram = `package main

import "./lib.hpp"

var table [lib.ROWS]int

func main() {
	table[5] = 1
}
`
	const callProgram = `package main

import "./lib.hpp"

func main() {
	lib.show("text")
}
`
	headerWith := func(rows, param string) string {
		return "#pragma once\n#define ROWS " + rows + "\ninline void show(" + param + ") {}\n"
	}
	// headerV1: 4 rows, so table[5] is out of range; show takes a number, so "text" does not fit.
	// headerV2: 8 rows, so table[5] is fine; show takes a string. Both have the same size.
	headerV1 := headerWith("4", "unsigned int")
	headerV2 := headerWith("8", "const char *")
	if len(headerV1) != len(headerV2) {
		t.Fatalf("the two headers must have the same size, they have %d and %d bytes",
			len(headerV1), len(headerV2))
	}

	typedEnv(t)
	cache := t.TempDir()
	t.Setenv(probe.EnvProbeCache, cache)
	t.Setenv(probe.EnvNoProbeCache, "")
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_CXX", "clang++")
	dir := inDir(t, map[string]string{
		"length.gp": lengthProgram,
		"call.gp":   callProgram,
		"gocpp.mod": "module m\n",
	})
	needClang(t)
	defer probe.SetRacyWindow(0)()

	header := filepath.Join(dir, "lib.hpp")
	longAgo := time.Now().Add(-time.Hour).Truncate(time.Second)
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(header, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(header, longAgo, longAgo); err != nil {
			t.Fatal(err)
		}
	}
	check := func() (string, int64) {
		t.Helper()
		probe.ResetCacheCounters()
		length := gocppRun("", false, "check", "-e", "length.gp")
		call := gocppRun("", false, "check", "-e", "call.gp")
		return length.stderr + call.stderr, probe.CacheCounters().Runs
	}

	// The first header: both mistakes are reported, and the compiler ran to find them.
	write(headerV1)
	got, runs := check()
	if runs == 0 {
		t.Errorf("a cold check made no compiler run")
	}
	for _, want := range []string{
		"./length.gp:8:8: invalid argument: index 5 out of bounds [0:4]",
		`./call.gp:6:11: cannot use "text" (untyped string constant) as C.uint value ` +
			"in argument to lib.show",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the first header: no %q in\n%s", want, got)
		}
	}

	// The same header again: the same errors, and no compiler run (the cache answers).
	again, runs := check()
	if again != got || runs != 0 {
		t.Errorf("the same header, second check: %d compiler runs, same output %v\n%s",
			runs, again == got, again)
	}

	// The edited header, same size and same time: no error left, and the compiler ran again.
	write(headerV2)
	fixed, runs := check()
	if runs == 0 {
		t.Errorf("the header changed, but no compiler ran: " +
			"the cache served the answers of the old contents")
	}
	if fixed != "" {
		t.Errorf("the edited header has no mistake left, but the check prints:\n%s", fixed)
	}

	// Back to the first header: its answers are in the cache still, and they are served.
	write(headerV1)
	back, runs := check()
	if back != got {
		t.Errorf("back to the first header: the output differs from the first one:\n%s", back)
	}
	if runs != 0 {
		t.Errorf("back to the first header: %d compiler runs, its answers were cached", runs)
	}
}
