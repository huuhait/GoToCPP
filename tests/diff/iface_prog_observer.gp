// phase: 05

// An event bus: interface listeners, func types with methods, unsubscribing by interface identity, a recover around the listeners.
package main

import "fmt"

type Event interface{ Kind() string }
type Click struct{ X, Y int }
type Key struct{ Code rune }
type Quit struct{}

func (Click) Kind() string { return "click" }
func (Key) Kind() string   { return "key" }
func (*Quit) Kind() string { return "quit" }

type Listener interface{ On(Event) bool }
type ListenerFunc func(Event) bool

func (f ListenerFunc) On(e Event) bool { return f(e) }

type Bus struct {
	byKind map[string][]Listener
	all    []Listener
	log    []string
}

func NewBus() *Bus { return &Bus{byKind: map[string][]Listener{}} }
func (b *Bus) Sub(kind string, l Listener) func() {
	b.byKind[kind] = append(b.byKind[kind], l)
	return func() {
		ls := b.byKind[kind]
		for i, x := range ls {
			if x == l {
				b.byKind[kind] = append(ls[:i:i], ls[i+1:]...)
				return
			}
		}
	}
}
func (b *Bus) Emit(e Event) (handled int) {
	defer func() {
		if r := recover(); r != nil {
			b.log = append(b.log, fmt.Sprintf("listener panic on %s: %v", e.Kind(), r))
		}
	}()
	for _, l := range append(b.byKind[e.Kind()], b.all...) {
		if l.On(e) {
			handled++
		}
	}
	return
}

type counter struct {
	name string
	n    int
}

func (c *counter) On(e Event) bool {
	c.n++
	return true
}

type clickLogger struct{ out *[]string }

func (c clickLogger) On(e Event) bool {
	if ck, ok := e.(Click); ok {
		*c.out = append(*c.out, fmt.Sprintf("click@%d,%d", ck.X, ck.Y))
		return true
	}
	return false
}

func main() {
	bus := NewBus()
	c1, c2 := &counter{name: "c1"}, &counter{name: "c2"}
	var out []string
	unsub1 := bus.Sub("click", c1)
	bus.Sub("click", c2)
	bus.Sub("click", clickLogger{&out})
	bus.Sub("key", ListenerFunc(func(e Event) bool {
		k := e.(Key)
		out = append(out, fmt.Sprintf("key %q", k.Code))
		return k.Code != 'x'
	}))
	bus.Sub("quit", ListenerFunc(func(e Event) bool {
		var m map[string]int
		m["quit"] = 1
		return true
	}))
	bus.all = append(bus.all, ListenerFunc(func(e Event) bool { return e.Kind() == "key" }))
	events := []Event{Click{1, 2}, Key{'a'}, Key{'x'}, Click{3, 4}, &Quit{}, Click{5, 6}}
	for i, e := range events {
		if i == 3 {
			unsub1()
		}
		fmt.Println(e.Kind(), bus.Emit(e))
	}
	fmt.Println(c1.n, c2.n, out)
	for _, l := range bus.log {
		fmt.Println(l)
	}
	fmt.Println(len(bus.byKind["click"]))
}
