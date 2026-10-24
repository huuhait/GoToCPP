// phase: 09
// flags: -release -trace

// -release -trace: runtime.Callers gives a frame for each call that the compiler inlined, as Go's
// does, and CallersFrames and Caller name them with their .gp lines: where and helper are inlined
// into main.main, and Caller(0) in main is the line of the call, not a line of the runtime's code
// that is inlined there (ADR-136). Each frame keeps Go's rules: a Frame with a Func has
// Function == Func.Name() (an inlined call's frame has none in Go, and in gocpp), and FuncForPC of
// the frame's PC names the frame's function (the innermost one at an inlined call). The second part
// has the shapes of a method value (no wrapper frame: Go elides its wrappers, gocpp leaves out the
// lambda of a method value), a pointer method value, two instances of a generic function and a
// function inlined twice.
package main

import (
	"fmt"
	"runtime"
	"strings"
)

// consistent says whether every frame keeps the rules above.
var consistent = true

func where() ([]string, []string) {
	pcs := make([]uintptr, 8)
	n := runtime.Callers(1, pcs)
	fr := runtime.CallersFrames(pcs[:n])
	var out, files []string
	for {
		f, more := fr.Next()
		if strings.HasPrefix(f.Function, "main.") {
			out = append(out, fmt.Sprintf("%s:%d", f.Function, f.Line))
			files = append(files, f.File)
			if f.Func != nil && f.Func.Name() != f.Function {
				consistent = false
			}
			if fn := runtime.FuncForPC(f.PC); fn == nil || fn.Name() != f.Function {
				consistent = false
			}
		}
		if !more {
			break
		}
	}
	return out, files
}

func helper() ([]string, []string) { return where() }

func main() {
	frames, files := helper()
	_, file, line, ok := runtime.Caller(0)
	fmt.Println(strings.Join(frames, " | "))
	same := len(files) == 3
	for _, f := range files {
		same = same && f == file
	}
	fmt.Println(line, ok, same, consistent)
	shapes()
}

func callers() []uintptr {
	pcs := make([]uintptr, 16)
	n := runtime.Callers(2, pcs)
	return pcs[:n]
}

func show(tag string, pcs []uintptr) {
	fr := runtime.CallersFrames(pcs)
	var parts []string
	bad := 0
	for {
		f, more := fr.Next()
		if strings.HasPrefix(f.Function, "main.") {
			parts = append(parts, fmt.Sprintf("%s:%d", f.Function, f.Line))
			if f.Func != nil && f.Func.Name() != f.Function {
				bad++
			}
			if g := runtime.FuncForPC(f.PC); g == nil || g.Name() != f.Function {
				bad++
			}
		}
		if !more {
			break
		}
	}
	fmt.Println(tag, strings.Join(parts, " < "), "bad", bad)
}

type T struct{ n int }

func (t T) M() []uintptr { return callers() }

func (t *T) P() []uintptr { return callers() }

func G[X any](x X) []uintptr { return callers() }

func twice() []uintptr { return callers() }

func shapes() {
	t := T{1}
	mv := t.M
	show("methodvalue", mv())
	pv := (&t).P
	show("ptrmethodvalue", pv())
	show("generic-int", G(1))
	show("generic-string", G("s"))
	a := twice()
	b := twice()
	show("twice-a", a)
	show("twice-b", b)
	fa, fb := runtime.FuncForPC(a[0]), runtime.FuncForPC(b[0])
	fmt.Println("twice names", fa.Name(), fb.Name())
}

// Output:
// main.where:26 | main.helper:48 | main.main:51
// 52 true true true
// methodvalue main.T.M:92 < main.shapes:103 < main.main:59 bad 0
// ptrmethodvalue main.(*T).P:94 < main.shapes:105 < main.main:59 bad 0
// generic-int main.G[...]:96 < main.shapes:106 < main.main:59 bad 0
// generic-string main.G[...]:96 < main.shapes:107 < main.main:59 bad 0
// twice-a main.twice:98 < main.shapes:108 < main.main:59 bad 0
// twice-b main.twice:98 < main.shapes:109 < main.main:59 bad 0
// twice names main.twice main.twice
