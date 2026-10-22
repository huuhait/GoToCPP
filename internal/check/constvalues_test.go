package check

import (
	"testing"
)

// checkErrors checks the program as the gates of the phase allow and with
// a probe that runs no compiler, and returns what gocpp check prints.
func checkErrors(t *testing.T, phase int, src string) string {
	t.Helper()
	return probeErrors(t, fakeProbeConfig(phase), map[string]string{
		"main.gp":    src,
		"lib/lib.gp": "package lib\n\nfunc F() C.int { return 1 }\n", // for the programs that import it
	})
}

// TestConstOfACVariableIsNotConstant checks the error of Go for a const declaration whose value
// holds a variable (constvalues.go): go/types has no type for the value, so it says nothing, and
// the lowering used to be the one to report it, after gocpp check passed.
func TestConstOfACVariableIsNotConstant(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, src, want string }{
		{"local C variable", `package main

func main() {
	var x C.int = 2
	const d = x + 1
	_ = d
}
`, "./main.gp:5:12: x + 1 (C/C++ value) is not constant"},
		{"comparison", `package main

func main() {
	var x C.int
	const c = x == 2
	_ = c
}
`, "./main.gp:5:12: x == 2 (C/C++ value) is not constant"},
		{"package C variable", `package main

var g C.int = 4

const k = g * 2
`, "./main.gp:5:11: g * 2 (C/C++ value) is not constant"},
		{"variable of a named C type", `package main

type Handle C.int

var h Handle

const k = h + 1
`, "./main.gp:7:11: h + 1 (C/C++ value) is not constant"},
		{"call of a Go function with a C result", `package main

func f() C.int { return 1 }

const k = f()
`, "./main.gp:5:11: f() (C/C++ value) is not constant"},
		{"call of a Go method with a C result", `package main

type T struct{}

func (T) M() C.int { return 1 }

const k = T{}.M()
`, "./main.gp:7:11: T{}.M() (C/C++ value) is not constant"},
		{"call of a Go function of another package", `package main

import "example.com/m/lib"

const k = lib.F()
`, "./main.gp:5:11: lib.F() (C/C++ value) is not constant"},
		{"call of a func value", `package main

var fv func() C.int

const k = fv()
`, "./main.gp:5:11: fv() (C/C++ value) is not constant"},
		{"variable next to a conversion", `package main

var g C.int

const k = g + C.int(1)
`, "./main.gp:5:11: g + C.int(1) (C/C++ value) is not constant"},
		{"variable in a group of constants", `package main

var g C.int

const (
	a = 1
	b = g + a
)
`, "./main.gp:7:6: g + a (C/C++ value) is not constant"},
		{"in the const of a function", `package main

func f(n C.int) {
	const k = n
	_ = k
}
`, "./main.gp:4:12: n (C/C++ value) is not constant"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			want := "# example.com/m\n" + c.want + "\n"
			if got := checkErrors(t, LastPhase, c.src); got != want {
				t.Errorf("errors:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// TestConstOfACVariableIsNotAGate checks that the error is not the gate of the C names in
// constant expressions when both would apply: the value holds a variable and a name of a header,
// and the variable is never a constant.
func TestConstOfACVariableIsNotAGate(t *testing.T) {
	t.Parallel()
	const src = `package main

import "<stdio.h>"

var g C.int

const k = g + stdio.BUFSIZ

const m = stdio.BUFSIZ
`
	got := checkErrors(t, 5, src)
	want := `# example.com/m
./main.gp:7:11: g + stdio.BUFSIZ (C/C++ value) is not constant
./main.gp:9:11: C/C++ names in constant expressions are not supported yet (Phase 12)
`
	if got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
}

// TestConstValuesThatAreConstants checks that a constant is no error: a variable only in the
// argument of len or cap (whose argument is not evaluated), a constant of a C type, and a
// conversion to one.
func TestConstValuesThatAreConstants(t *testing.T) {
	t.Parallel()
	const src = `package main

var arr [3]C.int
var g C.int

const (
	n    = len(arr)
	c    = cap(arr)
	k    C.int = 1
	j          = k + 1
	cv         = C.int(1)
	z    C.int = iota
	w          = C.int(len(arr))
	v          = C.int(cap(arr)) + k
)

func main() {
	println(n, c, j, cv, z, w, v)
}
`
	if got := checkErrors(t, LastPhase, src); got != "" {
		t.Errorf("errors:\n%s", got)
	}
}
