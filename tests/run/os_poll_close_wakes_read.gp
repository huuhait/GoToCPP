// phase: 09

//go:build unix

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Close wakes a Read that is blocked on a pipe (Go's poller does, here the wait is ppoll(2) with a
// wake descriptor), and the Read fails with "file already closed"; the write end closing gives the
// reader io.EOF, as before.
func main() {
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	type result struct {
		n   int
		err error
	}
	got := make(chan result)
	go func() {
		buf := make([]byte, 4)
		n, err := r.Read(buf)
		got <- result{n, err}
		n, err = r.Read(buf) // nothing to read: it waits
		got <- result{n, err}
	}()
	w.WriteString("x")
	res := <-got
	fmt.Println(res.n, res.err)
	time.Sleep(100 * time.Millisecond) // the second Read waits (or has not started: both are right)
	fmt.Println("close:", r.Close())
	res = <-got
	fmt.Println(res.n, res.err, errors.Is(res.err, os.ErrClosed))
	fmt.Println("close again:", r.Close())
	_, err = r.Read(make([]byte, 1))
	fmt.Println(err)
	fmt.Println(w.Close())

	// Several readers blocked on one pipe: one waits in the kernel, the others for the read lock;
	// Close fails all of them.
	r, w, _ = os.Pipe()
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = r.Read(make([]byte, 8))
		}()
	}
	time.Sleep(100 * time.Millisecond)
	r.Close()
	wg.Wait()
	for _, e := range errs {
		fmt.Println(e)
	}
	w.Close()

	// The write end closes: the reader gets the data, then io.EOF.
	r, w, _ = os.Pipe()
	done := make(chan []byte)
	var rerr error
	go func() {
		var b []byte
		b, rerr = io.ReadAll(r)
		done <- b
	}()
	time.Sleep(50 * time.Millisecond)
	w.WriteString("abc")
	fmt.Println(w.Close())
	fmt.Println(string(<-done), rerr)
	fmt.Println(r.Close())
	fmt.Println(r.Close())

	// A Read of a closed file does not wait; a Close with no one waiting does not either.
	r, w, _ = os.Pipe()
	r.Close()
	_, err = r.Read(make([]byte, 1))
	fmt.Println(err)
	w.Close()
}

// Output:
// 1 <nil>
// close: <nil>
// 0 read |0: file already closed true
// close again: close |0: file already closed
// read |0: file already closed
// <nil>
// read |0: file already closed
// read |0: file already closed
// read |0: file already closed
// read |0: file already closed
// <nil>
// abc <nil>
// <nil>
// close |0: file already closed
// read |0: file already closed
