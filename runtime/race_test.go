package rtfiles_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// raceReport is Go's report of the race of the cases below: the two accesses with their
// goroutines, and the last line.
var raceReport = regexp.MustCompile(`(?s)^==================\nWARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by goroutine \d+:\n.*\n\nPrevious (write|read) at 0x[0-9a-f]+ by goroutine \d+:\n.*\n\nGoroutine \d+ \((running|finished)\) created at:\n.*==================\nFound \d+ data race\(s\)\n$`)

// TestRaceTSan builds test/race_test.cpp as gocpp build -race builds a program (ThreadSanitizer
// and gocpp/race_atomicity.hpp), with each compiler, and runs its cases: a program with a data
// race ends with exit code 66 and Go's report (also when the race is on an object whose
// reference count both goroutines change), one without ends with 0 and prints nothing, and
// GORACE's options reach TSan. Phase 08's step 8.12.
func TestRaceTSan(t *testing.T) {
	t.Parallel()
	ccs := compilers(t)
	rt := extractRuntime(t)
	for _, c := range ccs {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			t.Parallel()
			flags := []string{"-O1", "-fsanitize=thread", "-fno-omit-frame-pointer", "-DGOCPP_RACE_MODE=1", "-Wl,--export-dynamic-symbol=__tsan_default_options"}
			bin := tsanBuild(t, c, rt, "test/race_test.cpp", flags)
			for _, name := range []string{"reporter", "quiet-handoff", "quiet-pointers", "quiet-free"} {
				t.Run(name, func(t *testing.T) {
					out, code, sig := caseRun(t, bin, name)
					if code != 0 || sig != "" || out != "" {
						t.Errorf("%s: exit code %d %s, output:\n%.3000s\nwant exit code 0 and no output", name, code, sig, out)
					}
				})
			}
			for _, name := range []string{"race-global", "race-through-pointer"} {
				t.Run(name, func(t *testing.T) {
					out, code, sig := caseRun(t, bin, name)
					if code != 66 || sig != "" || !raceReport.MatchString(out) {
						t.Errorf("%s: exit code %d %s, output:\n%.3000s\nwant exit code 66 and Go's report", name, code, sig, out)
					}
				})
			}
			t.Run("race-then-exit", func(t *testing.T) {
				// The report of a race is written before the process ends right after it (_exit: the
				// reporter dies with its parent).
				out, code, sig := caseRun(t, bin, "race-then-exit")
				if (code != 7 && code != 66) || sig != "" || !strings.HasPrefix(out, "==================\nWARNING: DATA RACE\n") {
					t.Errorf("race-then-exit: exit code %d %s, output:\n%.3000s\nwant Go's report and the end of the process", code, sig, out)
				}
			})
			t.Run("GORACE", func(t *testing.T) {
				run := func(gorace string) (string, int) {
					stdout, _, code, err := command(os.TempDir(), append(os.Environ(), "GORACE="+gorace, "TSAN_OPTIONS="), true, bin, "race-global")
					if err != nil {
						t.Fatal(err)
					}
					return string(stdout), code
				}
				if out, code := run("exitcode=9"); code != 9 || !strings.Contains(out, "WARNING: DATA RACE") {
					t.Errorf("GORACE=exitcode=9: exit code %d, output:\n%.2000s\nwant exit code 9 and the report", code, out)
				}
				// halt_on_error stops the process in the callback: the report is Go's all the same
				// (the reporter is another process), without the count, as Go prints it.
				if out, code := run("halt_on_error=1"); code != 66 || !strings.Contains(out, "WARNING: DATA RACE") ||
					strings.Contains(out, "Found") {
					t.Errorf("GORACE=halt_on_error=1: exit code %d, output:\n%.2000s\nwant exit code 66 and Go's report", code, out)
				}
			})
		})
	}
}
