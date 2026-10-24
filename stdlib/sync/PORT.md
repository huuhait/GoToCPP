# sync and sync/atomic

Ported by hand from go1.25.14's `src/sync`, `src/internal/sync` and `src/sync/atomic` (BSD-3-Clause, see
`../LICENSE`) in Phase 08 step 8.7. Phase 09's `tools/portstd` will take over; these are the changes it must keep.
The design is ADR-040; the natives are `runtime/include/gocpp/sync.hpp` and `atomic.hpp`.

- `Mutex`, `RWMutex`, `WaitGroup`, `Cond`: the same fields as Go's (`state`/`sema`, `w`/`writerSem`/`readerSem`/
  `readerCount`/`readerWait`, `state uint64`/`sema`, `L` and the notify list as two words and the copy checker as
  a `uintptr`), with `atomic.Int32` and friends written as plain integers. The methods are one-line calls of
  natives that hold Go's algorithms (`internal/sync/mutex.go`, `sync/rwmutex.go`, `sync/waitgroup.go`,
  `sync/cond.go`, `runtime/sema.go`) over `std::atomic_ref`; the C++ templates read the fields by name. Not
  ported: the race-detector calls, the `synctest` bubbles of `WaitGroup`, `runtime_canSpin`'s test of the other
  Ps (spinning is 4 rounds on a machine with more than one CPU), the starvation threshold (1 ms plus 100 µs
  for each waiter, ADR-040). The starvation hand-off is tested with a clock of the test (`gp::sync::detail::set_clock_for_test`,
  runtime/test/sync_test.cpp): a waiter "that waited an hour" is held after its wake-up while the test takes the mutex.
- `Once`: `done` is a `uint32` read and written by natives; `doSlow` defers a closure, because a native is a
  C++ template and has no address to defer.
- `OnceFunc`: unchanged. `OnceValue`, `OnceValues` (instantiated by the caller, step 7.1): their state is local
  variables that the closures capture, and the function that `once.Do` runs (`g`) is made once, outside the returned
  closure (Go 1.22's form, with Go 1.25's `f = nil` in the deferred function), not Go 1.25's one unnamed struct, so that
  a type argument may be a C/C++ type (`sync.OnceValue(func() std.string {...})`): gocpp has no unnamed struct type
  with a C/C++ type argument of a generic function yet (dev.genericstd). The same behavior. Measured with `-release`
  (`OnceValue` of a `func() int`, best of 5 runs of 2,000,000): a call takes 2.0 ns on g++ and clang++ (11.5 and 10.0 ns
  with the struct, whose call made the closure for `Do` each time); making the function takes 106 ns on g++ and 88 ns
  on clang++ (36 and 35 ns with the struct: five boxes instead of one, and `g`).
- `Map`: a `map[any]any` under an `RWMutex` instead of `internal/sync.HashTrieMap`; `hashCheck` hashes the key
  first, so an unhashable key panics with Go's text before the lock. `Range` walks a list of the entries that the
  first `Range` after a change of the set of keys makes (under the write lock; a `Store` of a new key, a delete,
  `LoadOrStore` of a new key and `Clear` drop it, `Store` of an existing key, `Swap` and `CompareAndSwap` keep it),
  looks each visited key up again for its current value and skips one that was deleted since (a key that is not equal
  to itself, a NaN, is shown with the value of the list: nothing can find or change it). `f` runs with no lock, so it
  may call any method. A `Range` that stops after 3 entries costs 3 lookups and no copy while the keys do not change
  and the map has at most 4096 entries (`rangeListMax`); the first `Range` after a change pays the list, O(n), and
  holds it (about 64 bytes an entry, 256 KiB at most) until the next change; a bigger map makes the list for the one
  `Range` and keeps nothing (a million entries: RSS 196 MB before and after a `Range`), as the old Map copied for each
  `Range`. A port of Go's read-only map plus dirty
  map was tried first and dropped: every `atomic.Pointer` operation takes a spin lock here (a counted pointer is not
  one atomic word) and `Load`, `Store` and `LoadOrStore` came out 2 to 10 times slower than the lock
  (docs/benchmarks/runtime.md). A padding field of 64 bytes between the lock and `m` gains 15 to 20 percent on `Load` with 8
  goroutines but does not remove the dependence on the address of the Map (measured: the same two modes, 1.8 times
  apart, with and without it; the cause is the counted pointer into the Map that the lowering of `m.mu.RLock()`
  makes, an atomic reference count increment and decrement on every call, about 2.5 times on `Load` for any struct
  with an embedded mutex). `sizeof(sync.Map)` is 144 bytes (80 without the padding; 40 before the cached list).
- `Cond`: `NewCond(&q.mu)` with `mu` a field of the struct `q` that also holds the `Cond` is a reference cycle
  (`q` holds the Cond, the Cond's `L` is a counted pointer into `q`), so `q` is never freed (LeakSanitizer
  reports it, docs/guides/sanitizers.md). A weak pointer for `L` is not sound: `L` is any `Locker`, and
  `sync.NewCond(&sync.Mutex{})` or a locker made on the spot is referenced by the Cond alone, so a weak
  reference would dangle. Keep the mutex in an allocation of its own (`mu *sync.Mutex` created next to the
  struct and passed to `NewCond(mu)`): then nothing points back and `q` is freed.
- `Pool`: a mutex and a slice (at most 65536 items) instead of the per-P queues and the victim cache: gocpp has
  no garbage collector to clear them.
- `sync/atomic`: the function-style API is natives (`gp::sync_atomic::fn_*`, `std::atomic_ref`); the typed
  structs have one field `v` and call `ty_*` natives that read it; `Bool` is a `uint32`; `Value` is
  `{lk uint32; v any}` with a spin lock instead of the two-word protocol; `atomic.Pointer[T]` is
  `{_ [0]*T; lk uint32; v *T}` with the same spin lock (`spinLock`, `spinUnlock`), written in Go over it (a native
  cannot be generic, step 7.3).
