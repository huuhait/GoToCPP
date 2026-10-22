package b

// Pair has the same unnamed interface type as a.Holder.Mu: one type in the whole program.
type Pair struct {
	First  interface{ Lock() }
	Second interface{ Lock() }
}

type Mutex struct{ locked bool }

func (m *Mutex) Lock() { m.locked = true }

func Lock(p *Pair) {
	m := &Mutex{}
	p.First = m
	p.First.Lock()
	p.Second = p.First
}

func Locked(p Pair) bool { return p.Second.(*Mutex).locked }
