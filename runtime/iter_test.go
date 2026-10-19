package rtfiles_test

import "testing"

// TestIterTSan builds test/iter_test.cpp (the coroutines of package iter: the hand-off between the
// two threads of a coroutine, step 7.10) with ThreadSanitizer, with each compiler, and runs it: it
// must pass and print no TSan warning (a data race in the hand-off word or the coroutine record).
func TestIterTSan(t *testing.T) {
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
			bin := tsanBuild(t, c, rt, "test/iter_test.cpp", flags)
			runUnit(t, bin)
		})
	}
}
