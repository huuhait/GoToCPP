// phase: 09

//go:build unix

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
)

// Deadlines of a pipe: a Read or Write that waits past its deadline fails with os.ErrDeadlineExceeded
// ("i/o timeout"); a deadline in the past fails at once, even if data could be read; a new deadline
// ends the wait that is going on; the zero time clears; a regular file has no deadlines.
func main() {
	r, w, _ := os.Pipe()
	buf := make([]byte, 8)

	r.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	n, err := r.Read(buf)
	fmt.Println(n, err, errors.Is(err, os.ErrDeadlineExceeded))

	// A deadline in the past: no data is read.
	w.WriteString("abc")
	r.SetReadDeadline(time.Now().Add(-time.Second))
	n, err = r.Read(buf)
	fmt.Println(n, err)
	// Cleared: the data is there.
	r.SetReadDeadline(time.Time{})
	n, err = r.Read(buf)
	fmt.Println(n, err, string(buf[:n]))

	// A deadline set while a Read waits: shorter ends it.
	r.SetReadDeadline(time.Now().Add(time.Hour))
	res := make(chan error)
	go func() {
		_, err := r.Read(buf)
		res <- err
	}()
	time.Sleep(100 * time.Millisecond)
	r.SetReadDeadline(time.Now().Add(-time.Millisecond))
	fmt.Println(<-res)

	// A deadline set while a Read waits: later lets it go on, and the data arrives in time.
	r.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	go func() {
		_, err := r.Read(buf)
		res <- err
	}()
	time.Sleep(20 * time.Millisecond)
	r.SetReadDeadline(time.Now().Add(time.Hour))
	time.Sleep(200 * time.Millisecond) // past the first deadline
	w.WriteString("late")
	fmt.Println(<-res, string(buf[:4]))

	// A write deadline on a full pipe; the read deadline is not involved.
	w.SetWriteDeadline(time.Now().Add(50 * time.Millisecond))
	n, err = w.Write(make([]byte, 1<<20))
	fmt.Println(n > 0 && n < 1<<20, err)
	// SetDeadline sets both.
	w.SetDeadline(time.Time{})
	r.SetDeadline(time.Now().Add(-time.Second))
	_, err = r.Read(buf)
	fmt.Println(err)

	// Close ends a Read that has a deadline, and SetDeadline of a closed file fails.
	r.Close()
	w.Close()
	r, w, _ = os.Pipe()
	r.SetDeadline(time.Now().Add(time.Hour))
	go func() {
		_, err := r.Read(buf)
		res <- err
	}()
	time.Sleep(100 * time.Millisecond)
	r.Close()
	fmt.Println(<-res)
	fmt.Println(r.SetDeadline(time.Now()))
	w.Close()

	// A regular file does not support deadlines.
	f, _ := os.CreateTemp("", "deadline")
	defer os.Remove(f.Name())
	err = f.SetReadDeadline(time.Now())
	fmt.Println(err, errors.Is(err, os.ErrNoDeadline))
	f.Close()
	// Neither does /dev/null, nor a directory; a FIFO does.
	for _, name := range []string{"/dev/null", "/"} {
		f, err := os.Open(name)
		if err != nil {
			panic(err)
		}
		fmt.Println(name, f.SetDeadline(time.Now().Add(time.Hour)))
		f.Close()
	}
	dir, _ := os.MkdirTemp("", "dl")
	defer os.RemoveAll(dir)
	if err := syscall.Mkfifo(dir+"/f", 0o600); err != nil {
		panic(err)
	}
	ff, _ := os.OpenFile(dir+"/f", os.O_RDWR, 0)
	fmt.Println("fifo", ff.SetDeadline(time.Now().Add(-time.Second)))
	_, err = ff.Read(buf)
	fmt.Println(strings.ReplaceAll(err.Error(), dir, "DIR"))
	ff.Close()
}

// Output:
// 0 read |0: i/o timeout true
// 0 read |0: i/o timeout
// 3 <nil> abc
// read |0: i/o timeout
// <nil> late
// true write |1: i/o timeout
// read |0: i/o timeout
// read |0: file already closed
// use of closed file
// file type does not support deadline true
// /dev/null file type does not support deadline
// / file type does not support deadline
// fifo <nil>
// read DIR/f: i/o timeout
