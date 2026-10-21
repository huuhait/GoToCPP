package cache

// A layered is a map that a state shares with the states cloned from it. A snapshot is cloned
// for every edit and keeps nearly everything it has read (1,500 files and 250 package handles
// on a big project), so copying each table for each clone made 100 KiB of garbage per edit,
// which the collector cycles are paid for. The table is a big map that the states cloned from
// one another share and never change (base), the keys of base that a state has dropped
// (hidden), and the entries that the state itself added or kept (own). A clone copies own and
// hidden, which stay small; when they have grown to a part of base, the clone makes a new base
// of what is visible instead, which is the cost of the plain copy, paid once for many clones.
//
// A key that is in both own and base is in hidden too (put adds an entry only where get found
// none). The table has no lock of its own: the state's mutex guards get and put, as it guarded
// the map. base is read by many states at the same time and is never written after the clone
// that made it.
type layered[K comparable, V any] struct {
	base   map[K]V
	hidden map[K]struct{}
	own    map[K]V
}

// layeredSlack is how many entries own and hidden may hold beside any base, and a quarter of the
// base besides: past it the clone makes a new base.
const layeredSlack = 32

// get returns the value of key, and whether there is one. A nil table has none (a released state's).
func (t *layered[K, V]) get(key K) (V, bool) {
	if t == nil {
		var none V
		return none, false
	}
	if v, ok := t.own[key]; ok {
		return v, true
	}
	if _, dropped := t.hidden[key]; dropped {
		var none V
		return none, false
	}
	v, ok := t.base[key]
	return v, ok
}

// put sets the value of key. The caller has seen that get found none.
func (t *layered[K, V]) put(key K, v V) {
	if t.own == nil {
		t.own = make(map[K]V)
	}
	t.own[key] = v
}

// size returns the number of entries in the table.
func (t *layered[K, V]) size() int {
	if t == nil {
		return 0
	}
	n := len(t.own) + len(t.base)
	for key := range t.hidden {
		if _, inBase := t.base[key]; inBase {
			n--
		}
	}
	return n
}

// each calls visit with every entry of the table, in no order.
func (t *layered[K, V]) each(visit func(key K, v V)) {
	if t == nil {
		return
	}
	for key, v := range t.base {
		if _, dropped := t.hidden[key]; !dropped {
			visit(key, v)
		}
	}
	for key, v := range t.own {
		visit(key, v)
	}
}

// clone returns the table without the entries that drop says so for. The table it is made from
// stays as it was and goes on being used (the requests of the old snapshot read and add entries).
func (t *layered[K, V]) clone(drop func(key K, v V) bool) *layered[K, V] {
	n := &layered[K, V]{base: t.base}
	if len(t.own) > 0 {
		n.own = make(map[K]V, len(t.own))
		for key, v := range t.own {
			if !drop(key, v) {
				n.own[key] = v
			}
		}
	}
	if len(t.hidden) > 0 {
		n.hidden = make(map[K]struct{}, len(t.hidden))
		for key := range t.hidden {
			n.hidden[key] = struct{}{}
		}
	}
	for key, v := range t.base {
		if _, dropped := t.hidden[key]; dropped || !drop(key, v) {
			continue
		}
		if n.hidden == nil {
			n.hidden = make(map[K]struct{})
		}
		n.hidden[key] = struct{}{}
	}
	if len(n.own)+len(n.hidden) > layeredSlack+len(n.base)/4 {
		return n.flatten()
	}
	return n
}

// flatten returns a table with the entries of t in base alone.
func (t *layered[K, V]) flatten() *layered[K, V] {
	all := make(map[K]V, t.size())
	t.each(func(key K, v V) { all[key] = v })
	return &layered[K, V]{base: all}
}
