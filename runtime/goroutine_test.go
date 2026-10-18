package rtfiles_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

// The cases of test/goroutine_test.cpp that end the process (Phase 08, step 8.1): a panic in a
// goroutine, a failed thread start, and stack overflows on goroutine stacks. Each is run with the
// case name as the only argument, and checked against what Go prints and its exit code 2 (the
// stack traces differ from Go's, so they are checked only for the function that panicked, where
// a build keeps its frames). The helpers are in thread_test.go.

// runGoroutine runs the cases of test/goroutine_test.cpp that end the process, for a build in
// the mode named mode ("O0", "O2", "asan"): the frames of the stack trace are exact except at
// -O2, and an overflow is the sanitizer's report in the asan mode (the runtime's handler is
// not installed there).
func runGoroutine(t *testing.T, bin, mode string) {
	frames := mode != "O2"
	asan := mode == "asan"

	// want checks the report of a case against a regular expression and the exit code 2.
	want := func(name, rx string) {
		t.Run(name, func(t *testing.T) {
			out, code, sig := caseRun(t, bin, name)
			if code != 2 || sig != "" || !regexp.MustCompile(rx).MatchString(out) {
				t.Errorf("%s: exit code %d %s, output:\n%s\nwant exit code 2 and %s", name, code, sig, out, rx)
			}
		})
	}

	t.Run("panic", func(t *testing.T) {
		// A goroutine's panic is Go's report, with the goroutine's number (the main goroutine
		// is 1; the first one started has 2, here: gocpp counts them from the main one), after
		// what the goroutine's frames did (destructors ran: the stack was unwound first).
		out, code, sig := caseRun(t, bin, "go-panic")
		rx := regexp.MustCompile(`^main starts\ngoroutine panics\n(?:destructor ran\n){4}` +
			`panic: boom\n\ngoroutine 2 \[running\]:\n`)
		if code != 2 || sig != "" || !rx.MatchString(out) {
			t.Fatalf("exit code %d %s, output:\n%s\nwant exit code 2 and %s", code, sig, out, rx)
		}
		// The goroutine's own frames, innermost first, and none of the runtime's.
		trace := out[rx.FindStringIndex(out)[1]:]
		if frames && !strings.HasPrefix(trace, "(anonymous namespace)::panic_deep(int)\n\t") {
			t.Errorf("the trace of the goroutine starts:\n%.300s", trace)
		}
		if strings.Contains(trace, "gp::") || strings.Contains(trace, "start_thread") || strings.Contains(trace, "clone") {
			t.Errorf("the trace of the goroutine has frames of the runtime or libc:\n%s", trace)
		}
	})
	want("go-panic-nested", `^panic: deep\n\ngoroutine 3 \[running\]:\n`)
	want("go-throw", `^panic: C\+\+ exception std::runtime_error: from a goroutine\n\ngoroutine 2 \[running\]:\n`)
	want("go-fatal", `^fatal error: a fatal error\n\ngoroutine 2 \[running\]:\n`)
	want("go-nil", `^panic: runtime error: invalid memory address or nil pointer dereference\n`+
		`\[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=`+hex+`\]\n\ngoroutine 2 \[running\]:\n`)
	want("main-panic", `^panic: main goroutine\n\ngoroutine 1 \[running\]:\n`)
	want("goexit-main", `^(?:main's deferred call\nother goroutine runs\n|other goroutine runs\nmain's deferred call\n)`+
		`fatal error: no goroutines \(main called runtime.Goexit\) - deadlock!\n$`)
	want("thread-fail-3", `^runtime: failed to create new OS thread \(have 4 already; errno=\d+\)\n`+
		`fatal error: newosproc\n\ngoroutine 1 \[running\]:\n`)
	want("bad-error-method", `^deferred call runs first\nfatal error: panic while printing panic value: inside Error\n\ngoroutine 1 \[running\]:\n`)
	t.Run("exit-in-defer", func(t *testing.T) {
		// os.Exit in a deferred call skips every deferred call that hasn't run, and keeps its code.
		out, code, sig := caseRun(t, bin, "exit-in-defer")
		if code != 3 || sig != "" || out != "exiting\n" {
			t.Errorf("exit code %d %s, output:\n%s\nwant exit code 3 and only \"exiting\"", code, sig, out)
		}
	})
	t.Run("claim-death", func(t *testing.T) {
		out, code, sig := caseRun(t, bin, "claim-death")
		if code != 0 || sig != "" || out != "second thread is blocked\n" {
			t.Errorf("exit code %d %s, output:\n%s\nwant a second thread that stays blocked in claim_death", code, sig, out)
		}
	})
	want("thread-fail", `^before\nruntime: failed to create new OS thread \(have 1 already; errno=\d+\)\n`+
		`fatal error: newosproc\n\ngoroutine 1 \[running\]:\n`)

	t.Run("panic-many", func(t *testing.T) {
		// Eight goroutines panic together: one report, one exit.
		out, code, sig := caseRun(t, bin, "go-panic-many")
		if code != 2 || sig != "" || strings.Count(out, "panic: ") != 1 || strings.Count(out, "goroutine ") != 1 ||
			!regexp.MustCompile(`^panic: [0-7]\n\ngoroutine [2-9] \[running\]:\n`).MatchString(out) {
			t.Errorf("exit code %d %s, output:\n%s\nwant one report of one goroutine's panic and exit code 2", code, sig, out)
		}
	})

	// Stack overflows on goroutine stacks (the thread layer's cases are in thread_test.go): the
	// runtime's handler reports them, with the stack of the thread in the sp= line.
	overflowCase(t, bin, "overflow-go", 8<<20, asan)
	overflowCase(t, bin, "overflow-go-small", 256<<10, asan)
	overflowCase(t, bin, "overflow-go-big-frames", 1<<20, asan)
	overflowCase(t, bin, "overflow-go-many", 256<<10, asan)
	hookCase(t, bin, "overflow-hook-go", asan)
	crashCase(t, bin, "segv-go", asan)
}

// TestGoroutineTSan builds test/goroutine_test.cpp with ThreadSanitizer, with each compiler, and
// runs it: the unit tests and the cases that end the process, which must not print a TSan
// warning (a data race in the waiter or the goroutine records). Phase 08's step 8.12 turns TSan
// into a mode of the whole concurrency suite; this one is for the runtime's first goroutine code.
func TestGoroutineTSan(t *testing.T) {
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
			bin := tsanBuild(t, c, rt, "test/goroutine_test.cpp", flags)
			t.Run("unit", func(t *testing.T) {
				runUnit(t, bin)
			})
			for _, name := range []string{"go-panic", "go-panic-many", "go-throw", "go-fatal", "main-panic", "thread-fail"} {
				t.Run(name, func(t *testing.T) {
					out, code, sig := caseRun(t, bin, name)
					if code != 2 || sig != "" || bytes.Contains([]byte(out), []byte("ThreadSanitizer")) {
						t.Errorf("%s: exit code %d %s, output:\n%.3000s\nwant exit code 2 and no TSan warning", name, code, sig, out)
					}
				})
			}
		})
	}
}
