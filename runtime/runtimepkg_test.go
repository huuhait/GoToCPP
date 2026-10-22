package rtfiles_test

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
)

// The cases of test/runtimepkg_test.cpp that end the process (Phase 08, steps 8.9 and 8.10): the
// exit path of gp::exit and the return of main. Each is run with the case name as the only
// argument; stdout and stderr are pipes, so C stdio is fully buffered. The helpers are in
// thread_test.go and cxx_test.go.

// runRuntimePkg runs the cases for a build in the mode named by asan (the leak check of the
// address sanitizer runs before _exit, and finds a leak).
func runRuntimePkg(t *testing.T, bin string, asan bool) {
	run := func(name string) (stdout, stderr string, code int) {
		t.Helper()
		o, e, code, err := command(bin[:strings.LastIndex(bin, "/")], nil, false, bin, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return string(o), string(e), code
	}
	t.Run("exit-printf", func(t *testing.T) {
		o, e, code := run("exit-printf")
		if code != 5 || o != "gocpp line\nc line without a newline" || e != "c error line\n" {
			t.Errorf("exit code %d, stdout %q, stderr %q; want 5, all the output flushed in order", code, o, e)
		}
	})
	t.Run("exit-busy", func(t *testing.T) {
		o, e, code := run("exit-busy")
		if code != 0 || o != "exiting\n" || e != "" {
			t.Errorf("exit code %d, stdout %q, stderr %q", code, o, e)
		}
	})
	t.Run("exit-many", func(t *testing.T) {
		o, e, code := run("exit-many")
		if code != 0 || o != "exiting\n" || e != "" {
			t.Errorf("exit code %d, stdout %q, stderr %q", code, o, e)
		}
	})
	t.Run("exit-code", func(t *testing.T) {
		_, _, code := run("exit-code")
		if code != 42 {
			t.Errorf("exit code %d, want 42", code)
		}
	})
	t.Run("exit-twice", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			o, e, code := run("exit-twice")
			if code < 10 || code > 13 || o != "" || e != "" {
				t.Errorf("exit code %d, stdout %q, stderr %q; want one of 10 to 13 and nothing printed", code, o, e)
			}
		}
	})
	// Like Go's os.Exit and the return of main: no static destructor, no atexit function, no
	// thread-local destructor (nothing but the program's own output).
	for _, name := range []string{"exit-atexit", "return-atexit"} {
		t.Run(name, func(t *testing.T) {
			o, e, code := run(name)
			if code != 0 || o != "main\n" || e != "" {
				t.Errorf("exit code %d, stdout %q, stderr %q; want 0, \"main\\n\" and nothing else", code, o, e)
			}
		})
	}
	t.Run("exit-claimed", func(t *testing.T) {
		// exit waits for the thread that claimed the end of the program: its code wins.
		o, e, code := run("exit-claimed")
		if code != 11 || o != "claimer ends\n" || e != "" {
			t.Errorf("exit code %d, stdout %q, stderr %q; want 11 and \"claimer ends\\n\"", code, o, e)
		}
	})
	t.Run("exit-guard", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("needs sched_setaffinity and SCHED_IDLE")
		}
		// The second exit waits for the first: the code of the one that got the guard.
		o, e, code := run("exit-guard")
		if code != 11 || o != "" || e != "" {
			t.Errorf("exit code %d, stdout %q, stderr %q; want 11 (22 is the second exit getting past the guard)", code, o, e)
		}
	})
	t.Run("return-main", func(t *testing.T) {
		o, e, code := run("return-main")
		if code != 0 || o != "main returns\n" || e != "" {
			t.Errorf("exit code %d, stdout %q, stderr %q", code, o, e)
		}
	})
	t.Run("exit-clean", func(t *testing.T) {
		o, e, code := run("exit-clean")
		if code != 0 || o != "clean\n" || e != "" {
			t.Errorf("exit code %d, stdout %q, stderr %q", code, o, e)
		}
	})
	t.Run("exit-leak", func(t *testing.T) {
		o, e, code := run("exit-leak")
		if o != "leaked\n" {
			t.Errorf("stdout %q", o)
		}
		leaked := bytes.Contains([]byte(e), []byte("LeakSanitizer: detected memory leaks"))
		switch {
		case asan && (!leaked || code == 0):
			t.Errorf("exit code %d, stderr:\n%.600s\nwant LeakSanitizer's report and a failing exit code: _exit must not skip the leak check", code, e)
		case !asan && (leaked || code != 0):
			t.Errorf("exit code %d, stderr %q; want a silent exit 0", code, e)
		}
	})
}

// TestRuntimePkgTSan builds test/runtimepkg_test.cpp with ThreadSanitizer, with each compiler:
// the unit tests and the exit cases must not print a TSan warning (a data race in the exit path,
// GOMAXPROCS or the table of the stack frames).
func TestRuntimePkgTSan(t *testing.T) {
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
			bin := tsanBuild(t, c, rt, "test/runtimepkg_test.cpp", flags)
			t.Run("unit", func(t *testing.T) {
				runUnit(t, bin)
			})
			for _, name := range []string{"exit-printf", "exit-busy", "exit-many", "exit-twice", "return-main"} {
				t.Run(name, func(t *testing.T) {
					out, _, sig := caseRun(t, bin, name)
					if sig != "" || strings.Contains(out, "ThreadSanitizer") {
						t.Errorf("%s: %s, output:\n%.3000s\nwant no TSan warning", name, sig, out)
					}
				})
			}
		})
	}
}
