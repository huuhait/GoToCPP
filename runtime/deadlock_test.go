package rtfiles_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The cases of test/deadlock_test.cpp that end the process (Phase 08, step 8.11): the deadlock
// detector's report, which Go ends with exit code 2, and the programs in which nothing is a
// deadlock (a timer that can fire, a sleeping goroutine, a thread of C code that entered gocpp).
// Each is run with the case name as the only argument. The helpers are in thread_test.go and
// cxx_test.go.

// A deadlockCase is a case of test/deadlock_test.cpp, the exit code and the pattern that the
// program's output (stdout, then stderr) must match.
type deadlockCase struct {
	name string
	exit int
	rx   string
}

const (
	dlAsleep = `fatal error: all goroutines are asleep - deadlock!\n`
	dlNone   = `fatal error: no goroutines \(main called runtime\.Goexit\) - deadlock!\n`
)

// deadlockCases: Go's report: the message, a blank line, then every goroutine with its reason, its
// stack and, for a goroutine, where it came from. The same cases run on Windows (windows_test.go).
var deadlockCases = []deadlockCase{
	{"chan", 2, `^` + dlAsleep + `\ngoroutine 1 \[chan receive\]:\n(?:.+\n\t.+:\d+ \+0x[0-9a-f]+\n)+$`},
	{"goroutines", 2, `(?s)^` + dlAsleep + `\ngoroutine 1 \[chan receive\]:\n.*\n\n` +
		`goroutine 2 \[chan receive \(nil chan\)\]:\n.*created by .* in goroutine 1\n\t.*:\d+ \+0x[0-9a-f]+\n\n` +
		`goroutine 3 \[chan send\]:\n.*created by .* in goroutine 1\n\t.*:\d+ \+0x[0-9a-f]+\n$`},
	// main called Goexit: no trace when no goroutine is left; the report of the goroutines that
	// are left when they are all asleep (main is not among them).
	{"goexit-none", 2, `^the goroutine runs\n` + dlNone + `$`},
	{"goexit-ended", 2, `^the goroutine ran\n` + dlNone + `$`},
	{"goexit-asleep-late", 2, `(?s)^` + dlAsleep + `\ngoroutine 2 \[chan receive\]:\n.*created by .*\n\t.*\n$`},
	{"goexit-late", 2, `^slept\n` + dlNone + `$`},
	{"goexit-afterfunc", 2, `^fired\n` + dlNone + `$`},
	{"goexit-asleep", 2, `(?s)^` + dlAsleep + `\ngoroutine 2 \[chan receive\]:\n.*created by .*\n\t.*\n$`},
	{"ticker", 2, `^` + dlAsleep + `\ngoroutine 1 \[chan receive\]:\n`},
	// Not a deadlock: a goroutine that sleeps, an armed AfterFunc, a timer that main waits on, a
	// thread of C code that entered gocpp (or a callback that C could call), main that returns.
	{"sleeping", 0, `^woke\n$`},
	{"afterfunc", 0, `^fired\n$`},
	{"timer-wait", 0, `^fired\n$`},
	{"foreign", 0, `^survived\n$`},
	{"callback", 0, `^survived\n$`},
	{"callback-fn", 0, `^survived\n$`},
	{"callback-fnval", 0, `^survived\n$`},
	{"main-returns", 0, `^main returns\n$`},
	// The report of the other goroutines with the barrier of run_main, and without it (the system
	// call is made to fail): the dump lists the goroutine either way.
	{"init-dump", 0, `^ok\n$`},
	{"noinit-dump", 0, `^ok\n$`},
}

// runDeadlock runs the cases of test/deadlock_test.cpp.
func runDeadlock(t *testing.T, bin string) {
	runTraceback(t, bin)
	for _, c := range deadlockCases {
		t.Run(c.name, func(t *testing.T) {
			out, code, sig := caseRun(t, bin, c.name)
			if code != c.exit || sig != "" || !regexp.MustCompile(c.rx).MatchString(out) {
				t.Errorf("%s: exit code %d %s, output:\n%s\nwant exit code %d and %s", c.name, code, sig, out, c.exit, c.rx)
			}
		})
	}
}

// A tracebackCase is a case of test/deadlock_test.cpp run with GOTRACEBACK set: a goroutine waits
// on a channel and one sleeps, then the main goroutine panics (tb-panic) or ends in the fatal
// error of an unlocked mutex (tb-fatal). Go 1.25.7 prints the other goroutines for "all" and for
// a number from 1 (not for the default and for "single"), and nothing but the text for "none"
// and "0". The reasons are Go's: chan receive, sleep.
type tracebackCase struct {
	name string
	env  string
	exit int
	rx   string
}

const (
	tbPanic = `^panic: boom\n\ngoroutine 1 \[running\]:\n`
	tbFatal = `^fatal error: sync: unlock of unlocked mutex\n\ngoroutine 1 \[running\]:\n`
	tbOther = `(?s).*\ngoroutine \d+ \[chan receive\]:\n.*\ngoroutine \d+ \[sleep\]:\n.*`
)

var tracebackCases = []tracebackCase{
	{"tb-panic", "GOTRACEBACK=all", 2, tbPanic + tbOther},
	{"tb-panic", "GOTRACEBACK=1", 2, tbPanic + tbOther},
	{"tb-panic", "GOTRACEBACK=system", 2, tbPanic + tbOther},
	{"tb-panic", "GOTRACEBACK=crash", 2, tbPanic + tbOther},
	{"tb-panic", "GOTRACEBACK=single", 2, tbPanic},
	{"tb-panic", "GOTRACEBACK=none", 2, `^panic: boom\n$`},
	{"tb-panic", "GOTRACEBACK=0", 2, `^panic: boom\n$`},
	{"tb-fatal", "GOTRACEBACK=all", 2, tbFatal + tbOther},
	{"tb-fatal", "GOTRACEBACK=none", 2, `^fatal error: sync: unlock of unlocked mutex\n$`},
	// The deadlock report is a throw: Go prints all goroutines for the default and for "single"
	// (the cases above, in deadlockCases), and only the line for level 0: none, 0, and a word that
	// Go does not know (wer is one on Linux).
	{"goroutines", "GOTRACEBACK=none", 2, `^` + dlAsleep + `$`},
	{"goroutines", "GOTRACEBACK=0", 2, `^` + dlAsleep + `$`},
	{"goroutines", "GOTRACEBACK=wer", 2, `^` + dlAsleep + `$`},
	{"goroutines", "GOTRACEBACK=banana", 2, `^` + dlAsleep + `$`},
	{"goroutines", "GOTRACEBACK=single", 2, `(?s)^` + dlAsleep + `\ngoroutine 1 \[chan receive\]:\n.*goroutine 3 \[chan send\]:\n`},
}

// runTraceback runs the traceback cases of test/deadlock_test.cpp. The programs print their report
// to stderr, which the combined output has after the stdout (nothing is printed there).
func runTraceback(t *testing.T, bin string) {
	for _, c := range tracebackCases {
		t.Run(c.name+"/"+c.env, func(t *testing.T) {
			env := append(os.Environ(), c.env)
			b, _, code, err := command(filepath.Dir(bin), env, true, bin, c.name)
			if err != nil && code == -1 {
				t.Fatalf("%s: %v", c.name, err)
			}
			out := string(b)
			if code != c.exit || !regexp.MustCompile(c.rx).MatchString(out) {
				t.Errorf("%s %s: exit code %d, output:\n%s\nwant exit code %d and %s", c.name, c.env, code, out, c.exit, c.rx)
			}
			if strings.HasPrefix(c.name, "tb-") && (strings.Contains(c.env, "single") || strings.Contains(c.env, "none") || strings.Contains(c.env, "=0")) {
				if strings.Contains(out, "[chan receive]") || strings.Contains(out, "[sleep]") {
					t.Errorf("%s %s: the output has the other goroutines:\n%s", c.name, c.env, out)
				}
			}
		})
	}
}

// TestDeadlockTSan builds test/deadlock_test.cpp with ThreadSanitizer, with each compiler, and runs
// it: the registry (a worker publishes the record of its goroutine, a detector reads it), the
// capture of stacks on the parked threads and the counts must not race.
func TestDeadlockTSan(t *testing.T) {
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
			bin := tsanBuild(t, c, rt, "test/deadlock_test.cpp", flags)
			t.Run("unit", func(t *testing.T) {
				runUnit(t, bin)
			})
			for _, name := range []string{"goroutines", "goexit-asleep", "afterfunc", "foreign"} {
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
