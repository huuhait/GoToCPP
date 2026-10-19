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
  for each waiter, ADR-040).
- `Once`: `done` is a `uint32` read and written by natives; `doSlow` defers a closure, because a native is a
  C++ template and has no address to defer.
- `OnceFunc`, `OnceValue`, `OnceValues`: unchanged (the generic two are instantiated by the caller, step 7.1).
- `Map`: a `map[any]any` under an `RWMutex` instead of `internal/sync.HashTrieMap`; `hashCheck` hashes the key
  first, so an unhashable key panics with Go's text before the lock. `Range` copies the entries under the read
  lock and looks each key up again (a key that is not equal to itself, a NaN, is shown with the copied value).
- `Pool`: a mutex and a slice (at most 65536 items) instead of the per-P queues and the victim cache: gocpp has
  no garbage collector to clear them.
- `sync/atomic`: the function-style API is natives (`gp::sync_atomic::fn_*`, `std::atomic_ref`); the typed
  structs have one field `v` and call `ty_*` natives that read it; `Bool` is a `uint32`; `Value` is
  `{lk uint32; v any}` with a spin lock instead of the two-word protocol; `atomic.Pointer[T]` is
  `{_ [0]*T; lk uint32; v *T}` with the same spin lock (`spinLock`, `spinUnlock`), written in Go over it (a native
  cannot be generic, step 7.3).
