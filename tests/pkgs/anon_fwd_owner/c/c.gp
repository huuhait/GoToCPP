package c

import (
	"example.com/anonfwd/a"
	"example.com/anonfwd/b"
)

type Getter interface{ Get() int }
type Incer interface{ Inc() }

// Only here is the struct converted to interfaces: the method tables, and so the forwarders, are
// asked for by a package that does not own the struct.
func Conv(n int) (Getter, Incer, any) {
	v := a.Make(n)
	p := &struct{ b.T }{b.T{N: n * 2}}
	_ = v
	return v, p, struct{ b.T }{b.T{N: 7}}
}
