package probe

import (
	"context"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
)

// EnvJobs names the environment variable that sets how many compiler runs of the
// probes may be going at the same time in this process (a positive number).
const EnvJobs = "GOCPP_PROBE_JOBS"

// Jobs returns the bound on the compiler runs of the probes that run at the same time:
// GOCPP_PROBE_JOBS, else min(8, NumCPU/2) and at least 1 (a build: its compiler jobs and the
// program's own work share the machine), or min(8, NumCPU) in a language server (SetInteractive).
// The bound is global to the process: the probe batches of Runner.Probe, the declaration runs of
// Runner.Decls and the rounds of Analyze all draw from it, so that a program with many names
// never starts more compilers than the bound, however the calls overlap.
func Jobs() int {
	if n, err := strconv.Atoi(os.Getenv(EnvJobs)); err == nil && n > 0 {
		return n
	}
	if interactive.Load() {
		return max(1, min(8, runtime.NumCPU()))
	}
	return max(1, min(8, runtime.NumCPU()/2))
}

// interactive is set in a language server, whose compiler runs are background work for answers
// that nobody waits for, and whose own work between the edits is small.
var interactive atomic.Bool

// SetInteractive makes the default of Jobs the one of a language server: all the cores, up to 8.
// The language server's cold C/C++ data is a chain of runs, and it is under 2 s only when eight
// of them can go at once (ADR-126); on eight pinned cores that cost no keystroke latency.
func SetInteractive(on bool) { interactive.Store(on) }

// A limiter is a counting semaphore whose bound is read at each acquisition
// (tests change the environment while they run).
type limiter struct {
	mu     sync.Mutex
	active int
	peak   int           // the largest value of active so far (tests)
	wake   chan struct{} // closed when a slot is released
}

// gate is the process's bound on compiler runs.
var gate = &limiter{wake: make(chan struct{})}

// acquire waits for a free slot and returns the function that frees it, or the
// context's error.
func (l *limiter) acquire(ctx context.Context) (func(), error) {
	for {
		l.mu.Lock()
		if l.active < Jobs() {
			l.active++
			l.peak = max(l.peak, l.active)
			l.mu.Unlock()
			var once sync.Once
			return func() {
				once.Do(func() {
					l.mu.Lock()
					l.active--
					close(l.wake)
					l.wake = make(chan struct{})
					l.mu.Unlock()
				})
			}, nil
		}
		wake := l.wake
		l.mu.Unlock()
		select {
		case <-wake:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// running returns the number of compiler runs going now and the largest number
// that were going at once since resetPeak.
func (l *limiter) running() (now, peak int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.active, l.peak
}

func (l *limiter) resetPeak() {
	l.mu.Lock()
	l.peak = l.active
	l.mu.Unlock()
}
