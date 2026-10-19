// phase: 09

//go:build linux

package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
)

// readClosed starts a Read on f, waits until it has surely started (or ended), closes f and
// reports how the Read ended.
func readClosed(f *os.File) string {
	return strings.ReplaceAll(readClosed1(f), dir, "DIR")
}

var dir string

func readClosed1(f *os.File) string {
	res := make(chan string)
	go func() {
		n, err := f.Read(make([]byte, 8))
		res <- fmt.Sprint(n, " ", err)
	}()
	time.Sleep(100 * time.Millisecond)
	cerr := f.Close()
	return fmt.Sprint(<-res, " / close: ", cerr)
}

func main() {
	var err error
	dir, err = os.MkdirTemp("", "fifo")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	path := dir + "/f"
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		panic(err)
	}

	// A FIFO opened for reading and writing (the open does not wait for a peer).
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		panic(err)
	}
	fmt.Println("fifo rdwr:", readClosed(f))

	// A FIFO opened for reading: the open waits for a writer.
	opened := make(chan *os.File)
	go func() {
		w, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			panic(err)
		}
		opened <- w
	}()
	r, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		panic(err)
	}
	w := <-opened
	fmt.Println("fifo rdonly:", readClosed(r))
	// The writer's end: the reader is gone, so a write fails with EPIPE.
	_, err = w.Write([]byte("x"))
	fmt.Println(strings.ReplaceAll(err.Error(), dir, "DIR"))
	fmt.Println(w.Close())

	// A FIFO whose writer closes: EOF.
	go func() {
		w, _ := os.OpenFile(path, os.O_WRONLY, 0)
		w.WriteString("hi")
		w.Close()
	}()
	r, _ = os.OpenFile(path, os.O_RDONLY, 0)
	buf := make([]byte, 8)
	n, err := r.Read(buf)
	fmt.Println(string(buf[:n]), err)
	n, err = r.Read(buf)
	fmt.Println(n, err)
	r.Close()

	// A terminal: the master side of a pseudo terminal has nothing to read.
	pty, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		// Not the line of the expected output: the test fails where it cannot test a terminal.
		fmt.Println("tty: unavailable:", err)
		return
	}
	fmt.Println("tty:", readClosed(pty))
}

// Output:
// fifo rdwr: 0 read DIR/f: file already closed / close: <nil>
// fifo rdonly: 0 read DIR/f: file already closed / close: <nil>
// write DIR/f: broken pipe
// <nil>
// hi <nil>
// 0 EOF
// tty: 0 read /dev/ptmx: file already closed / close: <nil>
