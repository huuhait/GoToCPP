// phase: 09

//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"time"
)

// Close wakes a Read that is blocked on a pipe (the system cancels the I/O: CancelIoEx), and the
// Read fails with "file already closed"; a reader that sees the writer close gets io.EOF.
func main() {
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	got := make(chan string)
	go func() {
		buf := make([]byte, 4)
		n, err := r.Read(buf) // the data first: the goroutine runs
		got <- fmt.Sprint(n, " ", err)
		n, err = r.Read(buf) // nothing to read: it waits in the system
		got <- fmt.Sprint(n, " ", err)
	}()
	w.WriteString("x")
	fmt.Println(<-got)
	// Give the second Read time to enter the system (if it has not, Close ends it before it starts).
	time.Sleep(200 * time.Millisecond)
	fmt.Println(r.Close())
	fmt.Println(<-got)
	fmt.Println(w.Close())

	// The writer closes: EOF after the data.
	r, w, _ = os.Pipe()
	w.WriteString("abc")
	w.Close()
	b, err := io.ReadAll(r)
	fmt.Println(string(b), err)
	n, err := r.Read(make([]byte, 1))
	fmt.Println(n, err)
	fmt.Println(r.Close())
	fmt.Println(r.Close())

	// The reader closes: a write fails.
	r, w, _ = os.Pipe()
	r.Close()
	_, err = w.Write([]byte("x"))
	fmt.Println(err)
	w.Close()

	// Seek and ReadAt on a pipe.
	r, w, _ = os.Pipe()
	_, err = r.Seek(0, 0)
	fmt.Println(err)
	_, err = r.ReadAt(make([]byte, 1), 0)
	fmt.Println(err)
	_, err = w.WriteAt([]byte("x"), 0)
	fmt.Println(err)
	r.Close()
	w.Close()
}

// Output:
// 1 <nil>
// <nil>
// 0 read |0: file already closed
// <nil>
// abc <nil>
// 0 EOF
// <nil>
// close |0: file already closed
// write |1: The pipe is being closed.
// seek |0: illegal seek
// read |0: illegal seek
// write |1: illegal seek
