// phase: 07

// A generic event bus: handlers of an instance's type parameter, registered and called, returning counts.
package main

import "fmt"

type Handler[E any] func(E) bool

type Bus[E any] struct {
	handlers []Handler[E]
	sent     int
}

func (b *Bus[E]) On(h Handler[E]) *Bus[E] {
	b.handlers = append(b.handlers, h)
	return b
}

func (b *Bus[E]) Emit(e E) int {
	b.sent++
	n := 0
	for _, h := range b.handlers {
		if h(e) {
			n++
		}
	}
	return n
}

type Click struct{ X, Y int }
type Key string

func main() {
	var log []string
	clicks := &Bus[Click]{}
	clicks.On(func(c Click) bool { log = append(log, fmt.Sprint("click ", c.X)); return true }).
		On(func(c Click) bool { return c.X > 5 })
	fmt.Println(clicks.Emit(Click{1, 2}), clicks.Emit(Click{9, 9}), clicks.sent)
	keys := &Bus[Key]{}
	keys.On(func(k Key) bool { log = append(log, string(k)); return len(k) > 1 })
	fmt.Println(keys.Emit("a"), keys.Emit("bb"))
	fmt.Println(log)
	fmt.Printf("%T %T\n", clicks, keys.handlers)
}

// Output:
// 1 2 2
// 0 1
// [click 1 click 9 a bb]
// *main.Bus[main.Click] []main.Handler[main.Key]
