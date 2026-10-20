package lib

type T struct {
	N    int
	Name string
}

var Held *T

// Use, Get and the field-less helper only read their pointer, so a program would borrow it
// (gp::bptr); in a library the exported ones keep gp::ptr (ADR-086): C++ callers hold the
// pointer however they like.
func Use(t *T) int {
	return t.N + len(t.Name)
}

func (t *T) Get() int { return t.N }

func (t *T) get2() int { return t.N }

func unexported(t *T) int { return t.N }

func Drop(t *T) int {
	Held = nil
	return t.N
}
