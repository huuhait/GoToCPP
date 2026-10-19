package build

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The tests of the executor (executor.go) count operations: how many actions run at the same
// time, in which order they start, in which order their text and their errors come out. None of
// them reads a clock to decide.

// recorder makes the actions of a test: each records when it starts and ends.
type recorder struct {
	mu      sync.Mutex
	started []int
	flushed []int
	events  []string // "start 0", "flush 0": in the order they happened
	running int
	peak    int
}

func (r *recorder) action(i int, work func() error) action {
	return action{name: fmt.Sprint(i), run: func() (func(), error) {
		r.mu.Lock()
		r.started = append(r.started, i)
		r.events = append(r.events, fmt.Sprint("start ", i))
		r.running++
		r.peak = max(r.peak, r.running)
		r.mu.Unlock()
		err := work()
		r.mu.Lock()
		r.running--
		r.mu.Unlock()
		return func() { // called under the executor's lock
			r.mu.Lock()
			defer r.mu.Unlock()
			r.flushed = append(r.flushed, i)
			r.events = append(r.events, fmt.Sprint("flush ", i))
		}, err
	}}
}

func TestRunActionsJobsOne(t *testing.T) {
	var r recorder
	var acts []action
	for i := range 20 {
		acts = append(acts, r.action(i, func() error { return nil }))
	}
	b := &Builder{Jobs: 1}
	errs, notStarted := b.runActions(context.Background(), acts)
	if notStarted != 0 || slices.ContainsFunc(errs, func(e error) bool { return e != nil }) {
		t.Fatalf("errs %v, notStarted %d", errs, notStarted)
	}
	want := make([]int, 20)
	for i := range want {
		want[i] = i
	}
	if !slices.Equal(r.started, want) || !slices.Equal(r.flushed, want) || r.peak != 1 {
		t.Errorf("-p 1: started %v, flushed %v, peak %d; want in order, one at a time", r.started, r.flushed, r.peak)
	}
	// The text of an action comes out as soon as it can (before the next action starts), not at the end.
	for i := range 20 {
		if got := r.events[2*i : 2*i+2]; got[0] != fmt.Sprint("start ", i) || got[1] != fmt.Sprint("flush ", i) {
			t.Fatalf("the events %v: action %d should start and be flushed before the next starts", r.events, i)
		}
	}
}

// With N jobs, no more than N actions run at the same time, and N of them do run at the same time
// (a barrier that opens when N have started proves it: if fewer than N ran at once, the test
// would never finish, so there is a timeout only to turn that into a failure, not to decide).
func TestRunActionsBound(t *testing.T) {
	for _, jobs := range []int{2, 3, 5} {
		t.Run(fmt.Sprint(jobs), func(t *testing.T) {
			var r recorder
			var arrived atomic.Int32
			gate := make(chan struct{})
			var acts []action
			for i := range 4 * jobs {
				acts = append(acts, r.action(i, func() error {
					if i < jobs { // the first wave waits for the whole wave
						if arrived.Add(1) == int32(jobs) {
							close(gate)
						}
						select {
						case <-gate:
						case <-time.After(30 * time.Second):
							return errors.New("the first wave never ran at the same time")
						}
					}
					return nil
				}))
			}
			b := &Builder{Jobs: jobs}
			errs, _ := b.runActions(context.Background(), acts)
			for _, err := range errs {
				if err != nil {
					t.Fatal(err)
				}
			}
			if r.peak != jobs {
				t.Errorf("peak %d, want %d", r.peak, jobs)
			}
			// The text comes in the order of the list, whatever the order of the ends.
			for i, v := range r.flushed {
				if v != i {
					t.Fatalf("flushed %v, want 0..%d in order", r.flushed, 4*jobs-1)
				}
			}
		})
	}
}

// The text of an action comes out only after the text of all the actions before it, even when the
// later ones end first: action 0 waits until 1 and 2 have done their work.
func TestRunActionsFlushOrder(t *testing.T) {
	var r recorder
	one, two := make(chan struct{}), make(chan struct{})
	wait := func(c chan struct{}) error {
		select {
		case <-c:
			return nil
		case <-time.After(30 * time.Second):
			return errors.New("the other actions never ran at the same time")
		}
	}
	acts := []action{
		r.action(0, func() error {
			if err := wait(one); err != nil {
				return err
			}
			return wait(two)
		}),
		r.action(1, func() error { close(one); return nil }),
		r.action(2, func() error { close(two); return nil }),
	}
	b := &Builder{Jobs: 3}
	errs, _ := b.runActions(context.Background(), acts)
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(r.flushed, []int{0, 1, 2}) {
		t.Errorf("flushed %v, want 0 1 2", r.flushed)
	}
}

// Every failure is returned, in the order of the list; the actions after a failure still run.
func TestRunActionsErrorsInOrder(t *testing.T) {
	var r recorder
	var acts []action
	for i := range 10 {
		acts = append(acts, r.action(i, func() error {
			if i%3 == 0 {
				return fmt.Errorf("action %d failed", i)
			}
			return nil
		}))
	}
	for _, jobs := range []int{1, 4} {
		b := &Builder{Jobs: jobs}
		errs, notStarted := b.runActions(context.Background(), acts)
		err := joinErrors(notStarted, errs...)
		var multi *Errors
		if !errors.As(err, &multi) || len(multi.Errs) != 4 || notStarted != 0 {
			t.Fatalf("-p %d: error %v, want 4 errors", jobs, err)
		}
		var got []string
		for _, e := range Flatten(err) {
			got = append(got, e.Error())
		}
		want := []string{"action 0 failed", "action 3 failed", "action 6 failed", "action 9 failed"}
		if !slices.Equal(got, want) {
			t.Errorf("-p %d: errors %v, want %v", jobs, got, want)
		}
	}
	// One failure is that error itself, not wrapped.
	one := errors.New("only")
	if got := joinErrors(0, nil, one, nil); got != one {
		t.Errorf("joinErrors of one error = %v", got)
	}
	if got := Flatten(one); len(got) != 1 || got[0] != one || Flatten(nil) != nil {
		t.Errorf("Flatten(one) = %v", got)
	}
}

// After maxFailedActions failures no new action starts, and the number of those that were left
// out is reported.
func TestRunActionsStopsAfterManyFailures(t *testing.T) {
	old := maxFailedActions
	maxFailedActions = 3
	defer func() { maxFailedActions = old }()
	var r recorder
	var acts []action
	for i := range 10 {
		acts = append(acts, r.action(i, func() error { return fmt.Errorf("failed %d", i) }))
	}
	b := &Builder{Jobs: 1}
	errs, notStarted := b.runActions(context.Background(), acts)
	err := joinErrors(notStarted, errs...)
	var multi *Errors
	if !errors.As(err, &multi) || len(multi.Errs) != 3 || multi.NotStarted != 7 {
		t.Fatalf("error %v, want 3 errors and 7 not started", err)
	}
	if !strings.Contains(err.Error(), "7 more actions were not started") {
		t.Errorf("the message does not say how many were left out: %v", err)
	}
	if !slices.Equal(r.started, []int{0, 1, 2}) {
		t.Errorf("started %v, want 0 1 2", r.started)
	}
}

// A canceled context stops the starting of actions; the ones that run finish.
func TestRunActionsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var r recorder
	var acts []action
	for i := range 10 {
		acts = append(acts, r.action(i, func() error {
			if i == 2 {
				cancel()
			}
			return nil
		}))
	}
	b := &Builder{Jobs: 1}
	_, notStarted := b.runActions(ctx, acts)
	if !slices.Equal(r.started, []int{0, 1, 2}) || notStarted != 7 {
		t.Errorf("started %v, not started %d, want 0 1 2 and 7", r.started, notStarted)
	}
	if !slices.Equal(r.flushed, []int{0, 1, 2}) {
		t.Errorf("flushed %v: the text of the actions that ran comes out", r.flushed)
	}
}

func TestJobs(t *testing.T) {
	if got := (&Builder{}).jobs(); got != DefaultJobs() || got < 1 {
		t.Errorf("default jobs %d", got)
	}
	if got := (&Builder{Jobs: 7}).jobs(); got != 7 {
		t.Errorf("jobs %d", got)
	}
}
