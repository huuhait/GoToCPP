// phase: 08

// Stack writes "goroutine N [running]:" and the calling goroutine's trace: main is goroutine 1.
package main

import (
	"fmt"
	"runtime"
)

func firstLine(b []byte) string {
	for i, c := range b {
		if c == '\n' {
			return string(b[:i])
		}
	}
	return string(b)
}

func has(b []byte, s string) bool {
	for i := 0; i+len(s) <= len(b); i++ {
		if string(b[i:i+len(s)]) == s {
			return true
		}
	}
	return false
}

func show() {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	fmt.Println(firstLine(buf[:n]))
	fmt.Println(has(buf[:n], "main.show"), has(buf[:n], "main.main"))
}

func main() {
	show()
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		l := firstLine(buf[:n])
		fmt.Println(l[:10], l[len(l)-11:], l != "goroutine 1 [running]:")
	}()
	<-done
}

// Output:
// goroutine 1 [running]:
// true true
// goroutine   [running]: true
