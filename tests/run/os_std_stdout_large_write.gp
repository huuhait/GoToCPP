// phase: 09

package main

import (
	"fmt"
	"os"
	"syscall"
)

func main() {
	null, _ := os.OpenFile("/dev/null", os.O_WRONLY, 0)
	saved, _ := syscall.Dup(1)
	syscall.Dup2(int(null.Fd()), 1)
	b := make([]byte, 3<<20)
	n, err := os.Stdout.Write(b)
	syscall.Dup2(saved, 1)
	fmt.Println(n, err)
}

// Output:
// 3145728 <nil>
