// phase: 09

//go:build unix

package main

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// Fd puts a pipe in blocking mode, as before pipes were pollable: a raw read waits for the data
// (no EAGAIN), and the File keeps working.
func main() {
	r, w, _ := os.Pipe()
	fd := int(r.Fd())
	wfd := int(w.Fd())
	go func() {
		time.Sleep(50 * time.Millisecond)
		syscall.Write(wfd, []byte("late"))
	}()
	buf := make([]byte, 8)
	n, err := syscall.Read(fd, buf)
	fmt.Println(n, err, string(buf[:n]))
	w.WriteString("more")
	n, err = r.Read(buf)
	fmt.Println(n, err, string(buf[:n]))
	w.Close()
	n, err = r.Read(buf)
	fmt.Println(n, err)
	r.Close()
}

// Output:
// 4 <nil> late
// 4 <nil> more
// 0 EOF
