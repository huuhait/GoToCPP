// phase: 05

// The error type switch idiom, with value and pointer error types (step 5.5).
package main

import "fmt"

type NotFound struct{ key string }
type Timeout struct{ ms int }
type Wrapped struct {
	op  string
	err error
}

func (n NotFound) Error() string { return "not found: " + n.key }
func (t *Timeout) Error() string { return fmt.Sprint("timeout ", t.ms) }
func (w *Wrapped) Error() string { return w.op + ": " + w.err.Error() }

func do(i int) error {
	switch i {
	case 0:
		return nil
	case 1:
		return NotFound{"k"}
	case 2:
		return &Timeout{30}
	case 3:
		return &Wrapped{"read", NotFound{"w"}}
	}
	return &Wrapped{"deep", &Wrapped{"inner", &Timeout{1}}}
}

func handle(err error) string {
	switch e := err.(type) {
	case nil:
		return "ok"
	case NotFound:
		return "nf " + e.key
	case *Timeout:
		return fmt.Sprint("to ", e.ms)
	case *Wrapped:
		return "wrapped(" + handle(e.err) + ")"
	default:
		return "unknown"
	}
}

func main() {
	for i := 0; i < 5; i++ {
		err := do(i)
		fmt.Println(handle(err))
		if err != nil {
			fmt.Println(err.Error())
		}
	}
}

// Output:
// ok
// nf k
// not found: k
// to 30
// timeout 30
// wrapped(nf w)
// read: not found: w
// wrapped(wrapped(to 1))
// deep: inner: timeout 1
