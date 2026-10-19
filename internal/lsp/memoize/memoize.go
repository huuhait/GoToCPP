// Package memoize computes values once and shares them, for the language
// server's caches (Phase 11, step 11.3), like gopls's internal/memoize
// (written anew, smaller): a Promise computes its value once for every
// caller that asks at the same time, and a Store keeps a Promise per key,
// such as the hash of the inputs, with a memory cap (least recently used
// values go first).
//
// The computed values are shared by every goroutine that gets them, so
// they must never be changed. A computation runs on its own goroutine with
// its own context: a caller that gives up (its context is done) stops
// waiting at once, and when the last caller has given up, the computation's
// context is cancelled, its result is dropped, and the next Get starts
// again. No lock is held while a value is computed.
package memoize

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
)

// A Promise is a value that is computed once, on its first Get, by the
// function that NewPromise got. Its methods are safe for concurrent use.
type Promise[V any] struct {
	mu      sync.Mutex
	compute func(ctx context.Context) (V, error) // nil once done
	run     *run                                 // the computation in flight, or nil
	done    bool
	val     V
	err     error
}

// A run is one computation of a Promise.
type run struct {
	waiters int // the Gets that wait for it
	cancel  context.CancelFunc
	done    chan struct{}
}

// NewPromise returns a promise of the value that compute computes. compute
// runs on a goroutine of its own, with a context that keeps the values of
// the context of the Get that started it, and that is done when every Get
// that waits for it has given up. Its result is kept, also an error, unless
// that context was done by then.
func NewPromise[V any](compute func(ctx context.Context) (V, error)) *Promise[V] {
	return &Promise[V]{compute: compute}
}

// Get returns the value of the promise, after it computes it (or waits for
// the computation in flight). When ctx is done first, Get returns at once
// with ctx.Err(); the computation goes on as long as another Get waits for
// it.
func (p *Promise[V]) Get(ctx context.Context) (V, error) {
	for {
		p.mu.Lock()
		if p.done {
			v, err := p.val, p.err
			p.mu.Unlock()
			return v, err
		}
		if err := ctx.Err(); err != nil {
			p.mu.Unlock()
			var zero V
			return zero, err
		}
		r := p.run
		if r == nil {
			rctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
			r = &run{cancel: cancel, done: make(chan struct{})}
			p.run = r
			go p.do(rctx, r, p.compute)
		}
		r.waiters++
		p.mu.Unlock()

		select {
		case <-r.done:
			// The run kept its result, unless its context was done, which
			// needs every waiter to leave first: then try again.
		case <-ctx.Done():
			p.mu.Lock()
			r.waiters--
			if r.waiters == 0 && p.run == r {
				p.run = nil
				r.cancel()
			}
			p.mu.Unlock()
			var zero V
			return zero, ctx.Err()
		}
	}
}

// do runs compute for the run r and keeps its result.
func (p *Promise[V]) do(ctx context.Context, r *run, compute func(ctx context.Context) (V, error)) {
	v, err := compute(ctx)
	p.mu.Lock()
	if p.run == r {
		p.run = nil
	}
	if ctx.Err() == nil {
		p.done, p.val, p.err, p.compute = true, v, err, nil
	}
	close(r.done)
	p.mu.Unlock()
	r.cancel()
}

// Done reports whether the value is computed, and returns it then.
func (p *Promise[V]) Done() (v V, err error, done bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.val, p.err, p.done
}

// idle reports whether the value is not computed and no computation is in
// flight.
func (p *Promise[V]) idle() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.done && p.run == nil
}

// A Store is a set of promises, one per key, whose computed values are kept
// up to a total cost: when the values that are done cost more than the
// maximum, the ones used least recently are dropped (a holder of a dropped
// value keeps it; the store computes it again when it is asked for it).
// Its methods are safe for concurrent use.
type Store[K comparable, V any] struct {
	maxCost int64
	cost    func(V, error) int64

	mu      sync.Mutex
	entries map[K]*entry[K, V]
	lru     list.List // the entries that are done and counted, most recently used first
	total   int64     // the cost of the entries in lru

	hits, misses atomic.Int64 // Counters
}

type entry[K comparable, V any] struct {
	key     K
	promise *Promise[V]
	elem    *list.Element // in Store.lru once the value is done and counted
	cost    int64
}

// NewStore returns a store whose values cost cost(v, err) each (nil: 1),
// up to maxCost in all (0 or less: no maximum). The most recently used value
// is always kept, also when it alone costs more than maxCost.
func NewStore[K comparable, V any](maxCost int64, cost func(V, error) int64) *Store[K, V] {
	if cost == nil {
		cost = func(V, error) int64 { return 1 }
	}
	return &Store[K, V]{maxCost: maxCost, cost: cost, entries: make(map[K]*entry[K, V])}
}

// Get returns the value of key: the kept one, or else the one that compute
// computes (see Promise.Get). A later Get of the same key shares the
// computation in flight and its result, whatever compute it gives, so the
// value must depend on the key alone.
func (s *Store[K, V]) Get(ctx context.Context, key K, compute func(ctx context.Context) (V, error)) (V, error) {
	s.mu.Lock()
	e := s.entries[key]
	if e == nil {
		e = &entry[K, V]{key: key, promise: NewPromise(compute)}
		s.entries[key] = e
		s.misses.Add(1)
	} else {
		s.hits.Add(1)
		if e.elem != nil {
			s.lru.MoveToFront(e.elem)
		}
	}
	s.mu.Unlock()

	v, err := e.promise.Get(ctx)
	// The value may be done even when ctx ended at the same time: it is
	// counted then too, so that no value escapes the cap.
	if dv, derr, done := e.promise.Done(); done {
		s.count(e, dv, derr)
		return v, err
	}
	// Nothing is computed for the key when every Get gave up: the entry
	// goes, with the compute function and what it holds. (A Get that
	// started meanwhile keeps it.)
	s.mu.Lock()
	if s.entries[key] == e && e.promise.idle() {
		delete(s.entries, key)
	}
	s.mu.Unlock()
	return v, err
}

// count puts the entry e, whose value v (and err) is done, in the LRU list,
// if it is not there and still in the store, and drops the least recently
// used values over the maximum cost.
func (s *Store[K, V]) count(e *entry[K, V], v V, err error) {
	c := s.cost(v, err)
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.elem != nil || s.entries[e.key] != e {
		return
	}
	e.cost = c
	e.elem = s.lru.PushFront(e)
	s.total += c
	for s.maxCost > 0 && s.total > s.maxCost && s.lru.Len() > 1 {
		old := s.lru.Remove(s.lru.Back()).(*entry[K, V])
		old.elem = nil
		s.total -= old.cost
		delete(s.entries, old.key)
	}
}

// Peek returns the kept value of key, if it is done, without computing it
// or counting it as used.
func (s *Store[K, V]) Peek(key K) (v V, err error, ok bool) {
	s.mu.Lock()
	e := s.entries[key]
	s.mu.Unlock()
	if e == nil {
		return v, nil, false
	}
	return e.promise.Done()
}

// Counters returns how often Get found the key in the store (a hit: its
// value is kept, or being computed for another Get) and how often it did
// not (a miss: the value is computed for this Get).
func (s *Store[K, V]) Counters() (hits, misses int64) { return s.hits.Load(), s.misses.Load() }

// MaxCost returns the cap of the store (0: none).
func (s *Store[K, V]) MaxCost() int64 { return s.maxCost }

// Stats returns the number of kept values that are done and their cost.
func (s *Store[K, V]) Stats() (n int, cost int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lru.Len(), s.total
}

// ForgetIf drops the value of key when it is done and stale reports true
// for it: the next Get computes it again. A holder of the value keeps it.
// The language server forgets a checked package whose namespace probe
// answers have changed since it was checked (cache.Package.Stale).
func (s *Store[K, V]) ForgetIf(key K, stale func(V, error) bool) {
	s.mu.Lock()
	e := s.entries[key]
	s.mu.Unlock()
	if e == nil {
		return
	}
	v, err, done := e.promise.Done()
	if !done || !stale(v, err) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries[key] != e {
		return
	}
	delete(s.entries, key)
	if e.elem != nil {
		s.lru.Remove(e.elem)
		e.elem = nil
		s.total -= e.cost
	}
}
