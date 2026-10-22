package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The calls that return an HRESULT are Go errors (ADR-137), and the errors in using them are the same
// in `gocpp check` as in `gocpp build`: the build is the check with the lowering behind it, and a
// misuse that only the lowering or the C++ compiler saw would be an error of the build that the editor
// and `gocpp check` do not show.

// hresultMisuses are bodies of main that misuse the error of a call: each must be refused by check
// and by build, at the same places, with no word of the C++ compiler in the build's output.
var hresultMisuses = map[string]string{
	"declaration to a C number":   "var n C.long = hresult.status_of(2)\n\t_ = n",
	"variable to a C number":      "hr := hresult.status_of(2)\n\tvar n C.long = hr\n\t_ = n",
	"assignment to a C number":    "hr := hresult.status_of(2)\n\tvar n C.long\n\tn = hr\n\t_ = n",
	"a number to the error":       "hr := hresult.status_of(2)\n\thr = hresult.nt_result(1)\n\t_ = hr",
	"a constant to the error":     "hr := hresult.status_of(2)\n\thr = 5\n\t_ = hr",
	"addition":                    "hr := hresult.status_of(2)\n\thr += 1\n\t_ = hr",
	"increment":                   "hr := hresult.status_of(2)\n\thr++",
	"index":                       "hr := hresult.status_of(2)\n\t_ = hr[0]",
	"comparison with 0":           "hr := hresult.status_of(2)\n\tif hr != 0 {\n\t}",
	"comparison of the call":      "if hresult.status_of(2) < 0 {\n\t}",
	"Go number":                   "var n int32 = hresult.status_of(2)\n\t_ = n",
	"Go number from the variable": "hr := hresult.status_of(2)\n\tvar n int32\n\tn = hr\n\t_ = n",
	"switch on the code":          "hr := hresult.status_of(2)\n\tswitch hr {\n\tcase 0:\n\t}",
	"arithmetic":                  "_ = hresult.status_of(2) + 1",
	"two variables":               "a, b := hresult.status_of(2)\n\t_, _ = a, b",
	"two declared errors":         "var x, y error = hresult.status_of(2)\n\t_, _ = x, y",
	"two callees":                 "sc := hresult.new_swap_chain()\n\ta, b := hresult.status_of(2)\n\tc, d := sc.Present(1, 2)\n\t_, _, _, _ = a, b, c, d",
	"two callees in parentheses":  "sc := hresult.new_swap_chain()\n\ta, b := (hresult.status_of(2))\n\tc, d := (sc.Present(1, 2))\n\t_, _, _, _ = a, b, c, d",
}

// hresultMisuseWords are the words that the errors of a misuse have to hold, in check and in build.
var hresultMisuseWords = map[string][]string{
	// each mismatch names its own function, in one body
	"two callees":                {"assignment mismatch: 2 variables but hresult.status_of returns 1 value", "assignment mismatch: 2 variables but sc.Present returns 1 value"},
	"two callees in parentheses": {"assignment mismatch: 2 variables but hresult.status_of returns 1 value", "assignment mismatch: 2 variables but sc.Present returns 1 value"},
}

func TestTypedHResultMisuseIsTheSameInCheckAndBuild(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		typedEnv(t)
		typedIn(t)
		needClang(t)
		t.Setenv("GOCPP_INTEROP", "typed")
		for name, body := range hresultMisuses {
			file := filepath.Join(".", "misuse_"+strings.ReplaceAll(strings.ToLower(name), " ", "_")+".gp")
			src := "package main\n\nimport (\n\t\"./hresult.hpp\"\n)\n\nfunc main() {\n\t" + body + "\n}\n"
			t.Run(name, func(t *testing.T) {
				if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
					t.Fatal(err)
				}
				check := gocppRun("", true, "check", file)
				build := gocppRun("", true, "build", "-o", filepath.Join(t.TempDir(), "x"), file)
				if check.code == 0 {
					t.Fatalf("check passes:\n%s\nthe build says:\n%s", src, build.stdout)
				}
				if build.code == 0 {
					t.Fatalf("the build passes:\n%s", src)
				}
				want := positionedErrors(check.stdout)
				if len(want) == 0 {
					t.Fatalf("check has no error with a position:\n%s", check.stdout)
				}
				got := positionedErrors(build.stdout)
				if strings.Join(got, "\n") != strings.Join(want, "\n") {
					t.Errorf("check and build differ:\ncheck:\n%s\nbuild:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
				}
				for _, out := range []string{check.stdout, build.stdout} {
					for _, word := range hresultMisuseWords[name] {
						if !strings.Contains(out, word) {
							t.Errorf("no %q in:\n%s", word, out)
						}
					}
					if strings.Contains(out, "gocpp_probe_internal") || strings.Contains(out, "gp_cgoh") || strings.Contains(out, "C++:") {
						t.Errorf("the output shows the machinery or the C++ compiler:\n%s", out)
					}
				}
			})
		}
	})
}

// TestTypedEmitHResultWrapsOnlyTheErrors pins which calls the generated C++ makes an error of: the
// ones the program uses as a value. A call that is a statement, deferred, dropped with
// `_ =`, or converted to a number is the C++ call as it is, and a call that the program converts to
// error itself is wrapped once (ADR-137).
func TestTypedEmitHResultWrapsOnlyTheErrors(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		typedEnv(t)
		typedIn(t)
		needClang(t)
		t.Setenv("GOCPP_INTEROP", "typed")
		const src = `package main

import "./hresult.hpp"

func main() {
	sc := hresult.new_swap_chain()
	sc.Present(1, 11)
	defer sc.Present(1, 12)
	_ = sc.Present(1, 14)
	a := int32(sc.Present(1, 15))
	b := C.long(sc.Present(1, 16))
	hr := sc.Present(1, 17)
	var e error = sc.Present(1, 18)
	c := error(sc.Present(1, 19))
	_, _, _, _, _ = a, b, hr, e, c
}
`
		if err := os.WriteFile("a.gp", []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		r := gocppRun("", false, "emit", "a.gp")
		if r.code != 0 {
			t.Fatalf("typed emit: exit %d\n%s", r.code, r.stderr)
		}
		out := r.stdout
		for _, flags := range []string{"11", "12", "14", "15", "16"} {
			i := strings.Index(out, "Present(1, "+flags+")")
			if i < 0 {
				t.Fatalf("no call with flags %s in:\n%s", flags, out)
			}
			line := out[strings.LastIndex(out[:i], "\n")+1 : i+strings.Index(out[i:], "\n")]
			if strings.Contains(line, "hresult_error") {
				t.Errorf("the call with flags %s is an error, and the program drops it or wants the number:\n%s", flags, line)
			}
		}
		for _, flags := range []string{"17", "18", "19"} {
			want := "gp::hresult_error(gp::nonnull((*sc))->Present(1, " + flags + "))"
			if !strings.Contains(out, want) {
				t.Errorf("no %q in:\n%s", want, out)
			}
		}
		if n := strings.Count(out, "hresult_error("); n != 3 {
			t.Errorf("%d conversions to error, want 3 (a call is wrapped once):\n%s", n, out)
		}
	})
}

// hresultAsNumber uses the result of an HRESULT function as the number that C++ returns.
const hresultAsNumber = `package main

import (
	"fmt"

	"./hresult.hpp"
)

func main() {
	hr := hresult.status_of(2)
	fmt.Println(hr != 0, int32(hr) < 0)
}
`

// -interop=opaque keeps the number (ADR-137): the program that typed mode refuses builds and runs, and
// the hidden developer command of typed interop, which checks without typed mode, does not make the
// calls errors either.
func TestOpaqueModeKeepsTheHResultNumber(t *testing.T) {
	typedEnv(t)
	typedIn(t)
	needClang(t)
	if err := os.WriteFile("number.gp", []byte(hresultAsNumber), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOCPP_INTEROP", "typed")
	if r := gocppRun("", true, "check", "number.gp"); r.code == 0 {
		t.Fatalf("typed mode accepts the number:\n%s", r.stdout)
	}
	t.Setenv("GOCPP_INTEROP", "opaque")
	if r := gocppRun("", true, "check", "number.gp"); r.code != 0 {
		t.Errorf("opaque check: exit %d\n%s", r.code, r.stdout)
	}
	if r := gocppRun("", false, "run", "number.gp"); r.code != 0 || strings.TrimSpace(r.stdout) != "true true" {
		t.Errorf("opaque run: exit %d, output %q\n%s", r.code, r.stdout, r.stderr)
	}
	if r := gocppRun("", true, "__typed-check", "number.gp"); r.code != 0 || strings.TrimSpace(r.stdout) != "" {
		t.Errorf("__typed-check: exit %d\n%s", r.code, r.stdout)
	}
}
