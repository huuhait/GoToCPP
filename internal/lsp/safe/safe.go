// Package safe turns a panic in a goroutine of the language server into a
// value: a bug in the work of one request, one check or one background task
// (a nil type of an expression that did not check, say) must not end the
// server and every open document with it. A goroutine that nobody can
// recover for (the process ends with its panic) gets a recover here, and
// the panic is reported to whoever waits for the work, or logged.
package safe

import (
	"fmt"
	"runtime/debug"
	"sync"
)

// A Panic is a panic that was recovered: its value and the stack of the
// goroutine that panicked.
type Panic struct {
	Value any
	Stack []byte
}

func (p *Panic) Error() string {
	return fmt.Sprintf("internal error: %v\n%s", p.Value, p.Stack)
}

// Call runs f and returns the panic that ended it, or nil.
func Call(f func()) (p *Panic) {
	defer func() {
		if r := recover(); r != nil {
			p = &Panic{Value: r, Stack: debug.Stack()}
		}
	}()
	f()
	return nil
}

// A Catcher keeps the first panic of the goroutines that run through it, to
// be raised again in the goroutine that waits for them (so that the code
// that recovers there, a request's, sees it).
//
//	var c safe.Catcher
//	go func() { defer wg.Done(); c.Run(work) }()
//	wg.Wait()
//	c.Rethrow()
type Catcher struct {
	mu sync.Mutex
	p  *Panic
}

// Run runs f, keeping its panic.
func (c *Catcher) Run(f func()) {
	if p := Call(f); p != nil {
		c.mu.Lock()
		if c.p == nil {
			c.p = p
		}
		c.mu.Unlock()
	}
}

// Rethrow panics, in the calling goroutine, with the first panic that Run
// kept (with the stack of the goroutine that panicked); it does nothing when
// there was none.
func (c *Catcher) Rethrow() {
	c.mu.Lock()
	p := c.p
	c.mu.Unlock()
	if p != nil {
		panic(p)
	}
}
