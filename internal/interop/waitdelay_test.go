package interop

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

// setWaitDelays sets waitDelay and retryWaitDelay for the rest of the test.
func setWaitDelays(t *testing.T, delay, retry time.Duration) {
	t.Helper()
	oldDelay, oldRetry := waitDelay, retryWaitDelay
	waitDelay, retryWaitDelay = delay, retry
	t.Cleanup(func() { waitDelay, retryWaitDelay = oldDelay, oldRetry })
}

// A probe whose compiler exited with a success status while a process that it started held the
// output open must not use the output (it is data, and may be cut): it runs the compiler again.
func TestProbeOutputOpenRetries(t *testing.T) {
	setWaitDelays(t, time.Millisecond, 10*time.Second)
	// The first run prints a part and leaves a process that holds the output; the second is
	// complete.
	c := fakeCompiler(t, build.GCC, `echo run >> "$0.log"
if [ ! -e "$0.first" ]; then : > "$0.first"; sleep 20 & echo first; exit 0; fi
echo second`)
	var out, errOut capBuffer
	out.max, errOut.max = 1<<10, 1<<10
	out.stop, errOut.stop = func() {}, func() {}
	start := time.Now()
	cmd, err := runCompiler(context.Background(), c, "", t.TempDir(), nil, &out, &errOut)
	if err := runFailure(context.Background(), "fake", cmd, err, nil); err != nil {
		t.Fatalf("runFailure: %v", err)
	}
	if got := out.buf.String(); got != "second\n" {
		t.Errorf("output %q, want only the second run's", got)
	}
	if runs, _ := os.ReadFile(c.Path + ".log"); strings.Count(string(runs), "\n") != 2 {
		t.Errorf("the compiler ran %q, want 2 runs", runs)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("took %v", d)
	}
}

// A process that holds the output on every run is an error after the second run, not a hang.
func TestProbeOutputOpenGivesUp(t *testing.T) {
	setWaitDelays(t, time.Millisecond, 20*time.Millisecond)
	c := fakeCompiler(t, build.GCC, "echo run >> \"$0.log\"\nsleep 20 &\necho out")
	out := &capBuffer{max: 1 << 10, stop: func() {}}
	start := time.Now()
	cmd, err := runCompiler(context.Background(), c, "", t.TempDir(), nil, out, out)
	if !errors.Is(err, exec.ErrWaitDelay) {
		t.Fatalf("err = %v, want ErrWaitDelay", err)
	}
	if err := runFailure(context.Background(), "fake", cmd, err, nil); err == nil || !strings.Contains(err.Error(), "kept its output open") {
		t.Errorf("runFailure = %v", err)
	}
	if runs, _ := os.ReadFile(c.Path + ".log"); strings.Count(string(runs), "\n") != 2 {
		t.Errorf("the compiler ran %q, want 2 runs", runs)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("took %v", d)
	}
}
