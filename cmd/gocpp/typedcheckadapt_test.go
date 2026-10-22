package main

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The errors of the argument adaptations (ADR-130, ADR-134) are the same in `gocpp check` as in
// `gocpp build`: the lowering and the interop checker ask one decision function (adapt.Decide), so
// an error test of tests/typed that the lowering refuses is refused by `gocpp check` with the same
// words at the same place, with either C++ compiler (they are errors of the language, not the
// warnings that a g++ build makes of the probe's).

// positionedError matches a line with a line and a column (the refusals of the lowering), not a C++
// compiler's line (`file.gp:16: C++: ...`); positionedErrors leaves out the warnings.
var positionedError = regexp.MustCompile(`^\./\S+\.gp:\d+:\d+: `)

// positionedErrors returns the lines of out that are errors with a position.
func positionedErrors(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if positionedError.MatchString(l) && !strings.Contains(l, ": warning: ") {
			lines = append(lines, l)
		}
	}
	return lines
}

// leftToCpp are the error tests of the adaptations whose error is the C++ compiler's, not a refusal
// of the lowering: the argument is not adapted (an array parameter, the address of an element,
// overloads that disagree, a typed constant for an enum), so C++ rejects the call, and `gocpp
// check` leaves it to C++ as it leaves every other call that the probe cannot mirror.
var leftToCpp = map[string]bool{
	"err_adapt_array_const":          true,
	"err_adapt_array_float":          true,
	"err_adapt_array_method":         true,
	"err_adapt_array_voidptr":        true,
	"err_adapt_array_writer":         true,
	"err_adapt_array_typedef":        true,
	"err_adapt_array_typedef_const":  true,
	"err_adapt_array_typedef_matrix": true,
	"err_adapt_array_typedef_method": true,
	"err_adapt_element":              true,
	"err_adapt_overloads":            true,
	"err_adapt_typedconst":           true,
}

func TestTypedCheckReportsAdaptationErrors(t *testing.T) {
	progs := loadTypedPrograms(t)
	forEachCXX(t, func(t *testing.T, c testCXX) {
		typedEnv(t)
		typedIn(t)
		needClang(t)
		t.Setenv("GOCPP_INTEROP", "typed")
		same := 0
		for _, p := range progs {
			if !strings.HasPrefix(p.name, "err_adapt_") || leftToCpp[p.name] {
				continue
			}
			t.Run(p.name, func(t *testing.T) {
				// The build without the checker's pass: what the lowering refuses on its own.
				t.Setenv("GOCPP_TEST_NO_ADAPT_ERRORS", "1")
				build := gocppRun("", true, "build", "-o", filepath.Join(t.TempDir(), "x"), p.file)
				t.Setenv("GOCPP_TEST_NO_ADAPT_ERRORS", "")
				want := positionedErrors(build.stdout)
				if build.code == 0 || len(want) == 0 {
					t.Fatalf("the build has no error with a position (exit %d):\n%s", build.code, build.stdout)
				}
				check := gocppRun("", true, "check", p.file)
				if check.code == 0 {
					t.Fatalf("check passes; the build says:\n%s", strings.Join(want, "\n"))
				}
				got := positionedErrors(check.stdout)
				if strings.Join(got, "\n") != strings.Join(want, "\n") {
					t.Fatalf("check and build differ:\ncheck:\n%s\nbuild:\n%s",
						strings.Join(got, "\n"), strings.Join(want, "\n"))
				}
				same++
			})
		}
		if same < 16 {
			t.Errorf("only %d error tests were compared, tests/typed has more refusals", same)
		}
	})
}

// A program of tests/typed that builds is accepted by check, with no error: the pass reports only
// what the lowering refuses.
func TestTypedCheckAcceptsTheAdaptedPrograms(t *testing.T) {
	progs := loadTypedPrograms(t)
	forEachCXX(t, func(t *testing.T, c testCXX) {
		typedEnv(t)
		typedIn(t)
		needClang(t)
		t.Setenv("GOCPP_INTEROP", "typed")
		for _, p := range progs {
			if p.errorRE != nil {
				continue
			}
			t.Run(p.name, func(t *testing.T) {
				if why := missingLibs(p.libs); why != "" {
					t.Skip(why)
				}
				r := gocppRun("", true, "check", p.file)
				if r.code != 0 || len(positionedErrors(r.stdout)) > 0 {
					t.Fatalf("check rejects a program that builds: exit %d\n%s", r.code, r.stdout)
				}
			})
		}
	})
}

// `gocpp vet` runs the checker too: it shows the refusals as `check` does.
func TestTypedVetReportsAdaptationErrors(t *testing.T) {
	progs := loadTypedPrograms(t)
	forEachCXX(t, func(t *testing.T, c testCXX) {
		typedEnv(t)
		typedIn(t)
		needClang(t)
		t.Setenv("GOCPP_INTEROP", "typed")
		for _, p := range progs {
			if p.name != "err_adapt_constchar" && p.name != "err_adapt_printf" {
				continue
			}
			t.Run(p.name, func(t *testing.T) {
				check := gocppRun("", true, "check", p.file)
				vet := gocppRun("", true, "vet", p.file)
				want := positionedErrors(check.stdout)
				if len(want) != 1 || vet.code == 0 ||
					strings.Join(positionedErrors(vet.stdout), "\n") != strings.Join(want, "\n") {
					t.Fatalf("vet differs from check:\ncheck (exit %d):\n%s\nvet (exit %d):\n%s",
						check.code, check.stdout, vet.code, vet.stdout)
				}
			})
		}
	})
}
