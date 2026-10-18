// phase: 05

// A state machine of value types behind an interface (comparison, map keys, an optional Unwrap method).
package main

import "fmt"

type State interface {
	Next(ev Event) State
	fmt.Stringer
}
type Event int

const (
	Coin Event = iota
	Push
	Reset
)

func (e Event) String() string { return [...]string{"Coin", "Push", "Reset"}[e] }

type locked struct{ pushes int }
type unlocked struct{}
type broken struct{ why error }

func (l locked) Next(ev Event) State {
	switch ev {
	case Coin:
		return unlocked{}
	case Push:
		l.pushes++
		if l.pushes > 2 {
			return broken{fmt.Errorf("pushed %d times while locked", l.pushes)}
		}
		return l
	}
	return l
}
func (locked) String() string   { return "locked" }
func (unlocked) String() string { return "unlocked" }
func (u unlocked) Next(ev Event) State {
	if ev == Push {
		return locked{}
	}
	return u
}
func (b broken) Next(ev Event) State {
	if ev == Reset {
		return locked{}
	}
	return b
}
func (b broken) String() string { return "broken: " + b.why.Error() }
func (b broken) Unwrap() error  { return b.why }

func main() {
	var s State = locked{}
	for _, ev := range []Event{Push, Push, Coin, Push, Push, Push, Push, Coin, Reset, Coin} {
		n := s.Next(ev)
		fmt.Printf("%v --%v--> %v (same=%v)\n", s, ev, n, n == s)
		s = n
	}
	var st fmt.Stringer = s
	if u, ok := st.(interface{ Unwrap() error }); ok {
		fmt.Println("unwrap", u.Unwrap())
	} else {
		fmt.Printf("%T %v\n", st, st)
	}
	m := map[State]int{locked{}: 1, unlocked{}: 2, locked{pushes: 1}: 3}
	fmt.Println(m[locked{}], m[unlocked{}], m[locked{1}], m[broken{}] == 0, len(m))
}
