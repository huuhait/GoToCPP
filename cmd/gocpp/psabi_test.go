package main

import "testing"

// A C++ value with 64-byte alignment as a parameter and as a deferred argument: g++ would print
// "note: the ABI for passing parameters with 64-byte alignment has changed in GCC 4.6" on every
// build of the program (build.GeneratedFlags has -Wno-psabi), and the run prints only the
// program's output.
func TestOverAlignedParameterPrintsNoNote(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", "15")
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, map[string]string{
			"gocpp.mod": "module example.com/wide\n",
			"wide.h":    "#pragma once\nstruct alignas(64) Wide { long v[8]; };\ninline void show(const Wide& w) { (void)w; }\n",
			"main.gp": `package main

import "./wide.h"

func f(w wide.Wide) {
	defer wide.show(w)
	println("in f")
}

func main() {
	var w wide.Wide
	f(w)
}
`,
		})
		if r := gocppRun("", true, "run", "."); r != (result{0, "in f\n", ""}) {
			t.Errorf("gocpp run = %+v, want exit 0 and only the program's output", r)
		}
	})
}
