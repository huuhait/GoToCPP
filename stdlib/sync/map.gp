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
	m  map[any]any
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
	m.m[key] = value
	m.mu.Unlock()
}

// Clear deletes all the entries, resulting in an empty Map.
func (m *Map) Clear() {
	m.mu.Lock()
	m.m = nil
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
	// The entries are copied under the read lock, and each key is looked up again before f sees
	// it, so f may call any method of m, and an entry that was deleted since (by f too) is not
	// shown, as in Go. A key that is not equal to itself (a NaN) cannot be looked up: it is shown
	// with the value that was copied.
	m.mu.RLock()
	keys := make([]any, 0, len(m.m))
	vals := make([]any, 0, len(m.m))
	for k, v := range m.m {
		keys = append(keys, k)
		vals = append(vals, v)
	}
	m.mu.RUnlock()
	for i, k := range keys {
		v := vals[i]
		if k == k {
			var ok bool
			v, ok = m.Load(k)
			if !ok {
				continue
			}
		}
		if !f(k, v) {
			break
		}
	}
}

// hashCheck panics for a key that can't be hashed ("runtime error: hash of unhashable type
// []int"), as the hash table of Go's sync.Map does for every operation; a lookup in an empty Go map
// would panic with another text (mapKeyError), and a panic with a lock held would leave it held.
//
//gocpp:native gp::sync::hash_check
func hashCheck(key any)
