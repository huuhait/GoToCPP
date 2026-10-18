package rtfiles_test

import (
	"regexp"
	"strings"
	"testing"
)

// The cases of test/select_test.cpp that end the process (Phase 08, step 8.5): select {} and a
// select whose cases are all nil channels block for ever (and count as blocked), and the panics of
// a select. Each is run with the case name as the only argument. The helpers are in
// thread_test.go and cxx_test.go.

// runSelect runs the cases of test/select_test.cpp for a build.
func runSelect(t *testing.T, bin string) {
	want := func(name string, exit int, rx string) {
		t.Run(name, func(t *testing.T) {
			out, code, sig := caseRun(t, bin, name)
			if code != exit || sig != "" || !regexp.MustCompile(rx).MatchString(out) {
				t.Errorf("%s: exit code %d %s, output:\n%s\nwant exit code %d and %s", name, code, sig, out, exit, rx)
			}
		})
	}
	want("select-empty", 0, `^blocked\n$`)
	want("select-all-nil", 0, `^blocked\n$`)
	want("range-nil", 0, `^blocked\n$`)
	want("select-send-closed", 2, `^panic: send on closed channel\n\ngoroutine 1 \[running\]:\n`)
	want("select-send-closed-goroutine", 2, `^panic: send on closed channel\n\ngoroutine [2-9]\d* \[running\]:\n`)
	want("select-recv-no-zero", 2, `^fatal error: receive from a closed channel whose element type has no zero value\n`)
}

// TestSelectTSan builds test/select_test.cpp with ThreadSanitizer, with each compiler, and runs
// it: the unit tests (two selects that meet, the claim flag raced by eight goroutines, selects
// that list the same channels in opposite orders, lost wake-ups) and the cases that end the process
// must not print a TSan warning, which is a data race in select, in the channel or in the waiter.
func TestSelectTSan(t *testing.T) {
	t.Parallel()
	ccs := compilers(t)
	rt := extractRuntime(t)
	for _, c := range ccs {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			t.Parallel()
			flags := []string{"-O1", "-fsanitize=thread", "-fno-omit-frame-pointer"}
			bin := tsanBuild(t, c, rt, "test/select_test.cpp", flags)
			t.Run("unit", func(t *testing.T) {
				runUnit(t, bin)
			})
			for _, name := range []string{"select-empty", "select-all-nil", "range-nil", "select-send-closed", "select-send-closed-goroutine"} {
				t.Run(name, func(t *testing.T) {
					out, code, sig := caseRun(t, bin, name)
					if sig != "" || strings.Contains(out, "ThreadSanitizer") {
						t.Errorf("%s: exit code %d %s, output:\n%.3000s\nwant no TSan warning", name, code, sig, out)
					}
				})
			}
		})
	}
}
