package rtfiles_test

import (
	"regexp"
	"strings"
	"testing"
)

// The cases of test/chan_test.cpp that end the process (Phase 08, step 8.3): a goroutine that
// blocks on a nil channel for ever, and the panics of the channel operations, which Go ends with
// exit code 2 and a report. Each is run with the case name as the only argument. The helpers are
// in thread_test.go and cxx_test.go.

// runChan runs the cases of test/chan_test.cpp for a build in the mode named mode.
func runChan(t *testing.T, bin string) {
	// want checks the report of a case against a regular expression and the exit code.
	want := func(name string, exit int, rx string) {
		t.Run(name, func(t *testing.T) {
			out, code, sig := caseRun(t, bin, name)
			if code != exit || sig != "" || !regexp.MustCompile(rx).MatchString(out) {
				t.Errorf("%s: exit code %d %s, output:\n%s\nwant exit code %d and %s", name, code, sig, out, exit, rx)
			}
		})
	}
	// A goroutine blocked on a nil channel is blocked (counted for the deadlock detector), and the
	// program ends when main does.
	want("recv-nil", 0, `^blocked\n$`)
	want("send-nil", 0, `^blocked\n$`)
	// The deferred call of a goroutine's panic wakes main, which returns: the panic still ends the
	// program (exit code 2), as in Go.
	want("panic-defer-wakes-main", 2, `^panic: simple\n\ngoroutine [2-9]\d* \[running\]:\n`)
	// The same, but the deferred call blocks for ever after it woke main: main exits with code 0
	// after the wait (cut to 20 ms), as Go's runtime.main does after it yielded 1000 times.
	want("panic-defer-blocks", 0, `^main returns\n$`)
	// main recovers a panic: the exit has nothing to wait for.
	want("recover-then-exit", 0, `^waits 0\n$`)
	want("go-nil", 2, `^fatal error: go of nil func value\n\ngoroutine 1 \[running\]:\n`)
	want("close-nil", 2, `^panic: close of nil channel\n\ngoroutine 1 \[running\]:\n`)
	want("close-closed", 2, `^panic: close of closed channel\n\ngoroutine 1 \[running\]:\n`)
	want("send-closed", 2, `^panic: send on closed channel\n\ngoroutine 1 \[running\]:\n`)
	// A sender that waits on a channel that is closed panics in its own goroutine.
	want("send-closed-goroutine", 2, `^panic: send on closed channel\n\ngoroutine [2-9]\d* \[running\]:\n`)
	want("recv-no-zero", 2, `^fatal error: receive from a closed channel whose element type has no zero value\n`)
}

// TestChanTSan builds test/chan_test.cpp with ThreadSanitizer, with each compiler, and runs it:
// the unit tests (many senders and receivers on one channel, close racing sends, the hand-off of
// values between goroutines, thread reuse) and the cases that end the process must not print a
// TSan warning, which is a data race in the channel, in the waiter or in the goroutine records.
func TestChanTSan(t *testing.T) {
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
			bin := tsanBuild(t, c, rt, "test/chan_test.cpp", flags)
			t.Run("unit", func(t *testing.T) {
				runUnit(t, bin)
			})
			for _, name := range []string{"recv-nil", "close-closed", "send-closed", "send-closed-goroutine"} {
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
