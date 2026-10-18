// phase: 05

// HTTP-style middleware: func types with methods, recover in a middleware, runtime errors and custom panics turned into errors.
package main

import (
	"errors"
	"fmt"
)

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }
func upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}

type Handler interface{ Handle(req string) (string, error) }
type HandlerFunc func(string) (string, error)

func (f HandlerFunc) Handle(r string) (string, error) { return f(r) }

type Middleware func(Handler) Handler

func Chain(h Handler, ms ...Middleware) Handler {
	for i := len(ms) - 1; i >= 0; i-- {
		h = ms[i](h)
	}
	return h
}

var log []string

func Logging(name string) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(r string) (string, error) {
			log = append(log, name+" in "+r)
			resp, err := next.Handle(r)
			log = append(log, fmt.Sprintf("%s out %q %v", name, resp, err))
			return resp, err
		})
	}
}

func Recover(next Handler) Handler {
	return HandlerFunc(func(r string) (resp string, err error) {
		defer func() {
			if x := recover(); x != nil {
				err = fmt.Errorf("panic: %v", x)
				if e, ok := x.(error); ok {
					err = fmt.Errorf("panic error: %w", e)
				}
			}
		}()
		return next.Handle(r)
	})
}

var ErrEmpty = errors.New("empty request")

type app struct{ calls int }

func (a *app) Handle(r string) (string, error) {
	a.calls++
	switch {
	case r == "":
		return "", ErrEmpty
	case hasPrefix(r, "boom"):
		panic(r)
	case hasPrefix(r, "nil"):
		var m map[string]int
		m["x"]++
	case hasPrefix(r, "idx"):
		var a []int
		_ = a[len(r)]
	}
	return upper(r), nil
}

func main() {
	a := &app{}
	h := Chain(a, Logging("outer"), Recover, Logging("inner"))
	for _, r := range []string{"hello", "", "boom!", "nil map", "idx", "bye"} {
		resp, err := h.Handle(r)
		fmt.Printf("%q -> %q, %v (empty=%v)\n", r, resp, err, errors.Is(err, ErrEmpty))
	}
	fmt.Println(a.calls)
	for _, l := range log {
		fmt.Println(l)
	}
}
