// phase: 09

package main

import (
	"fmt"
	"os"
	"syscall"
)

func main() {
	var p [2]int
	syscall.Pipe2(p[:], syscall.O_NONBLOCK|syscall.O_CLOEXEC)
	r := os.NewFile(uintptr(p[0]), "r")
	w := os.NewFile(uintptr(p[1]), "w")
	go func() {
		w.WriteString("late data")
		w.Close()
	}()
	buf := make([]byte, 32)
	n, err := r.Read(buf)
	fmt.Println(n, err, string(buf[:n]))
	n, err = r.Read(buf)
	fmt.Println(n, err)
	r.Close()
}

// Output:
// 9 <nil> late data
// 0 EOF
