package tests

import (
	"runtime"
	"slices"
	"testing"
)

// overflowFormat is the format of tests/overflow: a run test (an // Output: block, // ExitCode: and
// // Stderr: lines). See tests/overflow/README.md.
var overflowFormat = format{"overflow", outputLine | exitCodeLine | stderrLine | goBuildLine, true}

// TestStackOverflow runs the programs of tests/overflow: endless recursion on the main goroutine and
// in a goroutine must end with Go's "fatal error: stack overflow" and exit code 2, not with a bare
// SIGSEGV. Under a sanitizer (GOCPP_FLAGS=-asan or -race) the overflow is the sanitizer's report
// instead of gocpp's (the runtime's own tests, runtime/goroutine_test.go, cover that mode), so the
// test is skipped there.
func TestStackOverflow(t *testing.T) {
	c := setup(t)
	for _, f := range []string{"-asan", "-race"} {
		if slices.Contains(c.flags, f) {
			t.Skipf("GOCPP_FLAGS has %s: the sanitizer reports the overflow, not gocpp's handler", f)
		}
	}
	c.runPrograms(t, "tests/overflow", loadTests(t, overflowFormat), func(t *testing.T, p *program) {
		want := expect{
			stdout:   p.header.output,
			from:     "the // Output: block",
			exitCode: p.header.exitCode,
			stderr:   p.header.stderr,
		}
		c.forEachCompiler(t, func(t *testing.T, cc compiler) {
			checkRun(t, c.run(t, lowStackLimit(gocppJob(p.dir, runEnv(p.header), cc.cxx, p.header.phase, c.runArgs(p.arg)...))), want)
		})
	})
}

// lowStackLimit runs the job under a hard RLIMIT_STACK of 64 MiB (Linux; elsewhere as it is). The
// runtime raises the main thread's soft limit to 1 GiB, Go's limit, at most the hard limit (the
// runtime's thread layer), so the endless recursion of overflow_main ends after 64 MiB of stack,
// not after touching 1 GiB. runtime/thread_test.go checks the overflow at the 1 GiB limit itself
// without the recursion.
func lowStackLimit(j job) job {
	if runtime.GOOS != "linux" {
		return j
	}
	j.args = append([]string{"-c", `ulimit -S -s 8192 && ulimit -H -s 65536 && exec "$0" "$@"`, j.name}, j.args...)
	j.name = "sh"
	j.show = "(ulimit -H -s 65536; " + j.show + ")"
	return j
}
