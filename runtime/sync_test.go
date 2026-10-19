package rtfiles_test

import (
	"regexp"
	"strings"
	"testing"
)

// The cases of test/sync_test.cpp that end the process (Phase 08, step 8.7): the fatal errors of
// package sync, which Go ends with exit code 2 and a report, that no recover stops. Each is run
// with the case name as the only argument. The helpers are in thread_test.go and cxx_test.go.

// runSync runs the cases of test/sync_test.cpp.
func runSync(t *testing.T, bin string) {
	want := func(name string, exit int, rx string) {
		t.Run(name, func(t *testing.T) {
			out, code, sig := caseRun(t, bin, name)
			if code != exit || sig != "" || !regexp.MustCompile(rx).MatchString(out) {
				t.Errorf("%s: exit code %d %s, output:\n%s\nwant exit code %d and %s", name, code, sig, out, exit, rx)
			}
		})
	}
	want("unlock-unlocked", 2, `^fatal error: sync: unlock of unlocked mutex\n\ngoroutine 1 \[running\]:\n`)
	// The fatal error is not an exception: nothing catches it ("caught" is never printed).
	want("unlock-unlocked-recover", 2, `^fatal error: sync: unlock of unlocked mutex\n`)
	want("unlock-twice", 2, `^fatal error: sync: unlock of unlocked mutex\n`)
	want("runlock-unlocked", 2, `^fatal error: sync: RUnlock of unlocked RWMutex\n`)
	want("rwunlock-unlocked", 2, `^fatal error: sync: Unlock of unlocked RWMutex\n`)
	want("negative-counter", 2, `^panic: sync: negative WaitGroup counter\n\ngoroutine 1 \[running\]:\n`)
	// A goroutine blocked on a mutex that nothing unlocks: counted as blocked, and the program ends
	// when main does.
	want("blocked-forever", 0, `^blocked\n$`)
}

// TestSyncTSan builds test/sync_test.cpp with ThreadSanitizer, with each compiler, and runs it:
// the unit tests (mutual exclusion, hand-off, the readers and writers of an RWMutex, the
// WaitGroup, the Cond, the atomics, atomic_ptr) and the cases that end the process must not print a
// TSan warning, which is a data race in the algorithms (a missing happens-before edge between an
// Unlock and the next Lock, a Wait and the Signal that woke it, a Done and the Wait that follows).
func TestSyncTSan(t *testing.T) {
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
			bin := tsanBuild(t, c, rt, "test/sync_test.cpp", flags)
			t.Run("unit", func(t *testing.T) {
				runUnit(t, bin)
			})
			for _, name := range []string{"unlock-unlocked", "negative-counter", "blocked-forever"} {
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
