package main

import (
	"fmt"
	"strings"
)

// A Mode is one way to build and run the programs under sanitizers.
type Mode struct {
	Name string
	// Kinds are the reports the mode can make; a canary of another kind does not run in it.
	Kinds []Kind
	// BuildArgs go to gocpp build; BuildEnv is added to its environment.
	BuildArgs []string
	BuildEnv  []string
	// RunEnv is added to the program's environment.
	RunEnv []string
}

// ubsanFlags build the program with UBSan alone, with the real allocator: nothing of ASan's
// layout changes, and a report ends the program. (The recover flag makes the compiler call the
// aborting handler; UBSAN_OPTIONS halt_on_error alone would do the same for the -asan build.)
const ubsanFlags = "-fsanitize=undefined -fno-sanitize-recover=undefined -fno-omit-frame-pointer"

// The modes. "asan" is gocpp's own -asan build (AddressSanitizer, LeakSanitizer and
// UBSan, what make ci-sanitize runs), with the options of a nightly: leaks on and every report
// stops the program. "ubsan" is UBSan alone, "tsan" is gocpp -race (ThreadSanitizer with Go's
// report; gocpp's runtime defaults: exit status 66, detect_deadlocks=0).
func modes() map[string]Mode {
	return map[string]Mode{
		"asan": {
			Name:      "asan",
			Kinds:     []Kind{ASan, UBSan, LSan},
			BuildArgs: []string{"-asan"},
			RunEnv: []string{
				"ASAN_OPTIONS=detect_leaks=1:halt_on_error=1:abort_on_error=0:print_summary=1:symbolize=1",
				"UBSAN_OPTIONS=halt_on_error=1:print_stacktrace=1:print_summary=1",
			},
		},
		"ubsan": {
			Name:     "ubsan",
			Kinds:    []Kind{UBSan},
			BuildEnv: []string{"GOCPP_CFLAGS=" + ubsanFlags, "GOCPP_LDFLAGS=-fsanitize=undefined"},
			RunEnv:   []string{"UBSAN_OPTIONS=halt_on_error=1:print_stacktrace=1:print_summary=1"},
		},
		"tsan": {
			Name:      "tsan",
			Kinds:     []Kind{TSan},
			BuildArgs: []string{"-race"},
		},
	}
}

// ModeNames lists the modes.
func ModeNames() []string { return []string{"asan", "ubsan", "tsan"} }

// ModeByName returns the mode name.
func ModeByName(name string) (Mode, error) {
	m, ok := modes()[name]
	if !ok {
		return Mode{}, fmt.Errorf("unknown mode %q (want %s)", name, strings.Join(ModeNames(), ", "))
	}
	return m, nil
}

// Has reports whether the mode can make reports of kind k.
func (m Mode) Has(k Kind) bool {
	for _, x := range m.Kinds {
		if x == k {
			return true
		}
	}
	return false
}
