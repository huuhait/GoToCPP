// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

// Map is like a Go map[any]any but is safe for concurrent use
// by multiple goroutines without additional locking or coordination.
// Loads, stores, and deletes run in amortized constant time.
//
// The Map type is specialized. Most code should use a plain Go map instead,
// with separate locking or coordination, for better type safety and to make it
// easier to maintain other invariants along with the map content.
//
// The Map type is optimized for two common use cases: (1) when the entry for a given
// key is only ever written once but read many times, as in caches that only grow,
// or (2) when multiple goroutines read, write, and overwrite entries for disjoint
// sets of keys. In these two cases, use of a Map may significantly reduce lock
// contention compared to a Go map paired with a separate [Mutex] or [RWMutex].
//
// The zero Map is empty and ready for use. A Map must not be copied after first use.
//
// In the terminology of [the Go memory model], Map arranges that a write operation
// “synchronizes before” any read operation that observes the effect of the write, where
// read and write operations are defined as follows.
// [Map.Load], [Map.LoadAndDelete], [Map.LoadOrStore], [Map.Swap], [Map.CompareAndSwap],
// and [Map.CompareAndDelete] are read operations;
// [Map.Delete], [Map.LoadAndDelete], [Map.Store], and [Map.Swap] are write operations;
// [Map.LoadOrStore] is a write operation when it returns loaded set to false;
// [Map.CompareAndSwap] is a write operation when it returns swapped set to true;
// and [Map.CompareAndDelete] is a write operation when it returns deleted set to true.
//
// [the Go memory model]: https://go.dev/ref/mem
type Map struct {
	mu RWMutex

	// (gocpp: 64 bytes of padding between the lock and the words that every Load reads, m and snap.
	// Measured with 8 pinned goroutines at every address of the Map modulo 64: Load is 15 to 20
	// percent faster with it, and still depends on the address (about 90 ns at 16 and 32, 160 at 0
	// and 48). It costs 64 bytes a Map: sizeof(sync.Map) is 144 bytes, 80 without it, 40 before the list.)
	_ [64]byte

	m map[any]any

	// snap is the list of the entries for Range of a map of at most rangeListMax entries: built by
	// the first Range after a key was added or removed (under the write lock), dropped (nil) by the
	// next change of the set of keys, never written after it is built, so a Range reads it with no
	// lock. A Store of a key that is there and a Swap or CompareAndSwap do not change the set of
	// keys and keep it.
	snap []snapEntry
}

// A snapEntry is a key of the map and, for a key that is not equal to itself (a NaN), its value.
// Range looks any other key up again for the current value; nothing can find, change or delete the
// entry of a NaN, so its value is the list's.
type snapEntry struct {
	key, value any
}

// Load returns the value stored in the map for a key, or nil if no
// value is present.
// The ok result indicates whether value was found in the map.
func (m *Map) Load(key any) (value any, ok bool) {
	hashCheck(key)
	m.mu.RLock()
	value, ok = m.m[key]
	m.mu.RUnlock()
	return value, ok
}

// Store sets the value for a key.
func (m *Map) Store(key, value any) {
	hashCheck(key)
	m.mu.Lock()
	if m.m == nil {
		m.m = make(map[any]any)
	}
	n := len(m.m)
	m.m[key] = value
	if len(m.m) != n {
		m.snap = nil
	}
	m.mu.Unlock()
}

// Clear deletes all the entries, resulting in an empty Map.
func (m *Map) Clear() {
	m.mu.Lock()
	m.m = nil
	m.snap = nil
	m.mu.Unlock()
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (m *Map) LoadOrStore(key, value any) (actual any, loaded bool) {
	hashCheck(key)
	m.mu.Lock()
	defer m.mu.Unlock()
	if v, ok := m.m[key]; ok {
		return v, true
	}
	if m.m == nil {
		m.m = make(map[any]any)
	}
	m.m[key] = value
	m.snap = nil
	return value, false
}

// LoadAndDelete deletes the value for a key, returning the previous value if any.
// The loaded result reports whether the key was present.
func (m *Map) LoadAndDelete(key any) (value any, loaded bool) {
	hashCheck(key)
	m.mu.Lock()
	defer m.mu.Unlock()
	value, loaded = m.m[key]
	if loaded {
		delete(m.m, key)
		m.snap = nil
	}
	return value, loaded
}

// Delete deletes the value for a key.
// If the key is not in the map, Delete does nothing.
func (m *Map) Delete(key any) {
	m.LoadAndDelete(key)
}

// Swap swaps the value for a key and returns the previous value if any.
// The loaded result reports whether the key was present.
func (m *Map) Swap(key, value any) (previous any, loaded bool) {
	hashCheck(key)
	m.mu.Lock()
	defer m.mu.Unlock()
	previous, loaded = m.m[key]
	if m.m == nil {
		m.m = make(map[any]any)
	}
	m.m[key] = value
	if !loaded {
		m.snap = nil
	}
	return previous, loaded
}

// CompareAndSwap swaps the old and new values for key
// if the value stored in the map is equal to old.
// The old value must be of a comparable type.
func (m *Map) CompareAndSwap(key, old, new any) (swapped bool) {
	hashCheck(key)
	m.mu.Lock()
	defer m.mu.Unlock()
	if cur, ok := m.m[key]; ok && cur == old {
		m.m[key] = new
		return true
	}
	return false
}

// CompareAndDelete deletes the entry for key if its value is equal to old.
// The old value must be of a comparable type.
//
// If there is no current value for key in the map, CompareAndDelete
// returns false (even if the old value is the nil interface value).
func (m *Map) CompareAndDelete(key, old any) (deleted bool) {
	hashCheck(key)
	m.mu.Lock()
	defer m.mu.Unlock()
	if cur, ok := m.m[key]; ok && cur == old {
		delete(m.m, key)
		m.snap = nil
		return true
	}
	return false
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
//
// Range does not necessarily correspond to any consistent snapshot of the Map's
// contents: no key will be visited more than once, but if the value for any key
// is stored or deleted concurrently (including by f), Range may reflect any
// mapping for that key from any point during the Range call. Range does not
// block other methods on the receiver; even f itself may call any method on m.
//
// Range may be O(N) with the number of elements in the map even if f returns
// false after a constant number of calls.
func (m *Map) Range(f func(key, value any) bool) {
	// f may call any method of m, so it runs with no lock: it walks the list of the entries (made
	// once for a set of keys, and dropped when a key is added or removed), looks each key up for its
	// current value, and skips one that was deleted since (by f too), as in Go. A key that is not
	// equal to itself (a NaN) cannot be looked up: it is shown with the value of the list.
	for _, e := range m.entries() {
		v := e.value
		if e.key == e.key {
			var ok bool
			v, ok = m.Load(e.key)
			if !ok {
				continue
			}
		}
		if !f(e.key, v) {
			break
		}
	}
}

// rangeListMax is the most entries for which the list of Range is kept between calls. The list costs
// about 64 bytes an entry (measured: 62 MB for a million), so 4096 entries are 256 KiB at most; a Range
// of a bigger map makes its list for the one call and keeps nothing, so a map of a million entries does
// not hold 62 MB more after a Range.
const rangeListMax = 4096

// entries returns the list of the entries for Range, which is not changed after it is returned.
func (m *Map) entries() []snapEntry {
	m.mu.RLock()
	snap := m.snap
	if snap == nil && len(m.m) > rangeListMax {
		snap = m.listLocked()
	}
	m.mu.RUnlock()
	if snap != nil {
		return snap
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.snap == nil {
		m.snap = m.listLocked()
	}
	return m.snap
}

// listLocked makes the list of the entries (with mu held, for reading at least).
func (m *Map) listLocked() []snapEntry {
	snap := make([]snapEntry, 0, len(m.m))
	for k, v := range m.m {
		if k == k {
			v = nil // looked up again by Range; not kept alive here after a Swap
		}
		snap = append(snap, snapEntry{k, v})
	}
	return snap
}

// hashCheck panics for a key that can't be hashed ("runtime error: hash of unhashable type
// []int"), as the hash table of Go's sync.Map does for every operation; a lookup in an empty Go map
// would panic with another text (mapKeyError), and a panic with a lock held would leave it held.
//
//gocpp:native gp::sync::hash_check
func hashCheck(key any)
