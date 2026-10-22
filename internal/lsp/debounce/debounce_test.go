package debounce

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/safe"
)

// checkGoroutines fails the test if, at its end, more goroutines run than
// at its start (after a few seconds to let them end).
func checkGoroutines(t *testing.T) {
	t.Helper()
	before := runtime.NumGoroutine()
	t.Cleanup(func() {
		deadline := time.Now().Add(time.Minute) // (only ends a hang)
		for runtime.NumGoroutine() > before {
			if time.Now().After(deadline) {
				t.Errorf("%d goroutines at the start, %d at the end", before, runtime.NumGoroutine())
				return
			}
			time.Sleep(time.Millisecond)
		}
	})
}

// stop stops d and fails if a run doesn't return in a minute.
func stop[K comparable](t *testing.T, d *Debouncer[K]) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if !d.Stop(ctx) {
		t.Fatal("Stop: the runs didn't return in a minute")
	}
}

// A recorder records the runs.
type recorder struct {
	mu   sync.Mutex
	runs []string
}

func (r *recorder) f(name string) func(context.Context) {
	return func(context.Context) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.runs = append(r.runs, name)
	}
}

func (r *recorder) get() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.runs)
}

// waitFor waits until cond holds, or fails after a minute.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s: not in a minute", what)
		}
		time.Sleep(100 * time.Microsecond)
	}
}

func TestDelay(t *testing.T) {
	checkGoroutines(t)
	clock := &FakeClock{}
	d := New[string](clock)
	rec := &recorder{}
	d.Run("a", 30*time.Millisecond, rec.f("a1"))
	clock.Advance(29 * time.Millisecond)
	// A newer run before the delay ends replaces it, and the delay
	// starts again.
	d.Run("a", 30*time.Millisecond, rec.f("a2"))
	d.Run("b", 10*time.Millisecond, rec.f("b1"))
	if clock.Timers() != 2 {
		t.Errorf("%d timers, want 2 (a's first one is stopped)", clock.Timers())
	}
	clock.Advance(10 * time.Millisecond)
	waitFor(t, "b1 runs", func() bool { return slices.Equal(rec.get(), []string{"b1"}) })
	clock.Advance(19 * time.Millisecond)
	if got := rec.get(); !slices.Equal(got, []string{"b1"}) {
		t.Errorf("after 29 ms of a2: %v", got)
	}
	clock.Advance(time.Millisecond)
	waitFor(t, "a2 runs", func() bool { return len(rec.get()) == 2 })
	stop(t, d)
	if got := rec.get(); !slices.Equal(got, []string{"b1", "a2"}) {
		t.Errorf("the runs: %v, want b1 a2", got)
	}
	if keys := d.Keys(); len(keys) != 0 {
		t.Errorf("keys left: %v", keys)
	}
}

// TestNoOverlap: a newer run cancels the running one and starts only when
// it has returned.
func TestNoOverlap(t *testing.T) {
	checkGoroutines(t)
	clock := &FakeClock{}
	d := New[int](clock)
	started := make(chan string, 10)
	cancelled := make(chan string, 10)
	gate := make(chan struct{})
	var running atomic.Int32
	slow := func(name string) func(context.Context) {
		return func(ctx context.Context) {
			if running.Add(1) != 1 {
				t.Errorf("%s runs while another run of the key runs", name)
			}
			defer running.Add(-1)
			started <- name
			<-ctx.Done()
			cancelled <- name
			<-gate // returns late, after its cancellation
		}
	}
	d.Run(1, 0, slow("first"))
	clock.Advance(0)
	if got := <-started; got != "first" {
		t.Fatal(got)
	}
	d.Run(1, 5*time.Millisecond, slow("second"))
	if got := <-cancelled; got != "first" {
		t.Fatalf("cancelled %s, want first", got)
	}
	clock.Advance(5 * time.Millisecond) // second is due, but first still runs
	select {
	case got := <-started:
		t.Fatalf("%s started while first runs", got)
	case <-time.After(20 * time.Millisecond):
	}
	if keys := d.Keys(); !slices.Equal(keys, []int{1}) {
		t.Errorf("Keys = %v", keys)
	}
	close(gate)
	if got := <-started; got != "second" {
		t.Fatal(got)
	}
	// Cancel cancels the running one.
	d.Cancel(1)
	if got := <-cancelled; got != "second" {
		t.Fatal(got)
	}
	stop(t, d)
}

func TestCancelAndStop(t *testing.T) {
	checkGoroutines(t)
	clock := &FakeClock{}
	d := New[string](clock)
	rec := &recorder{}
	d.Run("a", time.Millisecond, rec.f("a"))
	d.Cancel("a")
	d.Cancel("a")    // twice: nothing more happens
	d.Cancel("none") // an unknown key
	if clock.Timers() != 0 {
		t.Errorf("%d timers after Cancel", clock.Timers())
	}
	d.Run("b", time.Millisecond, rec.f("b"))
	ctxs := make(chan context.Context, 1)
	d.Run("c", 0, func(ctx context.Context) { ctxs <- ctx; <-ctx.Done() })
	clock.Advance(0)
	ctx := <-ctxs
	stop(t, d) // b never runs; c is cancelled, and Stop waits for it
	if ctx.Err() == nil {
		t.Error("c's context is not done after Stop")
	}
	d.Run("d", 0, rec.f("d")) // after Stop: nothing
	clock.Advance(time.Hour)
	stop(t, d)
	if got := rec.get(); len(got) != 0 {
		t.Errorf("ran %v, want nothing", got)
	}
	if clock.Timers() != 0 {
		t.Errorf("%d timers left", clock.Timers())
	}
}

// TestRealClock runs many keys at once with the real clock (run it with
// -race): the runs of a key never overlap, and the last Run of each key
// runs.
func TestRealClock(t *testing.T) {
	checkGoroutines(t)
	d := New[int](RealClock{})
	const keys, runs = 8, 50
	var mu sync.Mutex
	last := make(map[int]string)
	var running [keys]atomic.Int32
	var wg sync.WaitGroup
	for k := range keys {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range runs {
				name := fmt.Sprintf("%d.%d", k, i)
				d.Run(k, time.Duration(i%3)*100*time.Microsecond, func(ctx context.Context) {
					if running[k].Add(1) != 1 {
						t.Errorf("two runs of key %d at once", k)
					}
					defer running[k].Add(-1)
					select {
					case <-ctx.Done():
					case <-time.After(50 * time.Microsecond):
					}
					mu.Lock()
					last[k] = name
					mu.Unlock()
				})
				if i%7 == 0 {
					time.Sleep(200 * time.Microsecond)
				}
			}
		}()
	}
	wg.Wait()
	waitFor(t, "every key's last run", func() bool { return len(d.Keys()) == 0 })
	stop(t, d)
	for k := range keys {
		if want := fmt.Sprintf("%d.%d", k, runs-1); last[k] != want {
			t.Errorf("the last run of key %d is %q, want %q", k, last[k], want)
		}
	}
}

// TestStuckRun: while a run's f ignores its context, more Runs of the key
// cost no goroutine (one waiting run per key), and Stop with a deadline
// gives up.
func TestStuckRun(t *testing.T) {
	checkGoroutines(t)
	clock := &FakeClock{}
	d := New[string](clock)
	gate := make(chan struct{})
	started := make(chan struct{}, 1)
	var calls atomic.Int32
	d.Run("k", 0, func(context.Context) { started <- struct{}{}; <-gate })
	clock.Advance(0)
	<-started
	before := runtime.NumGoroutine()
	for i := range 1000 {
		d.Run("k", time.Duration(i%3)*time.Millisecond, func(context.Context) { calls.Add(1) })
		d.Run(fmt.Sprint("other", i%4), time.Millisecond, func(context.Context) {})
		clock.Advance(time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > before+8 {
		t.Errorf("%d goroutines before the 1000 Runs, %d after", before, n)
	}
	if clock.Timers() > 1 {
		t.Errorf("%d timers wait", clock.Timers())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if d.Stop(ctx) {
		t.Error("Stop = true while a run is stuck")
	}
	close(gate)
	stop(t, d) // a second Stop waits for the rest
	if calls.Load() != 0 {
		t.Errorf("%d later runs ran after Stop", calls.Load())
	}
}

// TestDueAfterStuck: the last Run of a key runs once the stuck run
// returns, and the ones in between never do.
func TestDueAfterStuck(t *testing.T) {
	checkGoroutines(t)
	clock := &FakeClock{}
	d := New[int](clock)
	gate := make(chan struct{})
	rec := &recorder{}
	d.Run(1, 0, func(context.Context) { <-gate; rec.f("stuck")(nil) })
	clock.Advance(0)
	waitFor(t, "the stuck run starts", func() bool { return clock.Timers() == 0 })
	for i := range 100 {
		d.Run(1, time.Millisecond, rec.f(fmt.Sprint(i)))
		clock.Advance(time.Millisecond)
	}
	close(gate)
	waitFor(t, "the last run", func() bool { return len(rec.get()) == 2 })
	stop(t, d)
	if got := rec.get(); !slices.Equal(got, []string{"stuck", "99"}) {
		t.Errorf("ran %v, want stuck 99", got)
	}
}

// TestOnPanic: a panic in a run goes to OnPanic, and the key is free for the
// next run (it would end the process, and keep the key running for ever).
func TestOnPanic(t *testing.T) {
	checkGoroutines(t)
	clock := &FakeClock{}
	d := New[string](clock)
	var got atomic.Value
	d.OnPanic = func(key string, p *safe.Panic) { got.Store(fmt.Sprintf("%s: %v", key, p.Value)) }
	var rec recorder
	d.Run("k", time.Millisecond, func(context.Context) { panic("boom") })
	clock.Advance(time.Millisecond)
	waitFor(t, "OnPanic", func() bool { return got.Load() != nil })
	if got.Load() != "k: boom" {
		t.Errorf("OnPanic got %v", got.Load())
	}
	d.Run("k", time.Millisecond, rec.f("after"))
	clock.Advance(time.Millisecond)
	waitFor(t, "the run after the panic", func() bool { return len(rec.get()) == 1 })
	stop(t, d)
}
