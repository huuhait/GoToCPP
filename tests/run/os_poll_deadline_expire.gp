// phase: 09

//go:build unix

package main

import (
	"fmt"
	"io"
	"os"
	"time"
)

// A deadline that has expired while nothing waited fails the next call even when it could proceed
// (data to read, room to write); the write deadline can be changed while a Write waits (it wakes
// the write waiter, not only the read one).
func main() {
	r, w, _ := os.Pipe()
	b := make([]byte, 8)

	// A positive deadline that expires, then data is there: the Read still fails.
	r.SetReadDeadline(time.Now().Add(20 * time.Millisecond))
	time.Sleep(60 * time.Millisecond)
	w.Write([]byte("data"))
	n, err := r.Read(b)
	fmt.Println(n, err)
	r.SetReadDeadline(time.Time{})
	n, err = r.Read(b)
	fmt.Println(n, err)

	// The same for a Write with room.
	w.SetWriteDeadline(time.Now().Add(20 * time.Millisecond))
	time.Sleep(60 * time.Millisecond)
	n, err = w.Write([]byte("x"))
	fmt.Println(n, err)
	w.SetWriteDeadline(time.Time{})

	// A Write that waits on a full pipe gets a deadline: it ends with the timeout.
	res := make(chan string)
	go func() {
		n, err := w.Write(make([]byte, 1<<20))
		res <- fmt.Sprint(n > 0 && n < 1<<20, err)
	}()
	time.Sleep(100 * time.Millisecond)
	w.SetWriteDeadline(time.Now().Add(50 * time.Millisecond))
	fmt.Println(<-res)

	// A deadline set far away while the Write waits (the wait was bounded by a near one): the Write
	// goes on and finishes when the reader drains the pipe.
	w.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
	go func() {
		n, err := w.Write(make([]byte, 1<<20))
		res <- fmt.Sprint(n, err)
	}()
	time.Sleep(30 * time.Millisecond)
	w.SetWriteDeadline(time.Now().Add(time.Hour))
	time.Sleep(200 * time.Millisecond) // past the first deadline: the Write still waits
	go func() {
		io.Copy(io.Discard, r)
	}()
	fmt.Println(<-res)
	w.Close()
	r.Close()
}

// Output:
// 0 read |0: i/o timeout
// 4 <nil>
// 0 write |1: i/o timeout
// true write |1: i/o timeout
// 1048576 <nil>
