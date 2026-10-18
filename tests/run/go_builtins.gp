// phase: 08
// Stderr: ^\n*$

// go with builtins: close, delete, println, and recover (which does nothing when it is the go
// statement's own call).
package main

import "fmt"

func main() {
	c := make(chan int)
	go close(c)
	_, ok := <-c
	fmt.Println("closed:", !ok)

	m := map[string]int{"a": 1, "b": 2}
	done := make(chan bool)
	go func() {
		delete(m, "a")
		done <- true
	}()
	<-done
	fmt.Println(len(m), m["b"])

	for i := 0; i < 3; i++ {
		go println() // an empty line on stderr; nothing on stdout
	}
	go recover()
	go func() {
		fmt.Println("recover() in a goroutine's own call:", recover())
		done <- true
	}()
	<-done
}

// Output:
// closed: true
// 1 2
// recover() in a goroutine's own call: <nil>
