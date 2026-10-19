package rtfiles_test

import "testing"

// TestTimeTSan builds test/time_test.cpp with ThreadSanitizer, with each compiler, and runs it: the
// timer thread, the goroutines that arm, stop, reset and receive on their timers at the same time,
// and the AfterFunc goroutines must not print a TSan warning, which would be a data race in the
// timer heap, in a timer record or in a channel that a timer sends on (Phase 08, step 8.8).
func TestTimeTSan(t *testing.T) {
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
			bin := tsanBuild(t, c, rt, "test/time_test.cpp", flags)
			// (runUnit fails on any output on stderr, where TSan writes its warnings)
			runUnit(t, bin)
		})
	}
}
