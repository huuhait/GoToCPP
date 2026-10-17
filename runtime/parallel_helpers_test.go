package rtfiles_test

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

// Tests of the helpers of parallel_test.go, without a C++ compiler.

// fakeT records what startBuilds reports, and runs the cleanups when the test says so, as the
// end of a test would.
type fakeT struct {
	mu       sync.Mutex
	errs     []string
	cleanups []func()
}

func (f *fakeT) Helper() {}

func (f *fakeT) Errorf(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errs = append(f.errs, fmt.Sprintf(format, args...))
}

func (f *fakeT) Cleanup(fn func()) { f.cleanups = append(f.cleanups, fn) }

func (f *fakeT) finish() {
	for i := len(f.cleanups) - 1; i >= 0; i-- {
		f.cleanups[i]()
	}
}

// A build nobody asks for (a skipped subtest) still fails the test of the mode, with its
// messages, and a failure that programs share is reported once.
func TestStartBuildsReportsEveryFailure(t *testing.T) {
	shared := errors.New("the runtime's objects: error: boom")
	own := errors.New("a.cpp: error: syntax")
	for round := range 2 { // a second round: nothing is spent by the first
		f := &fakeT{}
		b := startBuilds(f, []prog{{src: "a"}, {src: "b"}, {src: "c"}, {src: "d"}}, func(p prog) buildResult {
			switch p.src {
			case "a":
				return buildResult{own: own}
			case "b", "c":
				return buildResult{shared: shared}
			}
			return buildResult{bin: "/bin/d"}
		})
		if got := b.bin(t, "d"); got != "/bin/d" { // the only program a subtest asks for
			t.Fatalf("round %d: bin(d) = %q", round, got)
		}
		f.finish()
		var nOwn, nShared int
		for _, e := range f.errs {
			switch {
			case strings.Contains(e, own.Error()) && strings.HasPrefix(e, "a "):
				nOwn++
			case strings.Contains(e, shared.Error()):
				nShared++
			default:
				t.Errorf("round %d: unexpected report %q", round, e)
			}
		}
		if nOwn != 1 || nShared != 1 || len(f.errs) != 2 {
			t.Errorf("round %d: reports %q, want one for a and one for the shared failure", round, f.errs)
		}
	}
}

// The cleanup waits for a build that is still running.
func TestStartBuildsWaits(t *testing.T) {
	f := &fakeT{}
	release := make(chan struct{})
	startBuilds(f, []prog{{src: "slow"}}, func(prog) buildResult {
		<-release
		return buildResult{own: errors.New("late failure")}
	})
	done := make(chan struct{})
	go func() { f.finish(); close(done) }()
	select {
	case <-done:
		t.Fatal("the cleanup didn't wait for the build")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	<-done
	if len(f.errs) != 1 || !strings.Contains(f.errs[0], "late failure") {
		t.Errorf("reports %q, want the late failure", f.errs)
	}
}

// Every caller of runtimeObjects gets the compiler's messages when it fails, the first and the
// later ones, and the same error value (so that startBuilds reports it once).
func TestRuntimeObjectsErrorForEveryCaller(t *testing.T) {
	fake, err := exec.LookPath("false")
	if err != nil {
		t.Skipf("no false command: %v", err)
	}
	c := cxx{name: "false", cc: build.CXX{Path: fake}}
	more := []string{"-Dfake-" + t.Name()}
	var errs [3]error
	for i := range errs {
		_, errs[i] = c.runtimeObjects(t.TempDir(), more)
		if errs[i] == nil || !strings.Contains(errs[i].Error(), fake) || !strings.Contains(errs[i].Error(), "exit 1") {
			t.Fatalf("call %d: error %v, want the compiler's failure", i, errs[i])
		}
	}
	if errs[0] != errs[1] || errs[1] != errs[2] {
		t.Errorf("the calls got different errors")
	}
}

// The gate gives a free slot to the highest priority first, the oldest first among equals.
func TestGatePriority(t *testing.T) {
	g := &slots{free: 1}
	g.acquire(0) // the slot is taken
	var mu sync.Mutex
	var order []int
	var wg sync.WaitGroup
	// prio, and an id that tells which of equal priority came first
	for _, w := range []struct{ prio, id int }{{1, 1}, {5, 2}, {3, 3}, {5, 4}, {3, 5}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g.acquire(w.prio)
			mu.Lock()
			order = append(order, w.id)
			mu.Unlock()
			g.release()
		}()
		for { // wait until it is queued, so the order of arrival is known
			g.mu.Lock()
			n := len(g.waiting)
			g.mu.Unlock()
			if n >= w.id {
				break
			}
			time.Sleep(time.Millisecond)
		}
	}
	g.release()
	wg.Wait()
	if want := []int{2, 4, 3, 5, 1}; fmt.Sprint(order) != fmt.Sprint(want) {
		t.Errorf("order %v, want %v", order, want)
	}
	if g.free != 1 || len(g.waiting) != 0 {
		t.Errorf("free %d, waiting %d at the end, want 1 and 0", g.free, len(g.waiting))
	}
}
