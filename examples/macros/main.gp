// phase: 10

// Macros shows gocpp macros: macro declarations; #Assert and #Dbg used inside
// code; #Trace and #Locked on top of funcs (start style: the macro runs at
// the start, and the func() it returns runs at the end); #Recover and
// #InThread on top of funcs (wrap style: #body() runs the function's code
// where the macro wants it); #Derive(String) on an enum and #Derive(JSON) on
// a struct.
package main

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Assert panics with the place and the text of the condition when cond is
// false. msg is lazy: it is only evaluated when the assertion fails.
macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: assertion failed: %s: %s", #file(), #line(), #text(cond), msg()))
	}
}

// Dbg prints where it is used, the text of v and its value, and returns v.
macro Dbg[T any](v T) T {
	fmt.Printf("[%s:%d] %s = %v\n", #file(), #line(), #text(v), v)
	return v
}

// Trace prints when the function starts and when it ends, also when it
// panics.
macro Trace() func() {
	fmt.Println("enter", #func())
	return func() { fmt.Println("exit ", #func()) }
}

// Locked holds mu while the function runs.
macro Locked(mu *sync.Mutex) func() {
	mu.Lock()
	return mu.Unlock
}

// Recover stops a panic in the function's code; the function then returns
// zero values.
macro Recover() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recovered in", #func()+":", r)
		}
	}()
	#body()
}

// InThread runs the function's code on a goroutine of its own and waits for
// it.
macro InThread() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		#body()
	}()
	<-done
}

#Derive(String)
type Level int

const (
	Debug Level = iota
	Info
	Warn
	Error
)

#Derive(JSON)
type Event struct {
	ID    int      `json:"id"`
	Name  string   `json:"name"`
	Level Level    `json:"level"`
	Tags  []string `json:"tags,omitempty"`
}

type Counter struct {
	mu sync.Mutex
	n  map[string]int
}

#Locked(&c.mu)
func (c *Counter) Add(key string, n int) {
	c.n[key] += n
}

// built counts how often an assertion message was built.
var built int

func note(format string, args ...any) string {
	built++
	return fmt.Sprintf(format, args...)
}

#Trace()
func parseEvent(data string) Event {
	var ev Event
	err := json.Unmarshal([]byte(data), &ev)
	#Assert(err == nil, note("bad event %s", data))
	return ev
}

#Trace()
#Recover()
func ratio(a, b int) int {
	#Assert(b != 0, note("ratio(%d, %d)", a, b))
	return a / b
}

#InThread()
func square(n int) int {
	return n * n
}

func main() {
	sum := #Dbg(square(3) + square(4))
	fmt.Println("sum:", sum)

	// square runs on a goroutine of its own, here called from 4 goroutines.
	results := make([]int, 4)
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() { results[i] = square(i + 1) })
	}
	wg.Wait()
	fmt.Println("squares:", results)

	// 100 Adds at the same time: Locked lets one run at a time.
	c := &Counter{n: map[string]int{}}
	for i := range 100 {
		key := "even"
		if i%2 == 1 {
			key = "odd"
		}
		wg.Go(func() { c.Add(key, i) })
	}
	wg.Wait()
	fmt.Println("counter:", c.n)

	// The assertion in parseEvent passes, so its message is never built.
	ev := parseEvent(`{"id": 1, "name": "start", "level": 1, "tags": ["boot"]}`)
	fmt.Println("event:", ev.ID, ev.Name, ev.Tags)
	fmt.Println("level:", #Dbg(ev.Level))

	// The derived String and MarshalJSON.
	fmt.Println(Debug, Info, Warn, Error, Level(7))
	data, err := json.Marshal(Event{ID: 2, Name: "stop", Level: Warn})
	fmt.Println(string(data), err)

	// ratio(1, 0) fails its assertion; Recover stops the panic and Trace
	// still prints the exit.
	fmt.Println("ratio:", ratio(8, 2))
	fmt.Println("ratio:", ratio(1, 0))
	fmt.Println("messages built:", built)
}
