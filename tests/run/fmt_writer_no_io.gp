// phase: 09

// fmt.Fprint, Fprintf and Fprintln with a writer of the program's own, in a program that does not
// import io (Go's fmt does): the call converts the value to io.Writer. A Format method that writes
// to its fmt.State needs nothing of the kind.
package main

import "fmt"

type counter struct{ calls, n int }

func (c *counter) Write(p []byte) (int, error) {
	c.calls++
	c.n += len(p)
	return len(p), nil
}

type failing struct{}

func (failing) Write(p []byte) (int, error) { return len(p) / 2, fmt.Errorf("short write of %d", len(p)) }

type F struct{}

func (F) Format(s fmt.State, c rune) { fmt.Fprintf(s, "F<%c>", c) }

func main() {
	w := &counter{}
	n, err := fmt.Fprintf(w, "hello %d", 5)
	fmt.Println(n, err, w.n, w.calls)
	fmt.Fprintln(w, "a", 1)
	fmt.Fprint(w, "bc", 2, 3)
	fmt.Println(w.n, w.calls)
	f := fmt.Fprintln
	f(w, "via a func value")
	fmt.Println(w.n, w.calls)
	fmt.Fprintf(w, "%v %s\n", F{}, F{})
	fmt.Println(w.n, w.calls, F{})
	n, err = fmt.Fprint(failing{}, "abcdef")
	fmt.Println(n, err)
}

// Output:
// 7 <nil> 7 1
// 16 3
// 33 4
// 43 5 F<v>
// 3 short write of 6
