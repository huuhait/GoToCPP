package safe

import (
	"strings"
	"sync"
	"testing"
)

func TestCall(t *testing.T) {
	if p := Call(func() {}); p != nil {
		t.Fatalf("no panic: %v", p)
	}
	p := Call(func() { var m map[string]int; m["a"] = 1 })
	if p == nil || !strings.Contains(p.Error(), "nil map") || !strings.Contains(string(p.Stack), "safe.TestCall") {
		t.Fatalf("panic %v", p)
	}
}

// TestCatcher: goroutines that panic are caught; the first panic is raised
// in the goroutine that waits, with the stack of the one that panicked.
func TestCatcher(t *testing.T) {
	var c Catcher
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Run(func() {
				if i%2 == 1 {
					panic("boom")
				}
			})
		}()
	}
	wg.Wait()
	defer func() {
		r := recover()
		p, ok := r.(*Panic)
		if !ok || p.Value != "boom" || !strings.Contains(string(p.Stack), "TestCatcher") {
			t.Fatalf("recovered %v", r)
		}
	}()
	c.Rethrow()
	t.Fatal("no panic")
}

func TestCatcherQuiet(t *testing.T) {
	var c Catcher
	c.Run(func() {})
	c.Rethrow() // nothing
}
