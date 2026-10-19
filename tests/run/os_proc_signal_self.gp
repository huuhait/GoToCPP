// phase: 09

package main

import (
	"fmt"
	"os"
	"syscall"
)

func main() {
	p, err := os.FindProcess(os.Getpid())
	fmt.Println(err, p.Pid == os.Getpid())
	fmt.Println(p.Signal(syscall.Signal(0)))
	q, _ := os.FindProcess(1 << 30)
	err = q.Signal(syscall.Signal(0))
	fmt.Println(err)
	fmt.Println(err == os.ErrProcessDone)
	fmt.Println(os.Interrupt, os.Kill)
	fmt.Println(p.Release())
	fmt.Println(p.Pid)
	fmt.Println(p.Signal(os.Interrupt))
	_, err = p.Wait()
	fmt.Println(err)
}

// Output:
// <nil> true
// <nil>
// os: process already finished
// true
// interrupt killed
// <nil>
// -1
// os: process already released
// invalid argument
