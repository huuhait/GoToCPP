package memoize

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/safe"
)

// TestPromiseOnce checks that many goroutines that ask at the same time get
// one computation and the same value.
func TestPromiseOnce(t *testing.T) {
	var runs atomic.Int32
	release := make(chan struct{})
	p := NewPromise(func(context.Context) (*int, error) {
		runs.Add(1)
		<-release
		v := 42
		return &v, nil
	})
	const n = 50
	got := make([]*int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := p.Get(context.Background())
			if err != nil {
				t.Error(err)
			}
			got[i] = v
		}()
	}
	time.Sleep(10 * time.Millisecond) // let them all wait
	close(release)
	wg.Wait()
	if runs.Load() != 1 {
		t.Errorf("%d computations, want 1", runs.Load())
	}
	for i, v := range got {
		if v != got[0] || *v != 42 {
			t.Fatalf("Get %d: %p (%d), want the shared %p", i, v, *v, got[0])
		}
	}
	if v, err, done := p.Done(); !done || err != nil || v != got[0] {
		t.Errorf("Done: %v, %v, %v", v, err, done)
	}
}

// TestPromiseError checks that an error is a result too: it is kept.
func TestPromiseError(t *testing.T) {
	runs := 0
	boom := errors.New("boom")
	p := NewPromise(func(context.Context) (int, error) { runs++; return 0, boom })
	for range 3 {
		if _, err := p.Get(context.Background()); err != boom {
			t.Fatalf("Get: %v", err)
		}
	}
	if runs != 1 {
		t.Errorf("%d computations, want 1", runs)
	}
}

// TestPromiseCancel checks the contexts: a Get that gives up returns at once
// while the computation goes on for the others; when every Get has given
// up, the computation's context is done, its result is dropped, and the
// next Get computes again.
func TestPromiseCancel(t *testing.T) {
	var runs atomic.Int32
	started := make(chan context.Context, 10)
	finish := make(chan struct{})
	p := NewPromise(func(ctx context.Context) (int, error) {
		n := runs.Add(1)
		started <- ctx
		select {
		case <-finish:
			return int(n), nil
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	})

	// Two Gets; the first gives up, the second gets the value.
	ctx1, cancel1 := context.WithCancel(context.Background())
	errc := make(chan error)
	go func() { _, err := p.Get(ctx1); errc <- err }()
	cctx := <-started
	valc := make(chan int)
	go func() {
		v, err := p.Get(context.Background())
		if err != nil {
			t.Error(err)
		}
		valc <- v
	}()
	waitFor(t, func() bool { return waiters(p) == 2 })
	cancel1()
	if err := <-errc; err != context.Canceled {
		t.Errorf("the Get that gave up: %v", err)
	}
	if cctx.Err() != nil {
		t.Fatal("the computation was cancelled while a Get still waits")
	}
	finish <- struct{}{}
	if v := <-valc; v != 1 {
		t.Errorf("value %d, want 1", v)
	}

	// Every Get gives up: the computation stops, and the next Get starts a
	// new one.
	q := NewPromise(func(ctx context.Context) (int, error) {
		n := runs.Add(1)
		started <- ctx
		select {
		case <-finish:
			return int(n), nil
		case <-ctx.Done():
			return -1, ctx.Err()
		}
	})
	ctx2, cancel2 := context.WithCancel(context.Background())
	go func() { _, err := q.Get(ctx2); errc <- err }()
	cctx = <-started
	cancel2()
	<-errc
	select {
	case <-cctx.Done():
	case <-time.After(time.Minute):
		t.Fatal("the computation was not cancelled after every Get gave up")
	}
	waitFor(t, q.idle)
	if _, _, done := q.Done(); done {
		t.Fatal("the result of a cancelled computation was kept")
	}
	go func() {
		v, _ := q.Get(context.Background())
		valc <- v
	}()
	<-started
	finish <- struct{}{}
	if v := <-valc; v != 3 {
		t.Errorf("the computation after the cancelled one gave %d, want 3 (the third run)", v)
	}

	// A Get with a done context doesn't start anything.
	done, cancel := context.WithCancel(context.Background())
	cancel()
	r := NewPromise(func(context.Context) (int, error) { t.Error("computed"); return 0, nil })
	if _, err := r.Get(done); err != context.Canceled {
		t.Errorf("Get with a done context: %v", err)
	}
}

// waiters returns the number of Gets that wait for the run in flight.
func waiters[V any](p *Promise[V]) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.run == nil {
		return 0
	}
	return p.run.waiters
}

// waitFor waits until cond holds; the minute only ends a hang.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(time.Minute); !cond(); {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(time.Millisecond)
	}
}

// TestStoreHits checks that a store computes each key once, and that the
// value depends on the key alone.
func TestStoreHits(t *testing.T) {
	s := NewStore[string, string](0, nil)
	var runs atomic.Int32
	get := func(key string) string {
		v, err := s.Get(context.Background(), key, func(context.Context) (string, error) {
			runs.Add(1)
			return "value of " + key, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := fmt.Sprint(i % 4)
			if v := get(key); v != "value of "+key {
				t.Errorf("Get(%s) = %q", key, v)
			}
		}()
	}
	wg.Wait()
	if runs.Load() != 4 {
		t.Errorf("%d computations for 4 keys", runs.Load())
	}
	if n, cost := s.Stats(); n != 4 || cost != 4 {
		t.Errorf("Stats: %d values, cost %d", n, cost)
	}
	if v, _, ok := s.Peek("2"); !ok || v != "value of 2" {
		t.Errorf("Peek(2) = %q, %v", v, ok)
	}
	if _, _, ok := s.Peek("9"); ok {
		t.Error("Peek(9) found a value")
	}
}

// TestStoreLRU checks the cost cap: the values used least recently go
// first, a dropped value is computed again when it is asked for, and the
// newest value stays even when it alone is over the cap.
func TestStoreLRU(t *testing.T) {
	s := NewStore[int, int](10, func(v int, _ error) int64 { return int64(v) })
	runs := make(map[int]int)
	get := func(v int) {
		t.Helper()
		got, err := s.Get(context.Background(), v, func(context.Context) (int, error) { runs[v]++; return v, nil })
		if err != nil || got != v {
			t.Fatalf("Get(%d) = %d, %v", v, got, err)
		}
	}
	kept := func() []int {
		var keys []int
		for e := s.lru.Front(); e != nil; e = e.Next() {
			keys = append(keys, e.Value.(*entry[int, int]).key)
		}
		return keys
	}
	get(3)
	get(4)
	get(2) // 9 in all
	get(3) // 3 is the most recent now
	get(1) // 10
	if got := fmt.Sprint(kept()); got != "[1 3 2 4]" {
		t.Errorf("kept %s, want [1 3 2 4]", got)
	}
	get(5) // 15: 4 and then 2 go (least recently used)
	if got := fmt.Sprint(kept()); got != "[5 1 3]" {
		t.Errorf("kept %s, want [5 1 3]", got)
	}
	if _, cost := s.Stats(); cost != 9 {
		t.Errorf("cost %d, want 9", cost)
	}
	get(4) // computed again
	if runs[4] != 2 || runs[3] != 1 {
		t.Errorf("computations: %v", runs)
	}
	get(50) // over the cap alone: only it stays
	if got := fmt.Sprint(kept()); got != "[50]" {
		t.Errorf("kept %s, want [50]", got)
	}
	if len(s.entries) != 1 {
		t.Errorf("%d entries, want 1", len(s.entries))
	}
}

// TestStoreCancel checks that a key whose every Get gave up leaves no entry,
// so the store holds nothing for it.
func TestStoreCancel(t *testing.T) {
	s := NewStore[string, int](0, nil)
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	errc := make(chan error)
	go func() {
		_, err := s.Get(ctx, "k", func(ctx context.Context) (int, error) {
			close(started)
			<-ctx.Done()
			return 0, ctx.Err()
		})
		errc <- err
	}()
	<-started
	cancel()
	if err := <-errc; err != context.Canceled {
		t.Fatalf("Get: %v", err)
	}
	waitFor(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.entries) == 0
	})
	v, err := s.Get(context.Background(), "k", func(context.Context) (int, error) { return 7, nil })
	if v != 7 || err != nil {
		t.Errorf("Get after the cancel: %d, %v", v, err)
	}
}

// TestStoreCancelRace cancels many Gets right when their values are done:
// every value that is done is counted, so the store keeps no more than its
// cap (a Get that gave up used to leave its entry uncounted).
func TestStoreCancelRace(t *testing.T) {
	const max = 10
	s := NewStore[int, int](max, nil)
	var wg sync.WaitGroup
	for i := range 3000 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			s.Get(ctx, i, func(context.Context) (int, error) {
				cancel() // the Get's context ends as the value is done
				return i, nil
			})
		}()
	}
	wg.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, e := range s.entries {
		if _, _, done := e.promise.Done(); done && e.elem == nil {
			t.Errorf("the value of %d is done but not counted", key)
		}
	}
	if len(s.entries) > max {
		t.Errorf("%d entries, over the cap %d", len(s.entries), max)
	}
}

func TestCounters(t *testing.T) {
	s := NewStore[string, int](0, nil)
	get := func(key string) {
		if _, err := s.Get(context.Background(), key, func(context.Context) (int, error) { return len(key), nil }); err != nil {
			t.Fatal(err)
		}
	}
	get("a")
	get("a")
	get("bb")
	get("a")
	if hits, misses := s.Counters(); hits != 2 || misses != 2 {
		t.Errorf("hits %d, misses %d, want 2 and 2", hits, misses)
	}
	if s.MaxCost() != 0 {
		t.Errorf("max cost %d", s.MaxCost())
	}
}

// TestPromisePanic: a computation runs on a goroutine that no caller owns, so
// a panic in it would end the process; it is the error of the promise, for
// every Get, and the stack of the panic is in it.
func TestPromisePanic(t *testing.T) {
	runs := 0
	p := NewPromise(func(context.Context) (int, error) {
		runs++
		var m map[string]int
		m["x"] = 1
		return 1, nil
	})
	for range 3 {
		_, err := p.Get(context.Background())
		var pn *safe.Panic
		if !errors.As(err, &pn) || !strings.Contains(err.Error(), "nil map") || !strings.Contains(string(pn.Stack), "TestPromisePanic") {
			t.Fatalf("Get: %v", err)
		}
	}
	if runs != 1 {
		t.Errorf("%d computations, want 1", runs)
	}
}

// TestStoreSetMaxCost: lowering the cap drops the least recently used values at
// once (the newest stays), raising it keeps all.
func TestStoreSetMaxCost(t *testing.T) {
	s := NewStore[int](0, func(v int, _ error) int64 { return 10 })
	for i := range 6 {
		if _, err := s.Get(context.Background(), i, func(context.Context) (int, error) { return i, nil }); err != nil {
			t.Fatal(err)
		}
	}
	if n, cost := s.Stats(); n != 6 || cost != 60 {
		t.Fatalf("%d values, cost %d", n, cost)
	}
	s.SetMaxCost(25)
	if n, cost := s.Stats(); n != 2 || cost != 20 || s.MaxCost() != 25 {
		t.Errorf("after the cap 25: %d values, cost %d", n, cost)
	}
	if _, _, ok := s.Peek(5); !ok {
		t.Error("the newest value was dropped")
	}
	if _, _, ok := s.Peek(0); ok {
		t.Error("the oldest value stayed")
	}
	s.SetMaxCost(0)
	if s.MaxCost() != 0 {
		t.Error("no cap")
	}
}
