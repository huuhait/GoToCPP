// phase: 09

//go:build unix

// A Read of a pipe that had to wait and got less than its buffer makes the next Read wait before it
// reads (gocpp's FD.Read: one system call less while a reader keeps waiting, stdlib/internal/poll
// PORT.md). Such a Read still sees data, a deadline, Close and the end of the pipe as Go's does,
// and a file in blocking mode does not wait first.
package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	go func() {
		for _, m := range []string{"one", "two", "three"} {
			time.Sleep(20 * time.Millisecond)
			w.Write([]byte(m))
		}
	}()
	buf := make([]byte, 4096)
	for i := 0; i < 3; i++ {
		n, err := r.Read(buf)
		fmt.Println(string(buf[:n]), err)
	}
	// The next Read waits first: a deadline ends it, and a later Read gets the data that comes.
	r.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	n, err := r.Read(buf)
	fmt.Println(n, err)
	r.SetReadDeadline(time.Time{})
	go func() {
		time.Sleep(20 * time.Millisecond)
		w.Write([]byte("four"))
	}()
	n, err = r.Read(buf)
	fmt.Println(string(buf[:n]), err)
	// Close ends a Read that waits first.
	done := make(chan error)
	go func() {
		_, err := r.Read(buf)
		done <- err
	}()
	time.Sleep(50 * time.Millisecond)
	r.Close()
	fmt.Println(<-done)
	w.Close()

	// The end of the pipe after a wait: the data, then io.EOF.
	r2, w2, _ := os.Pipe()
	go func() {
		time.Sleep(20 * time.Millisecond)
		w2.Write([]byte("last"))
		time.Sleep(20 * time.Millisecond)
		w2.Close()
	}()
	for {
		n, err := r2.Read(buf)
		fmt.Printf("%q %v\n", buf[:n], err)
		if err != nil {
			break
		}
	}
	// A Read that fills its buffer reads at once again; one byte at a time here.
	r3, w3, _ := os.Pipe()
	w3.Write([]byte("abc"))
	one := make([]byte, 1)
	got := ""
	for i := 0; i < 3; i++ {
		n, err := r3.Read(one)
		got += fmt.Sprint(string(one[:n]), err, ";")
	}
	fmt.Println(got)
	r2.Close()
	r3.Close()
	w3.Close()

	// In blocking mode (File.Fd) a Read does not wait first: it blocks in read, which a deadline
	// does not end (Go).
	r4, w4, _ := os.Pipe()
	go func() {
		time.Sleep(30 * time.Millisecond)
		w4.Write([]byte("ab"))
		time.Sleep(300 * time.Millisecond)
		w4.Write([]byte("cd"))
	}()
	n, err = r4.Read(buf) // waits, gets less than its buffer
	fmt.Println(string(buf[:n]), err)
	_ = r4.Fd()
	fmt.Println(r4.SetReadDeadline(time.Now().Add(100 * time.Millisecond)))
	n, err = r4.Read(buf)
	fmt.Println(string(buf[:n]), err)
	r4.Close()
	w4.Close()
}

// Output:
// one <nil>
// two <nil>
// three <nil>
// 0 read |0: i/o timeout
// four <nil>
// read |0: file already closed
// "last" <nil>
// "" EOF
// a<nil>;b<nil>;c<nil>;
// ab <nil>
// <nil>
// cd <nil>
