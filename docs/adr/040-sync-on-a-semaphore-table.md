# ADR-040: `sync` is Go's algorithms on words, with the waiters in a table outside the objects

Status: accepted (2026-10-18)

## Context

Phase 08 step 8.7 builds `sync` and `sync/atomic`. A goroutine is an OS thread (ADR-007) and every
blocking operation parks on a `gp::waiter` (ADR-026). Three constraints shaped the package:

- Go lets one goroutine lock a `Mutex` and another unlock it, which is undefined behavior for
  `std::mutex`, and a `Mutex` is copied freely before first use (a copy of a locked mutex is locked).
  So a `sync` object can have no pointers, no queues and no destructor.
- Every pointer method call in generated code passes the receiver `gp::ptr<T>` by value: two atomic
  operations on the object's reference count (ADR-004), before the method does anything.
- Natives (ADR-027) are plain functions, and the C++ type of a Go struct is not known to the runtime.

## Decision

- **The types are Go structs in `stdlib/sync/*.gp`** (`Mutex{state int32; sema uint32}`,
  `RWMutex`, `WaitGroup{state uint64; sema uint32}`, `Cond`, `Once`), with Go's field layout and
  Go's doc comments. Their methods are one-line Go bodies that call natives.
- **The natives are C++ templates over the struct** (`gp::sync::mutex_lock(const gp::ptr<M>&)`,
  `gocpp/sync.hpp`): they read the fields by name, with a `static_assert` of the type in each, so a
  change in the `.gp` file that breaks the contract is a compile error. The fast paths are inline (one
  `std::atomic_ref` operation); the slow paths are in `src/sync.cpp`. The same for the typed atomics
  (`gp::sync_atomic::ty_load` and the others, over a struct with a field `v`). No pointer to a field is
  made, which would cost two more atomic operations.
- **The algorithms are Go's, line by line**: `internal/sync/mutex.go` (normal and starvation mode,
  `lockSlow`, `unlockSlow`), `sync/rwmutex.go` (writer preference), `sync/waitgroup.go`,
  `sync/cond.go` with `runtime/sema.go`'s notify list, and the semaphore (`semacquire`/`semrelease`,
  with the waiter count read without the lock, and the hand-off of the starving mutex). One difference: a
  hand-off release (`sema_release(addr, true)`) takes the first waiter off the queue and gives it the unit in
  one step under the bucket's lock, and the count of the word never shows the unit; Go adds the unit first and
  takes it for the waiter after the dequeue, a window of nanoseconds on a P, but microseconds with a thread for
  each goroutine, in which a newcomer took the unit (the easy path of `semacquire`) and got in ahead of the
  waiters of a starving mutex. With nobody queued the unit is left in the word, as a plain release does
  (`test_sema_handoff_is_one_step`).
- **The waiters live in a table keyed by the address of the semaphore word** (251 buckets of a
  `std::mutex` and a FIFO list of `gp::detail::sudog`, on the waiting goroutine's stack), as Go's
  `semaTable` is. A goroutine parks through `block_scope` (entered under the bucket's lock, before the
  entry is visible) with Go's wait reason (`sync.Mutex.Lock`, `sync.WaitGroup.Wait`,
  `sync.RWMutex.RLock`, `sync.RWMutex.Lock`, `sync.Cond.Wait`), and is woken through `detail::wake`
  (`unblock`), exactly as a channel does, so the blocked count for the deadlock detector (8.11) is
  right.
- **Unlock of an unlocked mutex is Go's fatal error** (`gp::detail::fatal`: `fatal error: sync: unlock of
  unlocked mutex`, exit code 2, no recover); the `WaitGroup` and `Cond` misuse messages are panics with a
  string value, as in Go.
- **Starvation threshold: Go's 1 ms plus 100 µs for each waiter.** In starvation mode every `Unlock` is a
  hand-off. Go's costs about 200 ns (a goroutine switch on the same P); here it is a futex wake-up and a
  context switch (20 to 50 µs idle, much more under load). With Go's constant, 100 threads on one mutex
  went into starvation mode at once and stayed in it: 13 s for 2M Lock/Unlock pairs against 0.3 s
  without it (`std::mutex`: 0.1 s), and 2000 goroutines 20 s. A wait that the length of the queue
  explains is not starvation; one that it does not explain still is. The values were measured with 100,
  2000 and 10000 goroutines.
- **`sync/atomic` on `std::atomic_ref`**, sequentially consistent (Go's model). Pointer operations
  (`LoadPointer`, `atomic.Pointer[T]`) cannot be one instruction because a gocpp pointer owns its object:
  a table of 64 spin locks keyed by the address of the pointer, and `gp::atomic_ptr<T>` (a lock and a
  `gp::ptr<T>`) for the runtime type under `atomic.Pointer[T]`. `atomic.Value` is a spin lock around an
  interface (two words and a count cannot be one atomic word).
- **`sync.Map` is a `map[any]any` under an `RWMutex`**, not Go's `HashTrieMap`; the key is hashed first
  (`hash_check`) so an unhashable key panics with Go's text before any lock is taken. `Range` walks a
  cached list of the entries (made by the first `Range` after the set of keys changed, kept only for a map of at
  most 4096 entries, about 64 bytes an entry) and looks each key up again, so it no longer copies everything under
  the read lock on every call (job 08.synclows; a port of Go's
  read-only plus dirty map was measured and dropped: its `atomic.Pointer` loads cost a spin lock each and made
  `Load` 2 to 10 times slower than the `RWMutex`). `Pool` is a
  mutex-protected free list (at most 65536 items: there is no collector to drop them).
- **`OnceFunc`, `OnceValue` and `OnceValues` are Go's source**; the two generic ones are instantiated
  per call by step 7.1 (merged) and tested. `atomic.Pointer[T]` (step 7.3) is a generic type written in Go over the same spin lock as `Value`
  (`{_ [0]*T; lk uint32; v *T}`; a native cannot be generic); `gp::atomic_ptr<T>` is not used by it.

## Alternatives

- **`std::mutex` and `std::shared_mutex`**: undefined behavior for the cross-goroutine unlock, not copyable.
- **`std::atomic::wait`/`notify` on the state word**: no queue of our own (nothing to account for the deadlock
  detector), no FIFO order, and no timed or counted wake-up for the notify list.
- **Keeping Go's starvation threshold**: measured above, a convoy of futex wake-ups.
- **Natives that take `*int32` for each field** (Go's own approach, `atomic.AddInt32(&m.state, ...)`): two
  more atomic operations per call for the aliasing pointer, and the whole of Go's `sync` lowered to C++ would
  call through them.
- **Intrinsic types for `Mutex` and `WaitGroup`** (as `weak.Pointer`): needs the lowerer to know the types,
  and gives no gain over the templates.

## Consequences

Good: Go's fairness, writer preference, wake-up order and panics, with objects that are plain words
(`unsafe.Sizeof(sync.Mutex{}) == 8` as in Go), ThreadSanitizer sees every edge (atomics and the bucket's
mutex), and the algorithms can be compared with Go's source line by line.

Bad:
- Every method call costs the two atomic operations of the by-value receiver: an uncontended Lock and
  Unlock is about 26 ns at -O2 against Go's 10 ns, of which about 20 ns is the reference count. A
  const-reference receiver in the lowering of pointer methods would help every method of the language, not
  only `sync` (Phase 15).
- A contended mutex wakes OS threads: hand-offs are microseconds, not nanoseconds.
- The templates read fields by name: `stdlib/sync` and `gocpp/sync.hpp` must change together.
- `sizeof(sync.Map)` grew from 40 to 144 bytes: the list (a slice) and 64 bytes of padding between the lock and the
  map (15 to 20 percent on `Load` at 8 goroutines; 104 MB more for a million Maps). A `Range` of a map above 4096
  entries makes its list for the one call and keeps nothing, and the first `Range` after a change of the keys pays
  O(n) for it (docs/benchmarks/runtime.md).
- `sync.Map` readers take the read lock (the same cost as before the cached list of `Range`) and
  `atomic.Value.Load` and `atomic.Pointer.Load` take a spin lock (a counted pointer is not one atomic word): about
  50 ns on one thread and microseconds under 64 threads, which is why `sync.Map` does not use `atomic.Pointer`.
- `NewCond(&q.mu)` with `mu` in the same struct as the Cond is a reference cycle; see the sync PORT.md.
