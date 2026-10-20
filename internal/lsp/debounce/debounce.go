// Package debounce runs work after a quiet time, per key: the language
// server's diagnostics run when a document has not changed for a moment
// (Phase 11 README §2.2 item 7, "debounce, don't delay"), and a newer edit
// cancels the run for the older text.
package debounce

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/safe"
)

// A Clock makes timers: RealClock, or a FakeClock in tests.
type Clock interface {
	// AfterFunc calls f on its own goroutine after d, unless the timer is
	// stopped first (like time.AfterFunc).
	AfterFunc(d time.Duration, f func()) Timer
}

// A Timer is a timer of a Clock.
type Timer interface {
	// Stop stops the timer. It reports false when the timer has fired
	// already (its f has been called or is being called).
	Stop() bool
}

// RealClock is the clock of the time package.
type RealClock struct{}

// AfterFunc is time.AfterFunc.
func (RealClock) AfterFunc(d time.Duration, f func()) Timer { return time.AfterFunc(d, f) }

// A Debouncer runs functions per key after a delay with no newer run for
// the key. It is safe for concurrent use.
//
// The runs of one key never overlap and start in the order of Run: a run
// that is due while the run before it (cancelled by then) still runs
// starts when that one returns. So a newer result is never overwritten by
// an older one that finishes late. Only a running f has a goroutine: a
// key holds at most one running f and one waiting run, however often Run
// is called.
//
// f must return soon after its ctx is done: until it does, the next run of
// its key can't start, and Stop waits for it.
type Debouncer[K comparable] struct {
	clock Clock

	// OnPanic, when set (before the first Run), gets the panic of an f, which
	// then counts as returned. Without it a panic in f ends the process.
	OnPanic func(key K, p *safe.Panic)

	mu      sync.Mutex
	keys    map[K]*entry
	stopped bool
	active  int           // runs that are not over (waiting or running)
	idle    chan struct{} // made by Stop, closed when active is 0
}

// An entry is the state of one key.
type entry struct {
	running *run // the run whose f runs, or nil
	next    *run // the newest run, waiting for its delay or for running; or nil
}

// A run is one call of Run.
type run struct {
	ctx    context.Context
	cancel context.CancelFunc
	f      func(context.Context)
	timer  Timer
	due    bool // the delay is over: it starts when running returns
	over   bool // retired: it ran, or it never will
}

// New returns a debouncer with the clock (RealClock{} outside tests).
func New[K comparable](clock Clock) *Debouncer[K] {
	return &Debouncer[K]{clock: clock, keys: make(map[K]*entry)}
}

// Run calls f(ctx) on a goroutine of its own once delay has passed without
// a newer Run for key (and the run before it has returned). A newer Run
// for key, Cancel(key) or Stop cancels this one: before it starts, f is
// never called; after, ctx is done, and f should return soon. After Stop,
// Run does nothing.
func (d *Debouncer[K]) Run(key K, delay time.Duration, f func(ctx context.Context)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return
	}
	e := d.keys[key]
	if e == nil {
		e = &entry{}
		d.keys[key] = e
	}
	d.cancel(e)
	ctx, cancel := context.WithCancel(context.Background())
	r := &run{ctx: ctx, cancel: cancel, f: f}
	e.next = r
	d.active++
	r.timer = d.clock.AfterFunc(delay, func() { d.fire(key, e, r) })
}

// Cancel cancels the run of key that is waiting or running, if there is
// one. It doesn't wait for f to return.
func (d *Debouncer[K]) Cancel(key K) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if e := d.keys[key]; e != nil {
		d.cancel(e)
		d.forget(key, e)
	}
}

// Stop cancels every run and makes later calls of Run do nothing. Then it
// waits until the running ones have returned, or until ctx is done; it
// reports whether they all returned.
func (d *Debouncer[K]) Stop(ctx context.Context) bool {
	d.mu.Lock()
	d.stopped = true
	for key, e := range d.keys {
		d.cancel(e)
		d.forget(key, e)
	}
	if d.idle == nil {
		d.idle = make(chan struct{})
		if d.active == 0 {
			close(d.idle)
		}
	}
	idle := d.idle
	d.mu.Unlock()
	select {
	case <-idle:
		return true
	case <-ctx.Done():
		return false
	}
}

// Busy reports whether a run is waiting for its delay or running.
func (d *Debouncer[K]) Busy() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.active > 0
}

// Keys returns the keys whose run is waiting or running (for tests and
// logs), in no particular order.
func (d *Debouncer[K]) Keys() []K {
	d.mu.Lock()
	defer d.mu.Unlock()
	keys := make([]K, 0, len(d.keys))
	for k := range d.keys {
		keys = append(keys, k)
	}
	return keys
}

// cancel cancels the waiting run of e (it is over at once) and the
// running one (it is over when its f returns). d.mu is held.
func (d *Debouncer[K]) cancel(e *entry) {
	if r := e.next; r != nil {
		e.next = nil
		r.timer.Stop() // if it fired already, fire sees r.over
		d.retire(r)
	}
	if e.running != nil {
		e.running.cancel()
	}
}

// retire marks r over. d.mu is held.
func (d *Debouncer[K]) retire(r *run) {
	r.cancel()
	r.over = true
	d.active--
	if d.active == 0 && d.idle != nil {
		close(d.idle)
	}
}

// forget drops the entry of key when it holds no run. d.mu is held.
func (d *Debouncer[K]) forget(key K, e *entry) {
	if e.running == nil && e.next == nil && d.keys[key] == e {
		delete(d.keys, key)
	}
}

// fire is the end of the delay of r: it starts r, or marks it due when the
// run before it still runs.
func (d *Debouncer[K]) fire(key K, e *entry, r *run) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if r.over || e.next != r {
		return
	}
	if e.running != nil {
		r.due = true
		return
	}
	d.start(key, e)
}

// start runs e.next, which is due, on a new goroutine. d.mu is held.
func (d *Debouncer[K]) start(key K, e *entry) {
	r := e.next
	e.next, e.running = nil, r
	go func() {
		// A panic in f would end the process, and leave the key running for
		// ever: it goes to OnPanic, and the key is free again.
		if p := safe.Call(func() { r.f(r.ctx) }); p != nil && d.OnPanic != nil {
			d.OnPanic(key, p)
		} else if p != nil {
			panic(p)
		}
		d.mu.Lock()
		defer d.mu.Unlock()
		e.running = nil
		d.retire(r)
		if e.next != nil && e.next.due {
			d.start(key, e)
		}
		d.forget(key, e)
	}()
}

// A FakeClock is a Clock for tests: time moves only by Advance.
type FakeClock struct {
	mu     sync.Mutex
	now    time.Duration
	timers []*fakeTimer
}

type fakeTimer struct {
	c     *FakeClock
	when  time.Duration
	f     func()
	fired bool
}

// AfterFunc adds a timer that fires when Advance reaches now+d.
func (c *FakeClock) AfterFunc(d time.Duration, f func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{c: c, when: c.now + d, f: f}
	c.timers = append(c.timers, t)
	return t
}

// Advance moves the time on by d and calls the f of each timer that is due,
// in the order of their times, on this goroutine (a Debouncer starts its
// own goroutine for a run).
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now += d
	var due, rest []*fakeTimer
	for _, t := range c.timers {
		if t.when <= c.now {
			t.fired = true
			due = append(due, t)
		} else {
			rest = append(rest, t)
		}
	}
	c.timers = rest
	c.mu.Unlock()
	sort.SliceStable(due, func(i, j int) bool { return due[i].when < due[j].when })
	for _, t := range due {
		t.f()
	}
}

// Timers returns the number of timers that have not fired and are not
// stopped.
func (c *FakeClock) Timers() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.timers)
}

func (t *fakeTimer) Stop() bool {
	t.c.mu.Lock()
	defer t.c.mu.Unlock()
	if t.fired {
		return false
	}
	for i, u := range t.c.timers {
		if u == t {
			t.c.timers = append(t.c.timers[:i], t.c.timers[i+1:]...)
			return true
		}
	}
	return false // stopped already
}
